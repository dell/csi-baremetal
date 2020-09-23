package acrcrd

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/dell/csi-baremetal/api/generated/v1"
)

// +kubebuilder:object:root=true

// AvailableCapacityReservation is the Schema for the availablecapacitiereservations API
type AvailableCapacityReservation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              api.AvailableCapacityReservation `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// AvailableCapacityReservationList contains a list of AvailableCapacityReservation
//+kubebuilder:object:generate=true
type AvailableCapacityReservationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AvailableCapacityReservation `json:"items"`
}

func init() {
	SchemeBuilderACR.Register(&AvailableCapacityReservation{}, &AvailableCapacityReservationList{})
}

func (in *AvailableCapacityReservation) DeepCopyInto(out *AvailableCapacityReservation) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec = out.Spec
}
