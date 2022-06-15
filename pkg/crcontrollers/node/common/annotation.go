/*
Copyright © 2021 Dell Inc. or its subsidiaries. All Rights Reserved.

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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dell/csi-baremetal/api/v1/nodecrd"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/dell/csi-baremetal/pkg/base/featureconfig"
)

type service struct {
	client            k8sClient.Client
	log               *logrus.Logger
	featureConfig     featureconfig.FeatureChecker
	delayBetweenRetry time.Duration
	numberOfRetry     int
}

// NodeAnnotation represent annotation service
type NodeAnnotation interface {
	ObtainNodeID(nodeName string) (nodeID string, err error)
	GetNodeID(node *corev1.Node, annotationKey, nodeSelector string) (string, error)
	SetFeatureConfig(featureConf featureconfig.FeatureChecker)
}

// New return NodeAnnotation service
func New(client k8sClient.Client, log *logrus.Logger, options ...func(*service)) NodeAnnotation {
	srv := &service{client: client, log: log}
	for _, o := range options {
		o(srv)
	}
	return srv
}

// SetFeatureConfig set feature config for service
func (srv *service) SetFeatureConfig(featureConf featureconfig.FeatureChecker) {
	srv.featureConfig = featureConf
}

// WithFeatureConfig set feature config for service
func WithFeatureConfig(featureConf featureconfig.FeatureChecker) func(*service) {
	return func(s *service) {
		s.featureConfig = featureConf
	}
}

// WithRetryNumber declare number of retries for ObtainNodeID
func WithRetryNumber(count int) func(*service) {
	return func(s *service) {
		s.numberOfRetry = count
	}
}

// WithRetryDelay declare delay between retries for ObtainNodeID
func WithRetryDelay(delay time.Duration) func(*service) {
	return func(s *service) {
		s.delayBetweenRetry = delay
	}
}

// ObtainNodeID obtains Node ID with retries
func (srv *service) ObtainNodeID(nodeName string) (nodeID string, err error) {
	ctx := context.Background()
	retryCount := 0
	for {
		srv.log.Info("Obtaining node ID...")
		retryCount++
		nodes := nodecrd.NodeList{}
		if err := srv.client.List(ctx, &nodes); err != nil {
			srv.log.Errorf("obtain node id for node: %s failed err: %s", nodeName, err)
			time.Sleep(srv.delayBetweenRetry)
			continue
		}
		for _, node := range nodes.Items {
			if node.Spec.Addresses["Hostname"] == nodeName {
				nodeID = node.Spec.UUID
			}
		}
		if nodeID != "" {
			return nodeID, nil
		}
		if retryCount > srv.numberOfRetry {
			break
		}
		srv.log.Warningf("Unable to get node ID name:%s from bmnodes, sleep:%s and retry:%d...",
			nodeName,
			srv.delayBetweenRetry,
			retryCount)
		time.Sleep(srv.delayBetweenRetry)
	}
	// return empty node ID and error
	return "", fmt.Errorf("number of retries %d exceeded", srv.numberOfRetry)
}

// GetNodeID return special id for node corev1.Node
// depends on NodeIdFromAnnotation and ExternalNodeAnnotation features
func (srv *service) GetNodeID(node *corev1.Node, annotationKey, nodeSelector string) (string, error) {
	nodeName, id := node.Name, string(node.UID)
	if srv.featureConfig.IsEnabled(featureconfig.FeatureNodeIDFromAnnotation) {
		if nodeSelector != "" {
			key, value := labelStringToKV(nodeSelector)
			if val, ok := node.GetLabels()[key]; !ok || val != value {
				return "", nil
			}
		}
		akey, err := srv.chooseAnnotationKey(annotationKey)
		if err != nil {
			return "", err
		}

		if val, ok := node.GetAnnotations()[akey]; ok {
			return val, nil
		}
		return "", fmt.Errorf("annotation %s hadn't been set for node %s", akey, nodeName)
	}
	if id == "" {
		return "", fmt.Errorf("uid for node %s is not set", nodeName)
	}
	return id, nil
}

func (srv *service) chooseAnnotationKey(annotationKey string) (string, error) {
	if srv.featureConfig.IsEnabled(featureconfig.FeatureExternalAnnotationForNode) {
		if annotationKey == "" {
			return "", fmt.Errorf("%s is set as True but annotation keys is empty", featureconfig.FeatureExternalAnnotationForNode)
		}
		return annotationKey, nil
	}

	return DeafultNodeIDAnnotationKey, nil
}

func labelStringToKV(payload string) (string, string) {
	data := strings.Split(payload, "=")
	if len(data) != 2 {
		return "", ""
	}
	return data[0], data[1]
}
