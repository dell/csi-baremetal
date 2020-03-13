// Package v1 contains API Schema definitions for the volume v1 API group
// +groupName=lvg.dell.com
// +versionName=v1
package lvgcrd

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersionLVG is group version used to register these objects
	GroupVersionLVG = schema.GroupVersion{Group: "lvg.dell.com", Version: "v1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilderLVG = &scheme.Builder{GroupVersion: GroupVersionLVG}

	// AddToSchemeLVG adds the types in this group-version to the given scheme.
	AddToSchemeLVG = SchemeBuilderLVG.AddToScheme
)
