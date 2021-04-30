/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
)

func newSchedulerRestartChecker(client clientset.Interface) *schedulerRestartChecker {
	return &schedulerRestartChecker{
		IsInitialized:      false,
		c:                  client,
		schedulerLabel:     "component=kube-scheduler",
		restartWaitTimeout: time.Minute * 2,
	}
}

type schedulerRestartChecker struct {
	c                  clientset.Interface
	initialState       map[string]metav1.Time
	schedulerLabel     string
	restartWaitTimeout time.Duration
	IsInitialized      bool
}

func (rc *schedulerRestartChecker) ReadInitialState() error {
	var err error
	rc.initialState, err = rc.getPODStartTimeMap()
	if err != nil {
		return err
	}
	if len(rc.initialState) == 0 {
		return fmt.Errorf("can't find schedulers PODs during reading initial state")
	}

	rc.IsInitialized = true
	return nil
}

func (rc *schedulerRestartChecker) WaitForRestart() (bool, error) {
	e2elog.Logf("Wait for scheduler restart")

	deadline := time.Now().Add(rc.restartWaitTimeout)
	for {
		ready, err := rc.CheckRestarted()
		if err != nil {
			return false, err
		}
		if ready {
			e2elog.Logf("Scheduler restarted")
			return true, nil
		}
		msg := "Scheduler restart NOT detected yet"
		e2elog.Logf(msg)
		if time.Now().After(deadline) {
			e2elog.Logf("Scheduler didn't receive extender configuration after %f minutes. Continue...",
				rc.restartWaitTimeout.Minutes())
			break
		}
		time.Sleep(time.Second * 5)
	}

	return false, nil
}

func (rc *schedulerRestartChecker) CheckRestarted() (bool, error) {
	currentState, err := rc.getPODStartTimeMap()
	if err != nil {
		return false, err
	}
	for podName, initialTime := range rc.initialState {
		currentTime, ok := currentState[podName]
		if !ok {
			// podName not found
			return false, nil
		}
		// check that POD start time changed
		if !currentTime.After(initialTime.Time) {
			// at lease on pod not restarted yet
			return false, nil
		}
		// check that POD uptime more than 10 seconds
		// we need to wait additional 10 seconds to protect from CrashLoopBackOff caused by frequently POD restarts
		if time.Since(currentTime.Time).Seconds() <= 10 {
			return false, nil
		}
	}
	return true, nil
}

func (rc *schedulerRestartChecker) getPODStartTimeMap() (map[string]metav1.Time, error) {
	pods, err := rc.findSchedulerPods()
	if err != nil {
		return nil, err
	}
	return rc.buildPODStartTimeMap(pods), nil
}

func (rc *schedulerRestartChecker) buildPODStartTimeMap(pods *corev1.PodList) map[string]metav1.Time {
	data := map[string]metav1.Time{}
	for _, p := range pods.Items {
		if len(p.Status.ContainerStatuses) == 0 {
			continue
		}
		if p.Status.ContainerStatuses[0].State.Running == nil {
			data[p.Name] = metav1.Time{}
			continue
		}
		data[p.Name] = p.Status.ContainerStatuses[0].State.Running.StartedAt
	}
	return data
}

func (rc *schedulerRestartChecker) findSchedulerPods() (*corev1.PodList, error) {
	pods, err := rc.c.CoreV1().Pods("").List(metav1.ListOptions{LabelSelector: rc.schedulerLabel})
	if err != nil {
		return nil, err
	}
	e2elog.Logf("Find %d scheduler pods", len(pods.Items))
	return pods, nil
}
