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
				"size": "8001563222016",
				"rota": "1",
				"serial": "hdd1"
				}, {
				"name": "/dev/sdb",
				"type": "disk",
				"size": "4001563222016",
				"rota": "0",
				"serial": "hdd2"
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
				"size": "213674622976"
				}]
			}`,
	Stderr: "",
	Err:    nil}

var LsblkDevNewVersion = `{
			"blockdevices": [{
				"name":"/dev/sdc",
				"type":"disk",
				"size":8001563222016,
				"rota":true,
				"serial":"5000cca0bbce17ff",
				"wwn":"0x5000cca0bbce17ff",
				"vendor":"ATA     ",
				"model":"HGST_HUS728T8TAL",
				"rev":"RT04",
				"mountpoint":null,
				"fstype":null,
				"partuuid":null
			}]}`

var LsblkAllNewVersion = `{
   "blockdevices": [
      {"name":"/dev/loop0", "type":"loop", "size":28405760, "rota":false, "serial":null, "wwn":null, "vendor":null, "model":null, "rev":null, "mountpoint":null, "fstype":"squashfs", "partuuid":null},
      {"name":"/dev/loop1", "type":"loop", "size":57614336, "rota":false, "serial":null, "wwn":null, "vendor":null, "model":null, "rev":null, "mountpoint":null, "fstype":"squashfs", "partuuid":null},
      {"name":"/dev/loop2", "type":"loop", "size":72318976, "rota":false, "serial":null, "wwn":null, "vendor":null, "model":null, "rev":null, "mountpoint":null, "fstype":"squashfs", "partuuid":null},
      {"name":"/dev/sdb", "type":"disk", "size":480103981056, "rota":false, "serial":"PHYH937100WD480K", "wwn":"0x55cd2e415119aed8", "vendor":"ATA     ", "model":"SSDSCKKB480G8R", "rev":"DL6N", "mountpoint":null, "fstype":null, "partuuid":null,
         "children": [
            {"name":"/dev/sdb1", "type":"part", "size":1048576, "rota":false, "serial":null, "wwn":"0x55cd2e415119aed8", "vendor":null, "model":null, "rev":null, "mountpoint":null, "fstype":null, "partuuid":"10d6830b-c966-4ead-a6d7-98ccbf6e1a37"},
            {"name":"/dev/sdb2", "type":"part", "size":536870912, "rota":false, "serial":null, "wwn":"0x55cd2e415119aed8", "vendor":null, "model":null, "rev":null, "mountpoint":null, "fstype":"ext4", "partuuid":"b487c6f7-4a1f-46f6-9773-bc64dac2b73f"},
            {"name":"/dev/sdb3", "type":"part", "size":479563087872, "rota":false, "serial":null, "wwn":"0x55cd2e415119aed8", "vendor":null, "model":null, "rev":null, "mountpoint":null, "fstype":"LVM2_member", "partuuid":"40b8a8aa-54fc-4725-9599-91f55a309c83",
               "children": [
                  {"name":"/dev/mapper/root--vg-lv_root", "type":"lvm", "size":53687091200, "rota":false, "serial":null, "wwn":null, "vendor":null, "model":null, "rev":null, "mountpoint":null, "fstype":"ext4", "partuuid":null},
                  {"name":"/dev/mapper/root--vg-lv_var", "type":"lvm", "size":107374182400, "rota":false, "serial":null, "wwn":null, "vendor":null, "model":null, "rev":null, "mountpoint":"/var/lib/kubelet/pods", "fstype":"ext4", "partuuid":null}
               ]
            }
         ]
      },
      {"name":"/dev/sdc", "type":"disk", "size":8001563222016, "rota":true, "serial":"5000cca0bbcea5c7", "wwn":"0x5000cca0bbcea5c7", "vendor":"ATA     ", "model":"HGST_HUS728T8TAL", "rev":"RT04", "mountpoint":null, "fstype":null, "partuuid":null},
      {"name":"/dev/sdd", "type":"disk", "size":8001563222016, "rota":true, "serial":"5000cca0bbce8d3d", "wwn":"0x5000cca0bbce8d3d", "vendor":"ATA     ", "model":"HGST_HUS728T8TAL", "rev":"RT04", "mountpoint":null, "fstype":null, "partuuid":null},
      {"name":"/dev/sde", "type":"disk", "size":8001563222016, "rota":true, "serial":"5000cca0bbcea1da", "wwn":"0x5000cca0bbcea1da", "vendor":"ATA     ", "model":"HGST_HUS728T8TAL", "rev":"RT04", "mountpoint":null, "fstype":null, "partuuid":null},
      {"name":"/dev/sdf", "type":"disk", "size":8001563222016, "rota":true, "serial":"5000cca0bbce7ff6", "wwn":"0x5000cca0bbce7ff6", "vendor":"ATA     ", "model":"HGST_HUS728T8TAL", "rev":"RT04", "mountpoint":null, "fstype":null, "partuuid":null},
      {"name":"/dev/sdg", "type":"disk", "size":8001563222016, "rota":true, "serial":"5000cca0bbce9d5f", "wwn":"0x5000cca0bbce9d5f", "vendor":"ATA     ", "model":"HGST_HUS728T8TAL", "rev":"RT04", "mountpoint":null, "fstype":null, "partuuid":null},
      {"name":"/dev/sdh", "type":"disk", "size":8001563222016, "rota":true, "serial":"5000cca0bbcea4d3", "wwn":"0x5000cca0bbcea4d3", "vendor":"ATA     ", "model":"HGST_HUS728T8TAL", "rev":"RT04", "mountpoint":null, "fstype":null, "partuuid":null},
      {"name":"/dev/sdi", "type":"disk", "size":8001563222016, "rota":true, "serial":"5000cca0bbce87b0", "wwn":"0x5000cca0bbce87b0", "vendor":"ATA     ", "model":"HGST_HUS728T8TAL", "rev":"RT04", "mountpoint":null, "fstype":null, "partuuid":null},
      {"name":"/dev/sdj", "type":"disk", "size":8001563222016, "rota":true, "serial":"5000cca0bbceb10f", "wwn":"0x5000cca0bbceb10f", "vendor":"ATA     ", "model":"HGST_HUS728T8TAL", "rev":"RT04", "mountpoint":null, "fstype":null, "partuuid":null},
      {"name":"/dev/sdk", "type":"disk", "size":8001563222016, "rota":true, "serial":"5000cca0bbce7a1d", "wwn":"0x5000cca0bbce7a1d", "vendor":"ATA     ", "model":"HGST_HUS728T8TAL", "rev":"RT04", "mountpoint":null, "fstype":null, "partuuid":null},
      {"name":"/dev/sdl", "type":"disk", "size":8001563222016, "rota":true, "serial":"5000cca0bbcea56f", "wwn":"0x5000cca0bbcea56f", "vendor":"ATA     ", "model":"HGST_HUS728T8TAL", "rev":"RT04", "mountpoint":null, "fstype":null, "partuuid":null},
      {"name":"/dev/sdm", "type":"disk", "size":8001563222016, "rota":true, "serial":"5000cca0bbcea4c3", "wwn":"0x5000cca0bbcea4c3", "vendor":"ATA     ", "model":"HGST_HUS728T8TAL", "rev":"RT04", "mountpoint":null, "fstype":null, "partuuid":null},
      {"name":"/dev/sdn", "type":"disk", "size":8001563222016, "rota":true, "serial":"5000cca0bbceb2b4", "wwn":"0x5000cca0bbceb2b4", "vendor":"ATA     ", "model":"HGST_HUS728T8TAL", "rev":"RT04", "mountpoint":null, "fstype":null, "partuuid":null},
      {"name":"/dev/sdo", "type":"disk", "size":8001563222016, "rota":true, "serial":"5000cca0bbccc75b", "wwn":"0x5000cca0bbccc75b", "vendor":"ATA     ", "model":"HGST_HUS728T8TAL", "rev":"RT04", "mountpoint":null, "fstype":null, "partuuid":null},
      {"name":"/dev/sdp", "type":"disk", "size":8001563222016, "rota":true, "serial":"5000cca0bbcea059", "wwn":"0x5000cca0bbcea059", "vendor":"ATA     ", "model":"HGST_HUS728T8TAL", "rev":"RT04", "mountpoint":null, "fstype":null, "partuuid":null},
      {"name":"/dev/sdq", "type":"disk", "size":8001563222016, "rota":true, "serial":"5000cca0bbcea5a4", "wwn":"0x5000cca0bbcea5a4", "vendor":"ATA     ", "model":"HGST_HUS728T8TAL", "rev":"RT04", "mountpoint":null, "fstype":null, "partuuid":null},
      {"name":"/dev/sdr", "type":"disk", "size":8001563222016, "rota":true, "serial":"5000cca0bbce9cb8", "wwn":"0x5000cca0bbce9cb8", "vendor":"ATA     ", "model":"HGST_HUS728T8TAL", "rev":"RT04", "mountpoint":null, "fstype":null, "partuuid":null},
      {"name":"/dev/sds", "type":"disk", "size":8001563222016, "rota":true, "serial":"5000cca0bbcc1ead", "wwn":"0x5000cca0bbcc1ead", "vendor":"ATA     ", "model":"HGST_HUS728T8TAL", "rev":"RT04", "mountpoint":null, "fstype":null, "partuuid":null},
      {"name":"/dev/sdt", "type":"disk", "size":8001563222016, "rota":true, "serial":"5000cca0bbcc46b7", "wwn":"0x5000cca0bbcc46b7", "vendor":"ATA     ", "model":"HGST_HUS728T8TAL", "rev":"RT04", "mountpoint":null, "fstype":null, "partuuid":null},
      {"name":"/dev/sdu", "type":"disk", "size":8001563222016, "rota":true, "serial":"5000cca0bbceb0d7", "wwn":"0x5000cca0bbceb0d7", "vendor":"ATA     ", "model":"HGST_HUS728T8TAL", "rev":"RT04", "mountpoint":null, "fstype":null, "partuuid":null},
      {"name":"/dev/sdv", "type":"disk", "size":8001563222016, "rota":true, "serial":"5000cca0bbceae2f", "wwn":"0x5000cca0bbceae2f", "vendor":"ATA     ", "model":"HGST_HUS728T8TAL", "rev":"RT04", "mountpoint":null, "fstype":null, "partuuid":null},
      {"name":"/dev/sdw", "type":"disk", "size":8001563222016, "rota":true, "serial":"5000cca0bbceb603", "wwn":"0x5000cca0bbceb603", "vendor":"ATA     ", "model":"HGST_HUS728T8TAL", "rev":"RT04", "mountpoint":null, "fstype":null, "partuuid":null},
      {"name":"/dev/sdx", "type":"disk", "size":8001563222016, "rota":true, "serial":"5000cca0bbcea6d2", "wwn":"0x5000cca0bbcea6d2", "vendor":"ATA     ", "model":"HGST_HUS728T8TAL", "rev":"RT04", "mountpoint":null, "fstype":null, "partuuid":null},
      {"name":"/dev/sdy", "type":"disk", "size":8001563222016, "rota":true, "serial":"5000cca0bbce9c6f", "wwn":"0x5000cca0bbce9c6f", "vendor":"ATA     ", "model":"HGST_HUS728T8TAL", "rev":"RT04", "mountpoint":null, "fstype":null, "partuuid":null},
      {"name":"/dev/sdz", "type":"disk", "size":8001563222016, "rota":true, "serial":"5000cca0bbcb7e6a", "wwn":"0x5000cca0bbcb7e6a", "vendor":"ATA     ", "model":"HGST_HUS728T8TAL", "rev":"RT04", "mountpoint":null, "fstype":null, "partuuid":null}
   ]
}`
