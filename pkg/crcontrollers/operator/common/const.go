// Package common contains variables that are used in controller code and in other places too
// the reason of that package is to avoid conflict during initialization k8s client for e2e test
package common

const (
	nodeKey = "nodes.csi-baremetal.dell.com"
	// NodeIDAnnotationKey hold key for annotation for node object
	NodeIDAnnotationKey = nodeKey + "/uuid"
	// NodeOSNameLabelKey used as a label key for k8s node object to sort nodes by OS name (for example, Ubuntu)
	NodeOSNameLabelKey = nodeKey + "/os-name"
	// NodeOSVersionLabelKey used as a label key for k8s node object to sort nodes by OS version (for example, 19.04)
	NodeOSVersionLabelKey = nodeKey + "/os-version"
	// NodeKernelVersionLabelKey used as a label key for k8s node object to sort nodes by kernel version (for example, 5.4.0)
	NodeKernelVersionLabelKey = nodeKey + "/kernel-version"
)
