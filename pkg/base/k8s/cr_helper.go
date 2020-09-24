package k8s

import (
	"context"
	"errors"
	"strings"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
)

// CRHelper is able to collect different CRs by different criteria
type CRHelper struct {
	k8sClient *KubeClient
	log       *logrus.Entry
}

// NewCRHelper is a constructor for CRHelper instance
func NewCRHelper(k *KubeClient, logger *logrus.Logger) *CRHelper {
	return &CRHelper{
		k8sClient: k,
		log:       logger.WithField("component", "CRHelper"),
	}
}

// GetACByLocation reads the whole list of AC CRs from a cluster and searches the AC with provided location
// Receive context and location name which should be equal to AvailableCapacity.Spec.Location
// Returns a pointer to the instance of accrd.AvailableCapacity or nil
func (cs *CRHelper) GetACByLocation(location string) *accrd.AvailableCapacity {
	ll := cs.log.WithFields(logrus.Fields{
		"method":   "GetACByLocation",
		"location": location,
	})

	acList := &accrd.AvailableCapacityList{}
	if err := cs.k8sClient.ReadList(context.Background(), acList); err != nil {
		ll.Errorf("Failed to get available capacity CR list, error %v", err)
		return nil
	}

	for _, ac := range acList.Items {
		if strings.EqualFold(ac.Spec.Location, location) {
			return &ac
		}
	}

	ll.Warn("Can't find AC assigned to provided location")

	return nil
}

