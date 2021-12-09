package wbt

import (
	"io/ioutil"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/dell/csi-baremetal/pkg/node"
	"github.com/dell/csi-baremetal/pkg/node/wbt/common"
)

const (
	confPath    = "/etc/wbt_config/config.yaml"
	kernelsPath = "/etc/wbt_config/acceptable_kernels.yaml"

	watchTimeout = 60 * time.Second
)

// ConfWatcher is watcher to update WBT changing configuration in VolumeManager
type ConfWatcher struct {
	log               *logrus.Entry
	nodeKernelVersion string
}

// NewConfWatcher create new WBT Config Watcher with node kernel version
func NewConfWatcher(log *logrus.Entry, nodeKernelVersion string) *ConfWatcher {
	return &ConfWatcher{
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

	return conf, nil
}
