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

package drivecrd

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/dell/csi-baremetal/api/generated/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +kubebuilder:object:root=true

// Drive is the Schema for the drives API
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="SIZE",type="string",JSONPath=".spec.Size",description="Drive capacity"
// +kubebuilder:printcolumn:name="TYPE",type="string",JSONPath=".spec.Type",description="Drive type (HDD/LVG/NVME)"
// +kubebuilder:printcolumn:name="HEALTH",type="string",JSONPath=".spec.Health",description="Drive health status"
// +kubebuilder:printcolumn:name="STATUS",type="string",JSONPath=".spec.Status",description="Drive status online/offline",priority=1
// +kubebuilder:printcolumn:name="USAGE",type="string",JSONPath=".spec.Usage",description="Drive usage",priority=1
// +kubebuilder:printcolumn:name="SYSTEM",type="string",JSONPath=".spec.IsSystem",description="Is system disk",priority=1
// +kubebuilder:printcolumn:name="PATH",type="string",JSONPath=".spec.Path",description="Drive path",priority=1
// +kubebuilder:printcolumn:name="SERIAL NUMBER",type="string",JSONPath=".spec.SerialNumber",description="Drive serial number"
// +kubebuilder:printcolumn:name="NODE",type="string",JSONPath=".spec.NodeId",description="Drive node location"
// +kubebuilder:printcolumn:name="SLOT",type="string",JSONPath=".spec.Slot",description="Drive slot"
type Drive struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec api.Drive `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// DriveList contains a list of Drive
//+kubebuilder:object:generate=true
type DriveList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Drive `json:"items"`
}

//Need to declare this method because api.Volume doesn't have DeepCopyInto
func (in *Drive) DeepCopyInto(out *Drive) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
}

func init() {
	SchemeBuilderDrive.Register(&Drive{}, &DriveList{})
}

func (in *Drive) Equals(drive *api.Drive) bool {
	return in.Spec.SerialNumber == drive.SerialNumber &&
		in.Spec.NodeId == drive.NodeId &&
		in.Spec.PID == drive.PID &&
		in.Spec.VID == drive.VID &&
		in.Spec.Status == drive.Status &&
		in.Spec.Health == drive.Health &&
		in.Spec.Type == drive.Type &&
		in.Spec.Size == drive.Size &&
		in.Spec.Path == drive.Path
}

func (in *Drive) GetDriveDescription() string {
	spec := in.Spec
	description := fmt.Sprintf("Drive Details: SN='%s', Model='%s %s', Type='%s', Size='%d'",
		spec.SerialNumber, spec.VID, spec.PID, spec.Type, spec.Size)
	// add firmware info only if detected
	if spec.Firmware != "" {
		description += fmt.Sprintf(", Firmware='%s'", spec.Firmware)
	}
	// add Slot info only if detected
	if spec.Slot != "" {
		description += fmt.Sprintf(", Slot='%s'", spec.Slot)
	}
	return description
}
