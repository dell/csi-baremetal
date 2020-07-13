package volumecrd

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/dell/csi-baremetal/api/generated/v1"
)

// +kubebuilder:object:root=true

// Volume is the Schema for the volumes API
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

//Need to declare this method because api.Volume doesn't have DeepCopyInto
func (in *Volume) DeepCopyInto(out *Volume) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec = out.Spec
}

func init() {
	SchemeBuilder.Register(&Volume{}, &VolumeList{})
}
