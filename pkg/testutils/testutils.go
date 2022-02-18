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

package testutils

import (
	"context"
	"time"

	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
)

// VolumeReconcileImitation looking for volume CR with name volId and sets it's status to newStatus
func VolumeReconcileImitation(k8sClient *k8s.KubeClient, volID string, namespace string, newStatus string) {
	for {
		<-time.After(200 * time.Millisecond)
		err := ReadVolumeAndChangeStatus(k8sClient, volID, namespace, newStatus)
		if err != nil {
			return
		}
	}
}

// AddAC create test AvailableCapacities
func AddAC(k8sClient *k8s.KubeClient, acs ...*accrd.AvailableCapacity) error {
	for _, ac := range acs {
		if err := k8sClient.CreateCR(context.Background(), ac.Name, ac); err != nil {
			return err
		}
	}
	return nil
}

// ReadVolumeAndChangeStatus returns error if something went wrong
func ReadVolumeAndChangeStatus(k8sClient *k8s.KubeClient, volumeID string, namespace string, newStatus string) error {
	var (
		v        = &volumecrd.Volume{}
		attempts = 10
		ctx      = context.WithValue(context.Background(), base.RequestUUID, volumeID)
	)

	if err := k8sClient.ReadCR(ctx, volumeID, namespace, v, &k8s.KubeClientRequestOptions{
		MaxBackoffRetries: &attempts,
	}); err != nil {
		return err
	}

	// change status
	v.Spec.CSIStatus = newStatus
	if err := k8sClient.UpdateCR(ctx, v, &k8s.KubeClientRequestOptions{
		MaxBackoffRetries: &attempts,
	}); err != nil {
		return err
	}
	return nil
}
