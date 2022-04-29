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

package k8s

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	k8sCl "sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
	"github.com/dell/csi-baremetal/pkg/base"
	errTypes "github.com/dell/csi-baremetal/pkg/base/error"
)

// CRHelper is able to collect different CRs by different criteria
type CRHelper struct {
	k8sClient *KubeClient
	reader    CRReader
	log       *logrus.Entry
}

// NewCRHelper is a constructor for CRHelper instance
func NewCRHelper(k *KubeClient, logger *logrus.Logger) *CRHelper {
	return &CRHelper{
		k8sClient: k,
		reader:    k,
		log:       logger.WithField("component", "CRHelper"),
	}
}

// SetReader allow to set separate CRReader for CRHelper, by default k8sClient will be used
func (cs *CRHelper) SetReader(reader CRReader) *CRHelper {
	cs.reader = reader
	return cs
}

// GetACByLocation reads the whole list of AC CRs from a cluster and searches the AC with provided location
// Receive context and location name which should be equal to AvailableCapacity.Spec.Location
// Returns a pointer to the instance of accrd.AvailableCapacity or nil
func (cs *CRHelper) GetACByLocation(location string) (*accrd.AvailableCapacity, error) {
	ll := cs.log.WithFields(logrus.Fields{
		"method":   "GetACByLocation",
		"location": location,
	})

	acList := &accrd.AvailableCapacityList{}
	if err := cs.reader.ReadList(context.Background(), acList); err != nil {
		ll.Errorf("Failed to get available capacity CR list, error %v", err)
		return nil, err
	}

	for _, ac := range acList.Items {
		if strings.EqualFold(ac.Spec.Location, location) {
			return &ac, nil
		}
	}

	ll.Warn("Can't find AC assigned to provided location: ", location)

	return nil, errTypes.ErrorNotFound
}

// DeleteACsByNodeID deletes AC CRs for specific node ID
// Receives unique identifier of the node
// Returns error or nil
func (cs *CRHelper) DeleteACsByNodeID(nodeID string) error {
	ll := cs.log.WithFields(logrus.Fields{"method": "DeleteACsByNodeID", "nodeID": nodeID})

	acList := &accrd.AvailableCapacityList{}
	if err := cs.reader.ReadList(context.Background(), acList); err != nil {
		ll.Errorf("Failed to get available capacity CR list, error %v", err)
		return err
	}

	// delete all ACs for specified node id if any.
	isError := false
	for _, ac := range acList.Items {
		if strings.EqualFold(ac.Spec.NodeId, nodeID) {
			// todo fix linter issue - https://github.com/kyoh86/scopelint/issues/5
			// nolint:scopelint
			if err := cs.k8sClient.DeleteCR(context.Background(), &ac); err != nil {
				ll.Warningf("Unable to delete AC %s: %s", ac.Name, err)
				isError = true
			}
		}
	}

	// return error when unable to delete some/all ACs
	if isError {
		return errors.New("failed to delete some custom resources")
	}
	return nil
}

// GetVolumesByLocation reads the whole list of Volume CRs from a cluster and searches the volume with provided location
// Receives golang context and location name which should be equal to Volume.Spec.Location
// Returns a list of a pointers to volumes which are belong to the location and error
func (cs *CRHelper) GetVolumesByLocation(ctx context.Context, location string) ([]*volumecrd.Volume, error) {
	ll := cs.log.WithFields(logrus.Fields{
		"method":   "GetVolumesByLocation",
		"location": location,
	})

	var volumes []*volumecrd.Volume
	volList := &volumecrd.VolumeList{}
	if err := cs.reader.ReadList(ctx, volList); err != nil {
		ll.Errorf("Failed to get volume CR list, error %v", err)
		return nil, err
	}
	lvg, err := cs.GetLVGByDrive(ctx, location)
	if err != nil {
		ll.Errorf("Failed to get LogicalVolumeGroup UUID for drive, error %v", err)
		return nil, err
	}

	if lvg != nil {
		location = lvg.Name
	}

	for _, v := range volList.Items {
		v := v
		if strings.EqualFold(v.Spec.Location, location) {
			volumes = append(volumes, &v)
			if v.Spec.LocationType == apiV1.LocationTypeDrive {
				// only one volume with LocationTypeDrive can exist on drive
				break
			}
		}
	}
	if len(volumes) == 0 {
		ll.Warn("Can't find VolumeCR assigned to provided location")
	}
	return volumes, nil
}

