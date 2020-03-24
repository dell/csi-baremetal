// Package v1 contains API Schema definitions for the LVG v1 API group
// +groupName=baremetal-csi.dellemc.com
// +versionName=v1
package lvgcrd

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
)

var (
	// GroupVersionLVG is group version used to register these objects
	GroupVersionLVG = schema.GroupVersion{Group: v1.CSICRsGroupVersion, Version: "v1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilderLVG = &scheme.Builder{GroupVersion: GroupVersionLVG}

	// AddToSchemeLVG adds the types in this group-version to the given scheme.
	AddToSchemeLVG = SchemeBuilderLVG.AddToScheme
)
