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
	"partprobe -d -s /dev/sde":        EmptyOutSuccess,
	"blockdev --rereadpt -v /dev/sde": EmptyOutSuccess,
	"partprobe -d -s /dev/sdqwe": {
		Stdout: "",
		Stderr: "",
		Err:    errors.New("unable to get partition table"),
	},
	"sgdisk /dev/sda -o": EmptyOutSuccess,
	"sgdisk /dev/sdc -o": EmptyOutSuccess,
	"sgdisk -d 1 /dev/sda": {
		Stdout: "The operation has completed successfully.",
		Stderr: "",
		Err:    nil,
	},
	"sgdisk -n 1:0:0 -c 1:CSI -u 1:64be631b-62a5-11e9-a756-00505680d67f /dev/sde": {
		Stdout: `Creating new GPT entries.
Setting name!
partNum is 0
The operation has completed successfully`,
		Stderr: "",
		Err:    nil,
	},
	"sgdisk -n 1:0:0 -c 1:CSI /dev/sde": {
		Stdout: `Creating new GPT entries.
Setting name!
partNum is 0
The operation has completed successfully`,
		Stderr: "",
		Err:    nil,
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
				"serial": "hdd1"
				}, {
				"name": "/dev/sdb",
				"type": "disk",
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

// LsblkDevV2 provides output for new lsblk version
var LsblkDevV2 = `{
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

var (
	// HDDBlockDeviceName contains name of HDD device
	HDDBlockDeviceName = "/dev/sdb"
	// LsblkAllV2 provides output for new lsblk version with children
	LsblkAllV2 = `{
   		"blockdevices": [
      		{"name":"` + HDDBlockDeviceName + `", "type":"disk", "size":480103981056, "rota":false, "serial":"PHYH937100WD480K",
"wwn":"0x55cd2e415119aed8", "vendor":"ATA     ", "model":"SSDSCKKB480G8R", "rev":"DL6N", "mountpoint":null, "fstype":null, "partuuid":null,
         		"children": [            
            		{"name":"/dev/sdc", "type":"part", "size":479563087872, "rota":false, "serial":null, "wwn":"0x55cd2e415119aed8",
"vendor":null, "model":null, "rev":null, "mountpoint":null, "fstype":"LVM2_member", "partuuid":"40b8a8aa-54fc-4725-9599-91f55a309c83",
               		"children": [
                		  {"name":"/dev/mapper/root--vg-lv_root", "type":"lvm", "size":53687091200, "rota":false, "serial":null, 
"wwn":null, "vendor":null, "model":null, "rev":null, "mountpoint":null, "fstype":"ext4", "partuuid":null}                  
               		]
            		}
        	 	]
      		},
    	  	{"name":"/dev/sdc", "type":"disk", "size":8001563222016, "rota":true, "serial":"5000cca0bbcea5c7", "wwn":"0x5000cca0bbcea5c7",
"vendor":"ATA     ", "model":"HGST_HUS728T8TAL", "rev":"RT04", "mountpoint":null, "fstype":null, "partuuid":null}
   			]
		}`
)