// GetLVGByDrive reads list of LogicalVolumeGroup CRs from a cluster and searches the lvg with provided location
// Receives golang context and drive uuid
// Returns found lvg and error
func (cs *CRHelper) GetLVGByDrive(ctx context.Context, driveUUID string) (*lvgcrd.LogicalVolumeGroup, error) {
	ll := cs.log.WithFields(logrus.Fields{
		"method":    "GetLVGByDrive",
		"driveUUID": driveUUID,
	})
	lvgList := &lvgcrd.LogicalVolumeGroupList{}
	if err := cs.reader.ReadList(ctx, lvgList); err != nil {
		ll.Errorf("Failed to get LogicalVolumeGroup CR list, error %v", err)
		return nil, err
	}
	for _, lvg := range lvgList.Items {
		if len(lvg.Spec.Locations) > 0 && lvg.Spec.Locations[0] == driveUUID {
			return &lvg, nil
		}
	}
	return nil, nil
}

// UpdateVolumesOpStatusOnNode updates operational status of volumes on a node without taking into account current state
// Receives unique identifier of the node and operational status to be set
// Returns error or nil
func (cs *CRHelper) UpdateVolumesOpStatusOnNode(nodeID, opStatus string) error {
	ll := cs.log.WithFields(logrus.Fields{"method": "UpdateVolumesOpStatus", "nodeID": nodeID})
	// TODO: check that operational status is valid https://github.com/dell/csi-baremetal/issues/80
	volumes, err := cs.GetVolumeCRs(nodeID)
	if err != nil {
		return err
	}

	isError := false
	for _, volume := range volumes {
		if volume.Spec.OperationalStatus != opStatus {
			volume.Spec.OperationalStatus = opStatus
			ctxWithID := context.WithValue(context.Background(), base.RequestUUID, volume.Spec.Id)
			// todo fix linter issue - https://github.com/kyoh86/scopelint/issues/5
			// nolint:scopelint
			if err := cs.k8sClient.UpdateCR(ctxWithID, &volume); err != nil {
				ll.Errorf("Unable to update operational status for volume ID %s: %s", volume.Spec.Id, err)
				isError = true
			}
		}
	}

	// return error when unable to delete some/all ACs
	if isError {
		return errors.New("failed to update some custom resources")
	}
	return nil
}

// GetVolumeByID reads volume CRs and returns volumes CR if it .Spec.Id == volId
func (cs *CRHelper) GetVolumeByID(volID string) (*volumecrd.Volume, error) {
	volumeCRs, err := cs.GetVolumeCRs()
	if err != nil {
		return nil, err
	}
	for _, v := range volumeCRs {
		if v.Spec.Id == volID {
			return &v, nil
		}
	}

	cs.log.WithFields(logrus.Fields{
		"method":   "GetVolumeByID",
		"volumeID": volID,
	}).Infof("Volume CR not found")
	return nil, fmt.Errorf("volume wasn't found")
}

// GetVolumeCRs collect volume CRs that locate on node, use just node[0] element
// if node isn't provided - return all volume CRs
// if error occurs - return nil and error
func (cs *CRHelper) GetVolumeCRs(node ...string) ([]volumecrd.Volume, error) {
	var (
		vList = &volumecrd.VolumeList{}
		err   error
	)

	if err = cs.reader.ReadList(context.Background(), vList); err != nil {
		return nil, err
	}

	if len(node) == 0 {
		return vList.Items, nil
	}

	// if node was provided, collect volumes that are on that node
	res := make([]volumecrd.Volume, 0)
	for _, v := range vList.Items {
		if v.Spec.NodeId == node[0] {
			res = append(res, v)
		}
	}
	return res, nil
}

