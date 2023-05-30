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

package sgcrd

import (
	api "github.com/dell/csi-baremetal/api/generated/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true

// StorageGroup is the Schema for the StorageGroups API
// +kubebuilder:resource:scope=Cluster,shortName={sg,sgs}
// +kubebuilder:printcolumn:name="DRIVES_PER_NODE",type="string",JSONPath=".spec.driveSelector.numberDrivesPerNode",description="numberDrivesPerNode of StorageGroup's DriveSelector"
// +kubebuilder:printcolumn:name="TYPE",type="string",JSONPath=".spec.driveSelector.matchFields.Type",description="Drive Type of StorageGroup's DriveSelector"
// +kubebuilder:printcolumn:name="SLOT",type="string",JSONPath=".spec.driveSelector.matchFields.Slot",description="Drive Slot of StorageGroup's DriveSelector"
// +kubebuilder:printcolumn:name="PATH",type="string",JSONPath=".spec.driveSelector.matchFields.Path",description="Drive Path of StorageGroup's DriveSelector"
// +kubebuilder:printcolumn:name="SYSTEM",type="string",JSONPath=".spec.driveSelector.matchFields.IsSystem",description="Whether StorageGroup's DriveSelector to Select System Drive"
type StorageGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              api.StorageGroupSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// StorageGroupList contains a list of StorageGroup
//+kubebuilder:object:generate=true
type StorageGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StorageGroup `json:"items"`
}

func init() {
	SchemeBuilderStorageGroup.Register(&StorageGroup{}, &StorageGroupList{})
}

func (in *StorageGroup) DeepCopyInto(out *StorageGroup) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
}
