/*


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

// Package v1 contains API Schema definitions for the dell.com v1 API group
// +groupName=drive.dell.com
// +versionName=v1
package drivecrd

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersionDrive is group version used to register these objects
	GroupVersionDrive = schema.GroupVersion{Group: "drive.dell.com", Version: "v1"}

	// SchemeBuilderDrive is used to add go types to the GroupVersionKind scheme
	SchemeBuilderDrive = &scheme.Builder{GroupVersion: GroupVersionDrive}

	// AddToSchemeDrive adds the types in this group-version to the given scheme.
	AddToSchemeDrive = SchemeBuilderDrive.AddToScheme
)
