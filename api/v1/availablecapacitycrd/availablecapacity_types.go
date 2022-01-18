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

package accrd

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/dell/csi-baremetal/api/generated/v1"
)

// +kubebuilder:object:root=true

// AvailableCapacity is the Schema for the availablecapacities API
// +kubebuilder:resource:scope=Cluster,shortName={ac,acs}
// +kubebuilder:printcolumn:name="SIZE",type="string",JSONPath=".spec.Size",description="Size of AvailableCapacity"
// +kubebuilder:printcolumn:name="STORAGE CLASS",type="string",JSONPath=".spec.storageClass",description="StorageClass of AvailableCapacity"
// +kubebuilder:printcolumn:name="LOCATION",type="string",JSONPath=".spec.Location",description="Drive/LVG UUID used by AvailableCapacity"
// +kubebuilder:printcolumn:name="NODE",type="string",JSONPath=".spec.NodeId",description="Node id of Available Capacity"
type AvailableCapacity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              api.AvailableCapacity `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// AvailableCapacityList contains a list of AvailableCapacity
//+kubebuilder:object:generate=true
type AvailableCapacityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AvailableCapacity `json:"items"`
}

func init() {
	SchemeBuilderAvailableCapacity.Register(&AvailableCapacity{}, &AvailableCapacityList{})
}

func (in *AvailableCapacity) DeepCopyInto(out *AvailableCapacity) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
}
