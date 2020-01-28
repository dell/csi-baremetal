package mocks

import "errors"

var err = errors.New("error")

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
	"partprobe -d -s /dev/sdqwe": {
		Stdout: "",
		Stderr: "",
		Err:    errors.New("unable to get partition table"),
	},
	"parted -s /dev/sda mklabel gpt": {
		Stdout: "",
		Stderr: "",
		Err:    nil,
	},
	"parted -s /dev/sdd mklabel gpt": {
		Stdout: "",
		Stderr: "",
		Err:    errors.New("unable to create partition table"),
	},
	"parted -s /dev/sdc mklabel gpt": {
		Stdout: "",
		Stderr: "",
		Err:    nil,
	},
	"parted -s /dev/sda rm 1": {
		Stdout: "",
		Stderr: "",
		Err:    nil,
	},
	"parted -s /dev/sdb rm 1": {
		Stdout: "",
		Stderr: "",
		Err:    err,
	},
	"parted -s /dev/sde mkpart --align optimal CSI 0% 100%": {
		Stdout: "",
		Stderr: "",
		Err:    nil,
	},
	"parted -s /dev/sdf mkpart --align optimal CSI 0% 100%": {
		Stdout: "",
		Stderr: "",
		Err:    err,
	},
	"sgdisk /dev/sda -u 1:64be631b-62a5-11e9-a756-00505680d67f": {
		Stdout: "The operation has completed successfully.",
		Stderr: "",
		Err:    nil,
	},
	"sgdisk /dev/sdb -u 1:64be631b-62a5-11e9-a756-00505680d67f": {
		Stdout: "The operation has completed successfully.",
		Stderr: "",
		Err:    err,
	},
	"sgdisk /dev/sda -i 1": {
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
	"sgdisk /dev/sdb -i 1": {
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
	"sgdisk /dev/sdc -i 1": {
		Stdout: "",
		Stderr: "",
		Err:    err,
	},
}

var NoLsblkKey = CmdOut{
	Stdout: `{"anotherKey": [{"name": "/dev/sda", "type": "disk"}]}`,
	Stderr: "",
	Err:    nil,
}

var LsblkTwoDevices = CmdOut{
	Stdout: `{
			  "blockdevices":[{
				"name": "/dev/sda",
				"type": "disk",
				"serial": "hdd1"
				}, {
				"name": "/dev/sdb",
				"type": "disk",
				"serial": "hdd2"
				}]
			}`,
	Stderr: "",
	Err:    nil,
}

var LsblkDevWithChildren = CmdOut{
	Stdout: `{
			  "blockdevices":[{
				"name": "/dev/sda",
				"type": "disk",
				"serial": "hdd1",
				"children": [{"name": "/dev/children0"}]
				}, {
				"name": "/dev/sdb",
				"type": "disk",
				"serial": "hdd2",
				"children": [{"name": "/dev/children1"}, {"name": "/dev/children2"}],
				"size": "213674622976"
				}]
			}`,
	Stderr: "",
	Err:    nil,
}
