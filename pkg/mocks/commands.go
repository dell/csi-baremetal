package mocks

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
