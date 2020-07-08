package provisioners

import (
	"fmt"

	"github.com/sirupsen/logrus"

	api "github.com/dell/csi-baremetal.git/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal.git/api/v1"
	"github.com/dell/csi-baremetal.git/pkg/base/command"
	"github.com/dell/csi-baremetal.git/pkg/base/k8s"
	"github.com/dell/csi-baremetal.git/pkg/base/linuxutils/fs"
	"github.com/dell/csi-baremetal.git/pkg/base/linuxutils/lvm"
	"github.com/dell/csi-baremetal.git/pkg/base/util"
)

// LVMProvisioner is a implementation of Provisioner interface
// Work with volumes based on Volume Groups
type LVMProvisioner struct {
	lvmOps   lvm.WrapLVM
	fsOps    fs.WrapFS
	crHelper *k8s.CRHelper
	log      *logrus.Entry
}

// NewLVMProvisioner is a constructor for LVMProvisioner
func NewLVMProvisioner(e command.CmdExecutor, k *k8s.KubeClient, log *logrus.Logger) *LVMProvisioner {
	return &LVMProvisioner{
		lvmOps:   lvm.NewLVM(e, log),
		fsOps:    fs.NewFSImpl(e),
		crHelper: k8s.NewCRHelper(k, log),
		log:      log.WithField("component", "LVMProvisioner"),
	}
}

// PrepareVolume search volume group based on vol attributes, creates Logical Volume
// and create file system on it. After that Logical Volume is ready for mount operations
func (l *LVMProvisioner) PrepareVolume(vol api.Volume) error {
	ll := l.log.WithFields(logrus.Fields{
		"method":   "PrepareVolume",
		"volumeID": vol.Id,
	})
	ll.Infof("Processing for volume %#v", vol)

	var (
		sizeStr = fmt.Sprintf("%.2fG", float64(vol.Size)/float64(util.GBYTE))
		vgName  string
		err     error
	)
	vgName, err = l.getVGName(&vol)
	if err != nil {
		return err
	}

	// create lv with name /dev/VG_NAME/vol.Id
	ll.Infof("Creating LV %s sizeof %s in VG %s", vol.Id, sizeStr, vgName)
	if err = l.lvmOps.LVCreate(vol.Id, sizeStr, vgName); err != nil {
		return fmt.Errorf("unable to create LV: %v", err)
	}

	deviceFile := fmt.Sprintf("/dev/%s/%s", vgName, vol.Id)
	ll.Debugf("Creating FS on %s", deviceFile)
	return l.fsOps.CreateFS(fs.FileSystem(vol.Type), deviceFile)
}

// ReleaseVolume search volume group based on vol attributes, remove Logical Volume
// and wipe file system on it. After that Logical Volume that had consumed by vol is completely removed
func (l *LVMProvisioner) ReleaseVolume(vol api.Volume) error {
	ll := logrus.WithFields(logrus.Fields{
		"method":   "ReleaseVolume",
		"volumeID": vol.Id,
	})
	ll.Infof("Processing for volume %v", vol)

	deviceFile, err := l.GetVolumePath(vol)
	if err != nil {
		return fmt.Errorf("unable to determine full path of the volume: %v", err)
	}

	if err := l.fsOps.WipeFS(deviceFile); err != nil {
		return fmt.Errorf("failed to wipe FS on device %s: %v", deviceFile, err)
	}

	ll.Debugf("Removing LV %s", deviceFile)
	return l.lvmOps.LVRemove(deviceFile)
}

// GetVolumePath search Volume Group name by vol attributes and construct
// full path to the volume using template: /dev/VG_NAME/LV_NAME
func (l *LVMProvisioner) GetVolumePath(vol api.Volume) (string, error) {
	ll := l.log.WithFields(logrus.Fields{
		"method":   "GetVolumePath",
		"volumeID": vol.Id,
	})
	ll.Debugf("Processing for %v", vol)

	vgName, err := l.getVGName(&vol)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("/dev/%s/%s", vgName, vol.Id), nil // /dev/VG_NAME/LV_NAME
}

func (l *LVMProvisioner) getVGName(vol *api.Volume) (string, error) {
	var vgName = vol.Location

	// Volume.Location is an LVG CR name, LVG CR name in general is same as a real VG name on node,
	// however for LVG based on system disk LVG CR name is not the same as a VG name
	// we need to read appropriate LVG CR and use LVG CR.Spec.Name as VG name
	if vol.StorageClass == apiV1.StorageClassSSDLVG {
		var err error
		vgName, err = l.crHelper.GetVGNameByLVGCRName(vol.Location)
		if err != nil {
			return "", fmt.Errorf("unable to determine VG name: %v", err)
		}
	}
	return vgName, nil
}
