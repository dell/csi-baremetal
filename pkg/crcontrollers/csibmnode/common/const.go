// Package common contains variables that are used in controller code and in other places too
// the reason of that package is to avoid conflict during initialization k8s client for e2e test
package common

const (
	// NodeIDAnnotationKey hold key for annotation for node object
	NodeIDAnnotationKey = "csibmnodes.csi-baremetal.dell.com/uuid"
)
