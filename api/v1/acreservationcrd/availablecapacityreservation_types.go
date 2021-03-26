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

package acrcrd

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/dell/csi-baremetal/api/generated/v1"
)

// +kubebuilder:object:root=true

// +kubebuilder:resource:scope=Cluster,shortName={acr,acrs}
// +kubebuilder:printcolumn:name="STORAGE CLASS",type="string",JSONPath=".spec.StorageClass",description="StorageClass of AvailableCapacityReservation"
// +kubebuilder:printcolumn:name="SIZE",type="string",JSONPath=".spec.SIZE",description="Size of AvailableCapacityReservation"
// +kubebuilder:printcolumn:name="RESERVATIONS",type="string",JSONPath=".spec.Reservations",description="List of reserved AvailableCapacity"
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
	out.Spec = in.Spec
}
