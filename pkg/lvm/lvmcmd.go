package lvm

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/util"
	"github.com/sirupsen/logrus"
)

// contain method for interacting with lvm util on host machine

const lvm = "/sbin/lvm"

func CreatePhysicalVolume(dev string) error {
	_, err := util.RunCmdAndCollectOutput(exec.Command(lvm, "pvcreate", "-ff", dev)) // TODO: remove force option
	if err != nil {
		return fmt.Errorf("failed to create physical volume %s", dev)
	}

	return nil
}

func IsPhysicalVolumeExist(dev string) (bool, error) {
	out, err := util.RunCmdAndCollectOutput(exec.Command(lvm, "pvdisplay", "-s"))
	if err != nil {
		return false, fmt.Errorf("failed to list physical volumes")
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, dev) {
			return true, nil
		}
	}

	return false, nil
}

func RemovePhysicalVolume(dev string) error {
	_, err := util.RunCmdAndCollectOutput(exec.Command(lvm, "pvremove", "--force", dev)) // TODO: remove force option
	if err != nil {
		return fmt.Errorf("failed to remove physical volume %s", dev)
	}
	return nil
}

func CreateVolumeGroup(name string, pvs ...string) error {
	args := []string{"vgcreate", "--force", name} // // TODO: remove force option
	args = append(args, pvs...)
	_, err := util.RunCmdAndCollectOutput(exec.Command(lvm, args...))
	if err != nil {
		return fmt.Errorf("failed to create volume group %s with physical volumes: %v", name, pvs)
	}
	return nil
}

func RemoveVolumeGroup(vgName string) error {
	_, err := util.RunCmdAndCollectOutput(exec.Command(lvm, "vgremove", "--force", vgName)) // TODO: remove force option
	if err != nil {
		return fmt.Errorf("failed to remove volume group %s", vgName)
	}
	return nil
}

func IsVolumeGroupExist(vgName string) (bool, error) {
	out, err := util.RunCmdAndCollectOutput(exec.Command(lvm, "vgdisplay", "-s"))
	if err != nil {
		return false, fmt.Errorf("could not list volume groups")
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, vgName) {
			return true, nil
		}
	}
	return false, nil
}

func ExtendVolumeGroup(vgName string, pvs ...string) error {
	args := []string{"vgextend", vgName}
	args = append(args, pvs...)
	_, err := util.RunCmdAndCollectOutput(exec.Command(lvm, args...))
	if err != nil {
		return fmt.Errorf("failed to extend volume group %s by adding physical volumes: %v", vgName, pvs)
	}
	return nil
}

func ReduceVolumeGroup(vgName string, pvName string) error {
	_, err := util.RunCmdAndCollectOutput(exec.Command(lvm, "vgreduce", vgName, pvName))
	if err != nil {
		return fmt.Errorf("failed to reduce volume group %s by removing %s pysical volume", vgName, pvName)
	}
	return nil
}

func VolumeGroupState(vgName string) (*VolumeGroup, error) {
	ll := logrus.WithField("method", "VolumeGroupState")
	out, err := util.RunCmdAndCollectOutput(exec.Command(lvm, "vgs", vgName, "--units", "g"))
	if err != nil {
		return nil, fmt.Errorf("failed to get info for volume group %s", vgName)
	}
	/**
	out will be like:
	VG     #PV #LV #SN Attr   VSize   VFree
	autovg   2   3   0 wz--n- 111.99g 44.59g
	**/
	splitted := strings.Split(out, "\n")
	for _, line := range splitted {
		if strings.Contains(line, vgName) {
			columns := strings.Fields(line)
			// parse VG size
			s := columns[5]
			sf, err := strconv.ParseFloat(s[:len(s)-1], 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse a size of volume group %s, error: %s", vgName, err)
			}
			// parse free size in VG
			fs := columns[6]
			fsf, err := strconv.ParseFloat(fs[:len(fs)-1], 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse a free size of volume group %s, error: %s", vgName, err)
			}
			return &VolumeGroup{
				Name:         vgName,
				SizeInGb:     sf,
				FreeSizeInGb: fsf,
			}, nil
		}
	}
	ll.Warnf("Could not find VG %s", vgName)
	return nil, nil
}

func CreateLogicalVolume(name string, size string, vg string) error {
	_, err := util.RunCmdAndCollectOutput(exec.Command(lvm, "lvcreate", "--name", name, "--size", size, vg))
	if err != nil {
		return fmt.Errorf("failed to create logical volume with name %s and size %s in volulme group %s",
			name, size, vg)
	}
	return nil
}

func RemoveLogicalVolume(lvName string, vgName string) error {
	p := fmt.Sprintf("/dev/%s/%s", vgName, lvName)
	_, err := util.RunCmdAndCollectOutput(exec.Command(lvm, "lvremove", "--force", p)) // TODO: remove force option
	if err != nil {
		return fmt.Errorf("failed to remove logical volume %s", p)
	}
	return nil
}

// return error if LV do not exist and nil in another cases
func IsLogicalVolumeExist(lvName string, vgName string) (bool, error) {
	lv := fmt.Sprintf("%s/%s", vgName, lvName)
	out, err := util.RunCmdAndCollectOutput(exec.Command(lvm, "lvdisplay", lv))
	if err != nil {
		return false, fmt.Errorf("could not list logincal volumes")
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, lvName) && strings.Contains(line, vgName) {
			return true, nil
		}
	}
	return false, nil
}

func MakeFileSystemOnLogicalVolume(lvName string, vgName string) error {
	p := fmt.Sprintf("/dev/%s/%s", vgName, lvName)
	_, err := util.RunCmdAndCollectOutput(exec.Command("mkfs.xfs", "-L", lvName, p))
	if err != nil {
		return fmt.Errorf("failed to make file system on logical volume %s", p)
	}
	return nil
}

func LMount(src string, dst string) error {
	_, err := util.RunCmdAndCollectOutput(exec.Command("mkdir", "-p", dst))
	if err != nil {
		return fmt.Errorf("failed to create destination folder %s", dst)
	}
	_, err = util.RunCmdAndCollectOutput(exec.Command("mount", src, dst))
	if err != nil {
		return fmt.Errorf("failed to mount %s to %s", src, dst)
	}
	return nil
}

// TODO: duplicated functional
func LUnmount(path string) error {
	_, err := util.RunCmdAndCollectOutput(exec.Command("umount", path))
	if err != nil {
		return fmt.Errorf("failed to umount %s", path)
	}
	return nil
}
