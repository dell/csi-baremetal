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

	"github.com/dell/csi-baremetal/api/generated/v1/api"
)

// +kubebuilder:object:root=true

// AvailableCapacityReservation is the Schema for the availablecapacitiereservations API
// +kubebuilder:resource:scope=Cluster,shortName={acr,acrs}
// +kubebuilder:printcolumn:name="NAMESPACE",type="string",JSONPath=".spec.Namespace",description="Pod namespace"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".spec.Status",description="Status of AvailableCapacityReservation"
// +kubebuilder:printcolumn:name="REQUESTED NODES",type="string",JSONPath=".spec.NodeRequests.Requested",description="List of requested nodes",priority=1
// +kubebuilder:printcolumn:name="RESERVED NODES",type="string",JSONPath=".spec.NodeRequests.Reserved",description="List of reserved nodes",priority=1
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
