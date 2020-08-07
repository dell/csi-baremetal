package mocks

import "errors"

// Err var of type error for test purposes
var Err = errors.New("error")

// EmptyOutSuccess var of type CmdOut for test purposes
var EmptyOutSuccess = CmdOut{
	Stdout: "",
	Stderr: "",
	Err:    nil,
}

// EmptyOutFail var of type CmdOut for test purposes
var EmptyOutFail = CmdOut{
	Stdout: "",
	Stderr: "",
	Err:    Err,
}

// DiskCommands is the map that contains Linux commands output
var DiskCommands = map[string]CmdOut{
	"partprobe -d -s /dev/sda": {
		Stdout: "(no output)",
		Stderr: "",
		Err:    nil,
	},
	"partprobe -d -s /dev/sdb": {
		Stdout: "/dev/sda: msdos partitions 1",
		Stderr: "",
		Err:    nil,
	},
	"partprobe -d -s /dev/sdc": {
		Stdout: "/dev/sda: msdos partitions",
		Stderr: "",
		Err:    nil,
	},
	"partprobe -d -s /dev/sdd": {
		Stdout: "",
		Stderr: "",
		Err:    errors.New("unable to check partition existence for /dev/sdd"),
	},
	"partprobe -d -s /dev/sde": EmptyOutSuccess,
	"partprobe /dev/sde":       EmptyOutSuccess,
	"partprobe /dev/sda":       EmptyOutSuccess,
	"partprobe -d -s /dev/sdqwe": {
		Stdout: "",
		Stderr: "",
		Err:    errors.New("unable to get partition table"),
	},
	"partprobe":                      EmptyOutSuccess,
	"parted -s /dev/sda mklabel gpt": EmptyOutSuccess,
	"parted -s /dev/sdd mklabel gpt": {
		Stdout: "",
		Stderr: "",
		Err:    errors.New("unable to create partition table"),
	},
	"parted -s /dev/sdc mklabel gpt":                        EmptyOutSuccess,
	"parted -s /dev/sda rm 1":                               EmptyOutSuccess,
	"parted -s /dev/sdb rm 1":                               EmptyOutFail,
	"parted -s /dev/sde mkpart --align optimal CSI 0% 100%": EmptyOutSuccess,
	"parted -s /dev/sdf mkpart --align optimal CSI 0% 100%": EmptyOutFail,
	"sgdisk /dev/sda --partition-guid=1:64be631b-62a5-11e9-a756-00505680d67f": {
		Stdout: "The operation has completed successfully.",
		Stderr: "",
		Err:    nil,
	},
	"sgdisk /dev/sdb --partition-guid=1:64be631b-62a5-11e9-a756-00505680d67f": {
		Stdout: "The operation has completed successfully.",
		Stderr: "",
		Err:    Err,
	},
	"sgdisk /dev/sda --info=1": {
		Stdout: `Partition GUID code: 0FC63DAF-8483-4772-8E79-3D69D8477DE4 (Linux filesystem)
Partition unique GUID: 64BE631B-62A5-11E9-A756-00505680D67F
First sector: 2048 (at 1024.0 KiB)
Last sector: 1953523711 (at 931.5 GiB)
Partition size: 1953521664 sectors (931.5 GiB)
Attribute flags: 0000000000000000
Partition name: 'CSI'`,
		Stderr: "",
		Err:    nil,
	},
	"sgdisk /dev/sdb --info=1": {
		Stdout: `Partition GUID code: 0FC63DAF-8483-4772-8E79-3D69D8477DE4 (Linux filesystem)
Partition: 64BE631B-62A5-11E9-A756-00505680D67F
First sector: 2048 (at 1024.0 KiB)
Last sector: 1953523711 (at 931.5 GiB)
Partition size: 1953521664 sectors (931.5 GiB)
Attribute flags: 0000000000000000
Partition name: 'CSI'`,
		Stderr: "",
		Err:    nil,
	},
	"sgdisk /dev/sdc --info=1": EmptyOutFail,
}

// NoLsblkKeyStr imitates lsblk output without normal key
var NoLsblkKeyStr = `{"anotherKey": [{"name": "/dev/sda", "type": "disk"}]}`

// LsblkTwoDevicesStr imitates lsblk output with two block devices
var LsblkTwoDevicesStr = `{
			  "blockdevices":[{
				"name": "/dev/sda",
				"type": "disk",
				"serial": "hdd1",
				"size": 1024
				}, {
				"name": "/dev/sdb",
				"type": "disk",
				"serial": "hdd2",
				"size": 1024
				}]
			}`

// LsblkListPartitionsStr imitates lsblk output with block device that has partition
var LsblkListPartitionsStr = `{
			  "blockdevices":[{
				"name": "/dev/sda",
				"type": "disk",
				"serial": "hdd1",
				"children": [{"name": "/dev/sda1", "mountpoint":"", "partuuid":"volume-1-id"}]
				}]
			}`

// LsblkDevWithChildren imitates lsblk output with two block devices with children
var LsblkDevWithChildren = CmdOut{
	Stdout: `{
			  "blockdevices":[{
				"name": "/dev/sdb",
				"type": "disk",
				"serial": "hdd2",
				"children": [{"name": "/dev/children1", "mountpoint":""}, 
							 {"name": "/dev/children2", "mountpoint":"/var/lib/kubelet/pods/27cc6e45-61f1-11e9-b966-001e67e6854b/volumes/kubernetes.io~csi/pvc-27cbea1b-61f1-11e9-b966-001e67e6854b/mount"}],
				"size": 213674622976
				}]
			}`,
	Stderr: "",
	Err:    nil}
