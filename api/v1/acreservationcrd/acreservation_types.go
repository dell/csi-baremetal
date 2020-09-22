package acreservationcrd

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/dell/csi-baremetal/api/generated/v1"
)

// +kubebuilder:object:root=true

// ACReservation is the Schema for the availablecapacitiereservations API
type ACReservation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              api.AvailableCapacityReservation `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// ACReservationList contains a list of ACReservations
//+kubebuilder:object:generate=true
type ACReservationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ACReservation `json:"items"`
}

func init() {
	SchemeBuilderACR.Register(&ACReservation{}, &ACReservationList{})
}

func (in *ACReservation) DeepCopyInto(out *ACReservation) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec = out.Spec
}
