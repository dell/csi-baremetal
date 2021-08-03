package common

const (
	// ExtenderConfigMapPath - the path to ExtenderConfigMap
	ExtenderConfigMapPath = "/status"
	// ExtenderConfigMapFile - ExtenderConfigMap data key
	ExtenderConfigMapFile = "nodes.yaml"
	// ExtenderConfigMapFullPath - full path to extender-readiness file
	ExtenderConfigMapFullPath = ExtenderConfigMapPath + "/" + ExtenderConfigMapFile
)

// ReadinessStatus contains info about kube-scheduler restart for the related node
type ReadinessStatus struct {
	NodeName      string `yaml:"node_name"`
	KubeScheduler string `yaml:"kube_scheduler"`
	Restarted     bool   `yaml:"restarted"`
}

// ReadinessStatusList contains info about all kube-schedulers
type ReadinessStatusList struct {
	Items []ReadinessStatus `yaml:"nodes"`
}
