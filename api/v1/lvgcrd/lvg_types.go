package lvgcrd

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/dell/csi-baremetal/api/generated/v1"
)

// +kubebuilder:object:root=true

// LVG is the Schema for the LVGs API
type LVG struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              api.LogicalVolumeGroup `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// LVGList contains a list of LVG
//+kubebuilder:object:generate=true
type LVGList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LVG `json:"items"`
}

func init() {
	SchemeBuilderLVG.Register(&LVG{}, &LVGList{})
}

//Need to declare this method because api.LogicalVolumeGroup doesn't have DeepCopyInto
func (in *LVG) DeepCopyInto(out *LVG) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec = out.Spec
}
