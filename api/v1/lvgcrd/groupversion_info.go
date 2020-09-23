// Package lvgcrd contains API Schema definitions for the LVG v1 API group
// +groupName=baremetal-csi.dellemc.com
// +versionName=v1
package lvgcrd

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	crScheme "sigs.k8s.io/controller-runtime/pkg/scheme"

	"github.com/dell/csi-baremetal/api/v1"
)

var (
	// GroupVersionLVG is group version used to register these objects
	GroupVersionLVG = schema.GroupVersion{Group: v1.CSICRsGroupVersion, Version: v1.Version}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilderLVG = &crScheme.Builder{GroupVersion: GroupVersionLVG}

	// AddToSchemeLVG adds the types in this group-version to the given scheme.
	AddToSchemeLVG = SchemeBuilderLVG.AddToScheme
)
