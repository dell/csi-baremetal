package wbt

import (
	"context"
	"github.com/dell/csi-baremetal/pkg/node/processor"
	"github.com/dell/csi-baremetal/pkg/node/processor/wbt/common"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	"os"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/dell/csi-baremetal/pkg/eventing"
	"github.com/dell/csi-baremetal/pkg/events"
	"github.com/dell/csi-baremetal/pkg/node"
)

const (
	confPath    = "/etc/node_config/wbt-config.yaml"
	kernelsPath = "/etc/node_config/wbt-acceptable_kernels.yaml"

	podNameEnv      = "POD_NAME"
	podNamespaceEnv = "NAMESPACE"
)

// confWatcher is watcher to update WBT changing configuration in VolumeManager
type confWatcher struct {
	client            k8sClient.Client
	cns               *node.CSINodeService
	nodeKernelVersion string
	eventsRecorder    *events.Recorder
	log               *logrus.Entry
}

// Handle tries to read Config for WBT changing from ConfigMap
// Set conf in VolumeManager if success
func (w *confWatcher) Handle(ctx context.Context) {
	wbtConf, err := w.readConfig()
	if err != nil {
		w.log.Errorf("unable to read WBT config: %+v", err)
		w.sendErrorConfigmapEvent(ctx)
		w.cns.SetWbtConfig(&common.WbtConfig{Enable: false})
	} else {
		w.cns.SetWbtConfig(wbtConf)
	}
}

func (w *confWatcher) readConfig() (*common.WbtConfig, error) {
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

func (w *confWatcher) sendErrorConfigmapEvent(ctx context.Context) {
	podName := os.Getenv(podNameEnv)
	podNamespace := os.Getenv(podNamespaceEnv)

	pod := &corev1.Pod{}
	if err := w.client.Get(ctx, k8sClient.ObjectKey{Name: podName, Namespace: podNamespace}, pod); err != nil {
		w.log.Errorf("Failed to get Pod %s in Namespace %s: %+v", podName, podNamespace, err)
		return
	}

	w.eventsRecorder.Eventf(pod, eventing.WBTConfigMapUpdateFailed,
		"Failed to get info from Node ConfigMap")
}

// NewConfWatcher create new WBT Config Watcher with node kernel version
func NewConfWatcher(client k8sClient.Client,
	cns *node.CSINodeService, nodeKernelVersion string,
	eventsRecorder *events.Recorder, log *logrus.Entry,
) processor.Processor {
	return &confWatcher{
		client:            client,
		cns:               cns,
		nodeKernelVersion: nodeKernelVersion,
		eventsRecorder:    eventsRecorder,
		log:               log,
	}
}
