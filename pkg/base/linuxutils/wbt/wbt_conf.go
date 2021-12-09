package wbt

import (
	"io/ioutil"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/dell/csi-baremetal/pkg/node"
)

type WbtConfig struct {
	Enable        bool   `yaml:"enable"`
	Value         uint32 `yaml:"wbt_lat_usec_value"`
	VolumeOptions struct {
		Modes        []string `yaml:"modes"`
		StorageTypes []string `yaml:"storage_types"`
	} `yaml:"acceptable_volume_options"`
}

type AcceptableKernelsConfig struct {
	KernelVersions []string `yaml:"node_kernel_versions"`
}

const (
	confPath = "/config.yaml"
	kernelsPath = "/acceptable_kernels.yaml"

	watchTimeout = 60 * time.Second
)

type ConfWatcher struct {
	log               *logrus.Entry
	nodeKernelVersion string
}

func NewConfWatcher(log *logrus.Entry, nodeKernelVersion string) *ConfWatcher {
	return &ConfWatcher{
		log:               log,
		nodeKernelVersion: nodeKernelVersion,

	}
}

func (w *ConfWatcher) StartWatch(cns *node.CSINodeService) {
	go func() {
		for {
			wbtConf, err := w.readConfig()
			if err != nil {
				w.log.Errorf("unable to read WBT config: %+v", err)
				cns.SetWbtConfig(&WbtConfig{Enable: false})
			} else {
				cns.SetWbtConfig(wbtConf)
			}
			time.Sleep(watchTimeout)
		}
	}()
}

func (w *ConfWatcher) readConfig() (*WbtConfig, error) {
	kernels := &AcceptableKernelsConfig{}
	kernelsFile, err := ioutil.ReadFile(kernelsPath)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(kernelsFile, kernels)
	if err != nil {
		return nil, err
	}

	conf := &WbtConfig{}
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

