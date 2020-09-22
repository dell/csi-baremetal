// Package accrd contains API Schema definitions for the available capacity v1 API group
// +groupName=baremetal-csi.dellemc.com
// +versionName=v1
package accrd

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	crScheme "sigs.k8s.io/controller-runtime/pkg/scheme"

	"github.com/dell/csi-baremetal/api/v1"
)

var (
	// GroupVersionAvailableCapacity is group version used to register these objects
	GroupVersionAvailableCapacity = schema.GroupVersion{Group: v1.CSICRsGroupVersion, Version: v1.Version}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilderAvailableCapacity = &crScheme.Builder{GroupVersion: GroupVersionAvailableCapacity}

	// AddToSchemeAvailableCapacity adds the types in this group-version to the given scheme.
	AddToSchemeAvailableCapacity = SchemeBuilderAvailableCapacity.AddToScheme
)
