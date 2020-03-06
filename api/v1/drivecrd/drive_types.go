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

package drivecrd

import (
	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +kubebuilder:object:root=true

// Drive is the Schema for the drives API
//kubebuilder:object:generate=false
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
	in.Spec = out.Spec
}

func init() {
	SchemeBuilderDrive.Register(&Drive{}, &DriveList{})
}

func (d *Drive) Equals(drive *api.Drive) bool {
	return d.Spec.SerialNumber == drive.SerialNumber &&
		d.Spec.NodeId == drive.NodeId &&
		d.Spec.PID == drive.PID &&
		d.Spec.VID == drive.VID &&
		d.Spec.Status == drive.Status &&
		d.Spec.Health == drive.Health &&
		d.Spec.Type == drive.Type &&
		d.Spec.Size == drive.Size
}
