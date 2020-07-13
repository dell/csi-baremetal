package accrd

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/dell/csi-baremetal/api/generated/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +kubebuilder:object:root=true

// AvailableCapacity is the Schema for the availablecapacities API
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
