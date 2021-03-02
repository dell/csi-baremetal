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

package lvgcrd

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/dell/csi-baremetal/api/generated/v1"
)

// +kubebuilder:object:root=true

// +kubebuilder:resource:scope=Cluster
// +kubebuilder:resource:scope=Cluster,shortName={lvg,lvgs}
// LogicalVolumeGroup is the Schema for the LVGs API
type LogicalVolumeGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              api.LogicalVolumeGroup `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// LogicalVolumeGroupList contains a list of LogicalVolumeGroup
//+kubebuilder:object:generate=true
type LogicalVolumeGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LogicalVolumeGroup `json:"items"`
}

func init() {
	SchemeBuilderLVG.Register(&LogicalVolumeGroup{}, &LogicalVolumeGroupList{})
}

// Need to declare this method because api.LogicalVolumeGroup doesn't have DeepCopyInto
func (in *LogicalVolumeGroup) DeepCopyInto(out *LogicalVolumeGroup) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
}
