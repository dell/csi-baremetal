package k8s

import (
	"context"
	"strings"

	"github.com/sirupsen/logrus"

	accrd "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/availablecapacitycrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/drivecrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/lvgcrd"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1/volumecrd"
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

// GetVolumeByID reads volume CRs and returns volumes CR if it .Spec.Id == volId
func (cs *CRHelper) GetVolumeByID(volID string) *volumecrd.Volume {
	for _, v := range cs.GetVolumeCRs() {
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
// if error occurs - return nil
func (cs *CRHelper) GetVolumeCRs(node ...string) []volumecrd.Volume {
	var (
		vList = &volumecrd.VolumeList{}
		err   error
	)

	if err = cs.k8sClient.ReadList(context.Background(), vList); err != nil {
		cs.log.WithField("method", "GetVolumeCRs").
			Errorf("Unable to read volume CRs list: %v", err)
		return nil
	}

	if len(node) == 0 {
		return vList.Items
	}

	// if node was provided, collect volumes that are on that node
	res := make([]volumecrd.Volume, 0)
	for _, v := range vList.Items {
		if v.Spec.NodeId == node[0] {
			res = append(res, v)
		}
	}
	return res
}

// GetDriveCRs collect drive CRs that locate on node, use just node[0] element
// if node isn't provided - return all volume CRs
// if error occurs - return nil
func (cs *CRHelper) GetDriveCRs(node ...string) []drivecrd.Drive {
	var (
		dList = &drivecrd.DriveList{}
		err   error
	)

	if err = cs.k8sClient.ReadList(context.Background(), dList); err != nil {
		cs.log.WithField("method", "GetDriveCRs").
			Errorf("Unable to read drive CRs list: %v", err)
		return nil
	}

	if len(node) == 0 {
		return dList.Items
	}

	// if node was provided, collect drives that are on that node
	res := make([]drivecrd.Drive, 0)
	for _, d := range dList.Items {
		if d.Spec.NodeId == node[0] {
			res = append(res, d)
		}
	}
	return res
}

// GetDriveCRByUUID reads drive CRs and returns drive CR with uuid dUUID
func (cs *CRHelper) GetDriveCRByUUID(dUUID string) *drivecrd.Drive {
	for _, d := range cs.GetDriveCRs() {
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
