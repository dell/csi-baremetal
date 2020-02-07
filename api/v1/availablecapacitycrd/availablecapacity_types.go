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

package accrd

import (
	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +kubebuilder:object:root=true

// AvailableCapacity is the Schema for the availablecapacities API
//kubebuilder:object:generate=false
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
	in.Spec = out.Spec
}
