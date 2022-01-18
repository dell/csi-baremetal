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

package volumecrd

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/dell/csi-baremetal/api/generated/v1"
)

// +kubebuilder:object:root=true

// Volume is the Schema for the volumes API
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="SIZE",type="string",JSONPath=".spec.Size",description="Volume allocated size"
// +kubebuilder:printcolumn:name="STORAGE CLASS",type="string",JSONPath=".spec.StorageClass",description="Volume storage class"
// +kubebuilder:printcolumn:name="HEALTH",type="string",JSONPath=".spec.Health",description="Volume health status"
// +kubebuilder:printcolumn:name="CSI_STATUS",type="string",JSONPath=".spec.CSIStatus",description="Volume internal CSI status"
// +kubebuilder:printcolumn:name="OP_STATUS",type="string",JSONPath=".spec.OperationalStatus",description="Volume operational status",priority=1
// +kubebuilder:printcolumn:name="USAGE",type="string",JSONPath=".spec.Usage",description="Volume usage status",priority=1
// +kubebuilder:printcolumn:name="TYPE",type="string",JSONPath=".spec.Type",description="Volume fs type",priority=1
// +kubebuilder:printcolumn:name="LOCATION",type="string",JSONPath=".spec.Location",description="Volume LVG or drive location"
// +kubebuilder:printcolumn:name="NODE",type="string",JSONPath=".spec.NodeId",description="Volume node location"
type Volume struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec api.Volume `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// VolumeList contains a list of Volume
//+kubebuilder:object:generate=true
type VolumeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Volume `json:"items"`
}

// Need to declare this method because api.Volume doesn't have DeepCopyInto
func (in *Volume) DeepCopyInto(out *Volume) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
}

func init() {
	SchemeBuilder.Register(&Volume{}, &VolumeList{})
}