// UpdateDrivesStatusOnNode updates status of drives on a node without taking into account current state
// Receives unique identifier of the node and status to be set
// Returns error or nil
func (cs *CRHelper) UpdateDrivesStatusOnNode(nodeID, status string) error {
	ll := cs.log.WithFields(logrus.Fields{"method": "UpdateDrivesStatusOnNode", "nodeID": nodeID})
	// TODO: check that drive status is valid - https://github.com/dell/csi-baremetal/issues/80
	drives, _ := cs.GetDriveCRs(nodeID)
	// node might not have drives reported to CSI. For example, filtered in drive manager level
	if drives == nil {
		return nil
	}

	isError := false
	for _, drive := range drives {
		if drive.Spec.Status != status {
			drive.Spec.Status = status
			// todo fix linter issue - https://github.com/kyoh86/scopelint/issues/5
			// nolint:scopelint
			if err := cs.k8sClient.UpdateCR(context.Background(), &drive); err != nil {
				ll.Errorf("Unable to update status for drive ID %s: %s", drive.Spec.UUID, err)
				isError = true
			}
		}
	}

	// return error when unable to update some/all ACs
	if isError {
		return errors.New("failed to update some custom resources")
	}
	return nil
}

// GetDriveCRs collect Drives CR that locate on node, use just node[0] element
// if node isn't provided - return all Drives CR
// if error occurs - return nil and error
func (cs *CRHelper) GetDriveCRs(node ...string) ([]drivecrd.Drive, error) {
	var (
		dList = &drivecrd.DriveList{}
		err   error
	)

	if err = cs.reader.ReadList(context.Background(), dList); err != nil {
		return nil, err
	}

	if len(node) == 0 {
		return dList.Items, nil
	}

	// if node was provided, collect drives that are on that node
	res := make([]drivecrd.Drive, 0)
	for _, d := range dList.Items {
		if d.Spec.NodeId == node[0] {
			res = append(res, d)
		}
	}
	return res, nil
}

// GetACCRs collect ACs CR that locate on node, use just node[0] element
// if node isn't provided - return all ACs CR
// if error occurs - return nil and error
func (cs *CRHelper) GetACCRs(node ...string) ([]accrd.AvailableCapacity, error) {
	var (
		acsList = &accrd.AvailableCapacityList{}
		err     error
	)

	if err = cs.reader.ReadList(context.Background(), acsList); err != nil {
		return nil, err
	}

	if len(node) == 0 {
		return acsList.Items, nil
	}

	// if node was provided, collect drives that are on that node
	res := make([]accrd.AvailableCapacity, 0)
	for _, ac := range acsList.Items {
		if ac.Spec.NodeId == node[0] {
			res = append(res, ac)
		}
	}
	return res, nil
}

// GetDriveCRByVolume reads drive CRs and returns CR for drive on which volume is located
func (cs *CRHelper) GetDriveCRByVolume(volume *volumecrd.Volume) (*drivecrd.Drive, error) {
	drive, _, err := cs.GetDriveCRAndLVGCRByVolume(volume)
	return drive, err
}

// GetDriveCRAndLVGCRByVolume reads drive CRs and returns CR for drive and CR for lvg (if exist) on which volume is located
func (cs *CRHelper) GetDriveCRAndLVGCRByVolume(volume *volumecrd.Volume) (*drivecrd.Drive, *lvgcrd.LogicalVolumeGroup, error) {
	ll := cs.log.WithFields(logrus.Fields{
		"method": "GetDriveCRByVolume",
		"volume": volume.Name,
	})

	var (
		dUUID = volume.Spec.Location
		drive = &drivecrd.Drive{}
		lvg   = &lvgcrd.LogicalVolumeGroup{}
		ctx   = context.Background()
	)

	if volume.Spec.LocationType == apiV1.LocationTypeLVM {
		err := cs.reader.ReadCR(ctx, volume.Spec.Location, "", lvg)
		if err != nil {
			ll.Errorf("failed to read LogicalVolumeGroup CR: %s", err.Error())
			return nil, nil, err
		}
		if len(lvg.Spec.Locations) == 0 {
			return nil, nil, errors.New("no drives in LogicalVolumeGroup CR")
		}
		dUUID = lvg.Spec.Locations[0]
	} else {
		lvg = nil
	}

	err := cs.reader.ReadCR(ctx, dUUID, "", drive)
	if err != nil {
		ll.Errorf("failed to read Drive CR: %v", err)
		return nil, lvg, err
	}

	return drive, lvg, err
}

