// Package acreservationcrd contains API Schema definitions for the available capacity v1 API group
// +groupName=baremetal-csi.dellemc.com
// +versionName=v1
package acreservationcrd

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	crScheme "sigs.k8s.io/controller-runtime/pkg/scheme"

	"github.com/dell/csi-baremetal/api/v1"
)

var (
	// GroupVersionAvailableCapacity is group version used to register these objects
	GroupVersionACR = schema.GroupVersion{Group: v1.CSICRsGroupVersion, Version: v1.Version}

	// SchemeBuilderACR is used to add go types to the GroupVersionKind scheme
	SchemeBuilderACR = &crScheme.Builder{GroupVersion: GroupVersionACR}

	// AddToSchemeAvailableCapacity adds the types in this group-version to the given scheme.
	AddToSchemeACR = SchemeBuilderACR.AddToScheme
)
