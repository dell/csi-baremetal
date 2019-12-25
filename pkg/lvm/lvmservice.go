package lvm

import (
	"fmt"
	"os/exec"
	"strings"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/util"
	"github.com/sirupsen/logrus"
)

type VolumeGroup struct {
	Name         string  `json:"name"`
	SizeInGb     float64 `json:"size_in_gb"`
	FreeSizeInGb float64 `json:"free_size_in_gb"`
	// function that used for searching block devices that should be added in volume group
	DiskFilter func() []string `json:"-"`
}

type LogicalVolume struct {
	Name   string `json:"lv_name"`
	VGName string `json:"vg_name"`
	LVSize string `json:"lv_size,omitempty"`
}

func (vg *VolumeGroup) InitVG() error {
	ll := logrus.WithField("method", "InitVG")
	// check whether VG exist
	ok, _ := IsVolumeGroupExist(vg.Name)
	if ok {
		ll.Infof("Volume group %s has already exist. Skip creation.", vg.Name)
		return nil
	}
	disks := vg.DiskFilter()
	// create physical volume from each disk with 0 partition
	pvs := make([]string, 0)
	for _, path := range disks {
		if ok, _ := IsPhysicalVolumeExist(path); ok {
			ll.Infof("PV for path %s has already exist", path)
			continue
		}
		ll.Infof("Creating PV for path %s", path)
		err := CreatePhysicalVolume(path)
		if err != nil {
			ll.Errorf("PV %s was not created: %v", path, err)
		} else {
			pvs = append(pvs, path)
		}
	}
	if len(pvs) == 0 {
		return fmt.Errorf("no one PVs were created")
	}

	ll.Infof("Create VG with next PVs: %v", pvs)
	return CreateVolumeGroup(vg.Name, pvs...)
}

func (vg *VolumeGroup) RemoveLVMStaff() bool {
	wasErr := false
	ll := logrus.WithField("method", "RemoveLVMStaff")
	ll.Info("========== Erase LVM ==========")

	ll.Info("*** LVs removing")
	lvs := make(map[string]string) // k - lv name, v - mount path
	// search logical volumes
	lvsOut, _ := util.RunCmdAndCollectOutput(exec.Command("lvdisplay", vg.Name, "--column"))
	for _, line := range strings.Split(lvsOut, "\n") {
		if strings.Contains(line, "Attr") || len(line) < 1 { // exclude header part: LV   VG     Att ...
			continue
		} else {
			lvs[strings.Fields(line)[0]] = "" // added lv name
		}
	}
	// search logical volumes that mounted in kubelet folder
	out, _ := util.RunCmdAndCollectOutput(exec.Command("mount"))
	for _, line := range strings.Split(out, "\n") {
		for lv := range lvs {
			if strings.Contains(line, lv) && strings.Contains(line, "kubelet/pods") {
				lvs[lv] = strings.Fields(line)[2]
			}
		}
	}
	ll.Infof("LV mount point mapping: %v", lvs)
	// unmount logical volumes
	for _, v := range lvs {
		if len(v) > 1 {
			err := LUnmount(v)
			if err != nil {
				ll.Errorf("Could not unmount %s, error: %v", v, err)
				wasErr = true
			}
		}
	}
	// remove logical volumes
	for lv := range lvs {
		err := RemoveLogicalVolume(lv, vg.Name)
		if err != nil {
			ll.Errorf("Could not remove LV %s. Error: %v", lv, err)
			wasErr = true
		}
	}
	ll.Info("*** LVs removed")
	ll.Info("*** VG removing")
	err := RemoveVolumeGroup(vg.Name)
	if err != nil {
		ll.Errorf("Could not remove VG. Error: %v", err)
		wasErr = true
	}
	ll.Infof("*** VG removed")
	ll.Info("*** PVs removing")
	pvs := make([]string, 0)
	out, _ = util.RunCmdAndCollectOutput(exec.Command("pvdisplay", "-s"))
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Device") {
			pv := strings.Fields(line)[1] // "/dev/DEV_NAME"
			pvs = append(pvs, pv[1:len(pv)-1])
		}
	}
	ll.Infof("Found PV: %v", pvs)
	for _, pv := range pvs {
		err := RemovePhysicalVolume(pv)
		if err != nil {
			ll.Errorf("Could not remove LV %s. Error: %v", pv, err)
			wasErr = true
		}
	}
	ll.Info("*** PVs removed")
	ll.Info("========== COMPLETE ==========")

	return wasErr
}