// GetVGNameByLVGCRName read LogicalVolumeGroup CR with name lvgCRName and returns LogicalVolumeGroup CR.Spec.Name
// method is used for LogicalVolumeGroup based on system VG because system VG name != LogicalVolumeGroup CR name
// in case of error returns empty string and error
func (cs *CRHelper) GetVGNameByLVGCRName(lvgCRName string) (string, error) {
	lvgCR := lvgcrd.LogicalVolumeGroup{}
	if err := cs.reader.ReadCR(context.Background(), lvgCRName, "", &lvgCR); err != nil {
		return "", err
	}
	return lvgCR.Spec.Name, nil
}

// GetLVGCRs collect LogicalVolumeGroup CRs that locate on node, use just node[0] element
// if node isn't provided - return all volume CRs
// if error occurs - return nil
func (cs *CRHelper) GetLVGCRs(node ...string) ([]lvgcrd.LogicalVolumeGroup, error) {
	var (
		lvgList = &lvgcrd.LogicalVolumeGroupList{}
		err     error
	)

	if err = cs.reader.ReadList(context.Background(), lvgList); err != nil {
		return nil, err
	}

	if len(node) == 0 {
		return lvgList.Items, nil
	}

	// if node was provided, collect LVGs that are on that node
	res := make([]lvgcrd.LogicalVolumeGroup, 0)
	for _, l := range lvgList.Items {
		if l.Spec.Node == node[0] {
			res = append(res, l)
		}
	}
	return res, nil
}

// UpdateVolumeCRSpec reads volume CR with name volName and update it's spec to newSpec
// returns nil or error in case of error
func (cs *CRHelper) UpdateVolumeCRSpec(volName string, namespace string, newSpec api.Volume) error {
	var (
		volumeCR = &volumecrd.Volume{}
		err      error
	)

	ctxWithID := context.WithValue(context.Background(), base.RequestUUID, volumeCR.Spec.Id)
	if err = cs.reader.ReadCR(ctxWithID, volName, namespace, volumeCR); err != nil {
		return err
	}

	volumeCR.Spec = newSpec
	return cs.k8sClient.UpdateCR(ctxWithID, volumeCR)
}

// DeleteObjectByName read runtime.Object by its name and then delete it
func (cs *CRHelper) DeleteObjectByName(ctx context.Context, name string, namespace string, obj k8sCl.Object) error {
	if err := cs.reader.ReadCR(ctx, name, namespace, obj); err != nil {
		if k8sError.IsNotFound(err) {
			return nil
		}
		return err
	}

	return cs.k8sClient.DeleteCR(context.Background(), obj)
}

// UpdateVolumeOpStatus Update volume Operational status to opStatus
// returns nil or error in case of error
func (cs *CRHelper) UpdateVolumeOpStatus(ctx context.Context, volume *volumecrd.Volume, opStatus string) error {
	ll := cs.log.WithFields(logrus.Fields{
		"method":   "UpdateVolumeOpStatus",
		"volume":   volume.Name,
		"opStatus": opStatus,
	})

	if volume.Spec.OperationalStatus != opStatus {
		volume.Spec.OperationalStatus = opStatus
		if err := cs.k8sClient.UpdateCR(ctx, volume); err != nil {
			ll.Errorf("Unable to update operational status for volume ID %s: %s", volume.Spec.Id, err)
			return err
		}
	}
	return nil
}
