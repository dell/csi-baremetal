package wbt

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/dell/csi-baremetal/pkg/eventing"
	"github.com/dell/csi-baremetal/pkg/events"
	"github.com/dell/csi-baremetal/pkg/node"
	"github.com/dell/csi-baremetal/pkg/node/wbt/common"
)

const (
	confPath    = "/etc/node_config/wbt-config.yaml"
	kernelsPath = "/etc/node_config/wbt-acceptable_kernels.yaml"

	watchTimeout = 60 * time.Second

	podNameEnv      = "POD_NAME"
	podNamespaceEnv = "NAMESPACE"
)

// ConfWatcher is watcher to update WBT changing configuration in VolumeManager
type ConfWatcher struct {
	client            k8sClient.Client
	eventsRecorder    *events.Recorder
	log               *logrus.Entry
	nodeKernelVersion string
}

// NewConfWatcher create new WBT Config Watcher with node kernel version
func NewConfWatcher(client k8sClient.Client, eventsRecorder *events.Recorder, log *logrus.Entry, nodeKernelVersion string) *ConfWatcher {
	return &ConfWatcher{
		client:            client,
		eventsRecorder:    eventsRecorder,
		log:               log,
		nodeKernelVersion: nodeKernelVersion,
	}
}

// StartWatch tries to read Config for WBT changing from ConfigMap
// Set conf in VolumeManager if success
func (w *ConfWatcher) StartWatch(cns *node.CSINodeService) {
	go func() {
		for {
			wbtConf, err := w.readConfig()
			if err != nil {
				w.log.Errorf("unable to read WBT config: %+v", err)
				w.sendErrorConfigmapEvent()
				cns.SetWbtConfig(&common.WbtConfig{Enable: false})
			} else {
				cns.SetWbtConfig(wbtConf)
			}
			time.Sleep(watchTimeout)
		}
	}()
}

func (w *ConfWatcher) readConfig() (*common.WbtConfig, error) {
	kernels := &common.AcceptableKernelsConfig{}
	kernelsFile, err := ioutil.ReadFile(kernelsPath)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(kernelsFile, kernels)
	if err != nil {
		return nil, err
	}

	conf := &common.WbtConfig{}
	confFile, err := ioutil.ReadFile(confPath)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(confFile, conf)
	if err != nil {
		return nil, err
	}

	if !kernels.EnableForAll {
		isKernelInList := false
		for _, kernelVersion := range kernels.KernelVersions {
			if kernelVersion == w.nodeKernelVersion {
				isKernelInList = true
				break
			}
		}
		if !isKernelInList {
			conf.Enable = false
		}
	}

	return conf, nil
}

func (w *ConfWatcher) sendErrorConfigmapEvent() {
	podName := os.Getenv(podNameEnv)
	podNamespace := os.Getenv(podNamespaceEnv)

	ctx := context.Background()

	pod := &corev1.Pod{}
	if err := w.client.Get(ctx, k8sClient.ObjectKey{Name: podName, Namespace: podNamespace}, pod); err != nil {
		w.log.Errorf("Failed to get Pod %s in Namespace %s: %+v", podName, podNamespace, err)
		return
	}

	w.eventsRecorder.Eventf(pod, eventing.WBTConfigMapUpdateFailed,
		"Failed to get info from Node ConfigMap")
}
