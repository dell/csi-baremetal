/*
Copyright © 2020 Dell Inc. or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package sgcrd contains API Schema definitions for the StorageGroup v1 API group
// +groupName=csi-baremetal.dell.com
// +versionName=v1
package sgcrd

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	crScheme "sigs.k8s.io/controller-runtime/pkg/scheme"

	v1 "github.com/dell/csi-baremetal/api/v1"
)

var (
	// GroupVersionStorageGroup is group version used to register these objects
	GroupVersionStorageGroup = schema.GroupVersion{Group: v1.CSICRsGroupVersion, Version: v1.Version}

	// SchemeBuilderStorageGroup is used to add go types to the GroupVersionKind scheme
	SchemeBuilderStorageGroup = &crScheme.Builder{GroupVersion: GroupVersionStorageGroup}

	// AddToSchemeStorageGroup adds the types in this group-version to the given scheme.
	AddToSchemeStorageGroup = SchemeBuilderStorageGroup.AddToScheme
)
