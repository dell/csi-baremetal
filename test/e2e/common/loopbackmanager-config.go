package common

import (
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LoopBackManagerConfigDevice struct contains fields to describe a loop device bound with a file
type LoopBackManagerConfigDevice struct {
	VendorID     *string `yaml:"vid,omitempty"`
	ProductID    *string `yaml:"pid,omitempty"`
	SerialNumber *string `yaml:"serialNumber,omitempty"`
	Size         *string `yaml:"size,omitempty"`
	Removed      *bool   `yaml:"removed,omitempty"`
	Health       *string `yaml:"health,omitempty"`
	DriveType    *string `yaml:"driveType,omitempty"`
}

// LoopBackManagerConfigNode struct represents particular configuration of LoopBackManager for specified node
type LoopBackManagerConfigNode struct {
	NodeID     *string                        `yaml:"nodeID,omitempty"`
	DriveCount *int                           `yaml:"driveCount,omitempty"`
	Drives     *[]LoopBackManagerConfigDevice `yaml:"drives,omitempty"`
}

// LoopBackManagerConfig struct is the configuration for LoopBackManager.
// It contains default settings and settings for each node
type LoopBackManagerConfig struct {
	DefaultDriveCount *int                         `yaml:"defaultDrivePerNodeCount,omitempty"`
	DefaultDriveSize  *string                      `yaml:"defaultDriveSize,omitempty"`
	Nodes             *[]LoopBackManagerConfigNode `yaml:"nodes,omitempty"`
}

// BuildLoopBackManagerConfigMap returns ConfigMap with configuration for loopback manager
func BuildLoopBackManagerConfigMap(namespace string, name string,
	config LoopBackManagerConfig) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	renderedConf, err := yaml.Marshal(config)
	if err != nil {
		return nil, err
	}
	cm.Data = map[string]string{"config.yaml": string(renderedConf)}
	return cm, nil
}