// DeleteACsByNodeID deletes AC CRs for specific node ID
// Receives unique identifier of the node
// Returns error or nil
func (cs *CRHelper) DeleteACsByNodeID(nodeID string) error {
	ll := cs.log.WithFields(logrus.Fields{"method": "DeleteACsByNodeID", "nodeID": nodeID})

	acList := &accrd.AvailableCapacityList{}
	if err := cs.k8sClient.ReadList(context.Background(), acList); err != nil {
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

// GetVolumeByLocation reads the whole list of Volume CRs from a cluster and searches the volume with provided location
// Receives golang context and location name which should be equal to Volume.Spec.Location
// Returns a pointer to the instance of volumecrd.Volume or nil
func (cs *CRHelper) GetVolumeByLocation(location string) *volumecrd.Volume {
	ll := cs.log.WithFields(logrus.Fields{
		"method":   "GetVolumeByLocation",
		"location": location,
	})

	volList := &volumecrd.VolumeList{}
	if err := cs.k8sClient.ReadList(context.Background(), volList); err != nil {
		ll.Errorf("Failed to get volume CR list, error %v", err)
		return nil
	}

	for _, v := range volList.Items {
		if strings.EqualFold(v.Spec.Location, location) {
			return &v
		}
	}

	ll.Warn("Can't find VolumeCR assigned to provided location")

	return nil
}

// UpdateVolumesOpStatusOnNode updates operational status of volumes on a node without taking into account current state
// Receives unique identifier of the node and operational status to be set
// Returns error or nil
func (cs *CRHelper) UpdateVolumesOpStatusOnNode(nodeID, opStatus string) error {
	ll := cs.log.WithFields(logrus.Fields{"method": "UpdateVolumesOpStatus", "nodeID": nodeID})
	// todo check that operational status is valid
	volumes, err := cs.GetVolumeCRs(nodeID)
	if err != nil {
		return err
	}

	isError := false
	for _, volume := range volumes {
		if volume.Spec.OperationalStatus != opStatus {
			volume.Spec.OperationalStatus = opStatus
			ctxWithID := context.WithValue(context.Background(), RequestUUID, volume.Spec.Id)
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
func (cs *CRHelper) GetVolumeByID(volID string) *volumecrd.Volume {
	volumeCRs, _ := cs.GetVolumeCRs()
	for _, v := range volumeCRs {
		if v.Spec.Id == volID {
			return &v
		}
	}

	cs.log.WithFields(logrus.Fields{
		"method":   "GetVolumeByID",
		"volumeID": volID,
	}).Infof("Volume CR isn't exist")
	return nil
}

// GetVolumeCRs collect volume CRs that locate on node, use just node[0] element
// if node isn't provided - return all volume CRs
// if error occurs - return nil and error
func (cs *CRHelper) GetVolumeCRs(node ...string) ([]volumecrd.Volume, error) {
	var (
		vList = &volumecrd.VolumeList{}
		err   error
	)

	if err = cs.k8sClient.ReadList(context.Background(), vList); err != nil {
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
	// todo check that drive status is valid
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

// GetDriveCRs collect drive CRs that locate on node, use just node[0] element
// if node isn't provided - return all volume CRs
// if error occurs - return nil and error
func (cs *CRHelper) GetDriveCRs(node ...string) ([]drivecrd.Drive, error) {
	var (
		dList = &drivecrd.DriveList{}
		err   error
	)

	if err = cs.k8sClient.ReadList(context.Background(), dList); err != nil {
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

// GetDriveCRByUUID reads drive CRs and returns drive CR with uuid dUUID
func (cs *CRHelper) GetDriveCRByUUID(dUUID string) *drivecrd.Drive {
	driveCRs, _ := cs.GetDriveCRs()
	for _, d := range driveCRs {
		if d.Spec.UUID == dUUID {
			return &d
		}
	}

	cs.log.WithFields(logrus.Fields{
		"method":    "GetDriveCRByUUID",
		"driveUUID": dUUID,
	}).Infof("Drive CR isn't exist")
	return nil
}

// GetVGNameByLVGCRName read LVG CR with name lvgCRName and returns LVG CR.Spec.Name
// method is used for LVG based on system VG because system VG name != LVG CR name
// in case of error returns empty string and error
func (cs *CRHelper) GetVGNameByLVGCRName(lvgCRName string) (string, error) {
	lvgCR := lvgcrd.LVG{}
	if err := cs.k8sClient.ReadCR(context.Background(), lvgCRName, &lvgCR); err != nil {
		return "", err
	}
	return lvgCR.Spec.Name, nil
}

// GetLVGCRs collect LVG CRs that locate on node, use just node[0] element
// if node isn't provided - return all volume CRs
// if error occurs - return nil
func (cs *CRHelper) GetLVGCRs(node ...string) []lvgcrd.LVG {
	var (
		lvgList = &lvgcrd.LVGList{}
		err     error
	)

	if err = cs.k8sClient.ReadList(context.Background(), lvgList); err != nil {
		cs.log.WithField("method", "GetLVGCRs").
			Errorf("Unable to read volume CRs list: %v", err)
		return nil
	}

	if len(node) == 0 {
		return lvgList.Items
	}

	// if node was provided, collect LVGs that are on that node
	res := make([]lvgcrd.LVG, 0)
	for _, l := range lvgList.Items {
		if l.Spec.Node == node[0] {
			res = append(res, l)
		}
	}
	return res
}

// UpdateVolumeCRSpec reads volume CR with name volName and update it's spec to newSpec
// returns nil or error in case of error
func (cs *CRHelper) UpdateVolumeCRSpec(volName string, newSpec api.Volume) error {
	var (
		volumeCR = &volumecrd.Volume{}
		err      error
	)

	ctxWithID := context.WithValue(context.Background(), RequestUUID, volumeCR.Spec.Id)
	if err = cs.k8sClient.ReadCR(ctxWithID, volName, volumeCR); err != nil {
		return err
	}

	volumeCR.Spec = newSpec
	return cs.k8sClient.UpdateCR(ctxWithID, volumeCR)
}

// DeleteObjectByName read runtime.Object by its name and then delete it
func (cs *CRHelper) DeleteObjectByName(name string, obj runtime.Object) error {
	if err := cs.k8sClient.ReadCR(context.Background(), name, obj); err != nil {
		return err
	}

	return cs.k8sClient.DeleteCR(context.Background(), obj)
}
