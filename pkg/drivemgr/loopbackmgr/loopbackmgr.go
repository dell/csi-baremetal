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

// Package loopbackmgr contains DriveManager for test purposes based on loop devices
package loopbackmgr

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v2"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/fs"
	"github.com/dell/csi-baremetal/pkg/base/util"
)

const (
	defaultNumberOfDevices = 3
	defaultVID             = "Test"
	defaultPID             = "Loopback"
	// 1Mi is for the metadata
	defaultSize      = "3Gi"
	defaultHealth    = apiV1.HealthGood
	defaultDriveType = apiV1.DriveTypeHDD

	threshold         = "1Gi"
	defaultFileName   = "loopback"
	rootPath          = "/"
	imagesFolder      = "/host/home"
	createFileCmdTmpl = "dd if=/dev/zero of=%s bs=1M count=%d"
	deleteFileCmdTmpl = "rm -rf %s"
	// requires root privileges
	losetupCmd                      = "losetup"
	readLoopBackDevicesMappingCmd   = losetupCmd + " -O NAME,BACK-FILE"
	setupLoopBackDeviceCmdTmpl      = losetupCmd + " -fP --show %s"
	detachLoopBackDeviceCmdTmpl     = losetupCmd + " -d %s"
	findUnusedLoopBackDeviceCmdTmpl = losetupCmd + " -f"

	configPath = "/etc/config/config.yaml"
)

/*
LoopBackManager is created for testing purposes only!
It allows to deploy CSI driver on your laptop with minikube or kind.
Developer can simulate different number of drives, their type (HDD, SSD, NVMe, etc.), health, size,
topology (accessibility), etc.
*/
type LoopBackManager struct {
	log      *logrus.Entry
	exec     command.CmdExecutor
	nodeID   string
	nodeName string
	devices  []*LoopBackDevice
	config   *Config
	sync.Mutex
}

// LoopBackDevice struct contains fields to describe a loop device bound with a file
type LoopBackDevice struct {
	VendorID     string `yaml:"vid"`
	ProductID    string `yaml:"pid"`
	SerialNumber string `yaml:"serialNumber"`
	Size         string `yaml:"size"`
	Removed      bool   `yaml:"removed"`
	Health       string `yaml:"health"`
	DriveType    string `yaml:"driveType"`
	LED          int    `yaml:"led"`

	fileName string
	// for example, /dev/loop0
	devicePath string
}

// Node struct represents particular configuration of LoopBackManager for specified node
type Node struct {
	NodeID     string            `yaml:"nodeID"`
	DriveCount int               `yaml:"driveCount"`
	Drives     []*LoopBackDevice `yaml:"drives"`
}

// Config struct is the configuration for LoopBackManager. It contains default settings and settings for each node
type Config struct {
	DefaultDriveCount int     `yaml:"defaultDrivePerNodeCount"`
	DefaultDriveSize  string  `yaml:"defaultDriveSize"`
	Nodes             []*Node `yaml:"nodes"`
}

// NewLoopBackManager is the constructor for LoopBackManager
// Receives CmdExecutor to execute os commands such as 'losetup' and logrus logger
// Returns an instance of LoopBackManager
func NewLoopBackManager(exec command.CmdExecutor, nodeID, nodeName string, logger *logrus.Logger) *LoopBackManager {
	if nodeID == "" {
		nodeID = defaultFileName
	}

	mgr := &LoopBackManager{
		log:      logger.WithField("component", "LoopBackManager"),
		exec:     exec,
		nodeID:   nodeID,
		nodeName: nodeName,
		devices:  make([]*LoopBackDevice, 0),
	}

	mgr.attemptToRecoverDevices(imagesFolder)

	return mgr
}

// attemptToRecoverDevices checks images folder for LoopBackManager's files and recovers devices from it.
// The main purpose of method to make LoopBackManager support node rebooting and prevent the creation of new files over
// existing ones
func (mgr *LoopBackManager) attemptToRecoverDevices(imagesPath string) {
	ll := mgr.log.WithField("method", "attemptToRecoveryDevices")
	mgr.Lock()
	defer mgr.Unlock()

	entries, err := ioutil.ReadDir(imagesPath)
	if err != nil {
		ll.Errorf("failed to read images folder: %v", err)
		return
	}

	// If images path doesn't contain files then it's nothing to recover. Return
	if len(entries) == 0 {
		return
	}

	mgr.readAndSetConfig(configPath)

	// Search for node config for this LoopbackMgr
	var nodeConfig *Node
	if mgr.config != nil {
		for _, node := range mgr.config.Nodes {
			if strings.EqualFold(node.NodeID, mgr.nodeName) {
				nodeConfig = node
				break
			}
		}
	}
	for _, file := range entries {
		// If image path contains files that don't correspond to this pod then delete them.
		// It's done because *.img files store on node side. If drivemgr will delete them during SIGTERM (postStop, pod
		// delete, etc) then LoopbackMgr can't support node rebooting. So it's needed to cleanup excess files on startup
		if !strings.Contains(file.Name(), mgr.nodeID) {
			absFilePath := fmt.Sprintf("%s/%s", imagesPath, file.Name())
			if _, _, err := mgr.exec.RunCmd(fmt.Sprintf(deleteFileCmdTmpl, absFilePath)); err != nil {
				ll.Errorf("Failed to cleanup excess file %s from image directory: %v", file.Name(), err)
			}
			continue
		}
		// image file looks like "pod-name-serialNumber.img". Split by '-' to extract serial number
		splitName := strings.Split(file.Name(), "-")
		// truncate ".img" from serial number
		serialNumber := strings.Trim(splitName[len(splitName)-1], ".img")
		addedFromConfig := false
		if nodeConfig != nil {
			for _, configDevice := range nodeConfig.Drives {
				// If node config contains device that matches serial number from image then recover device from config
				if strings.Contains(configDevice.SerialNumber, serialNumber) {
					ll.Infof("restore concrete device %s from config", serialNumber)
					addedFromConfig = true
					configDevice.fileName = fmt.Sprintf("%s/%s", imagesPath, file.Name())
					configDevice.fillEmptyFieldsWithDefaults()
					mgr.devices = append(mgr.devices, configDevice)
				}
			}
		}
		// If there is image file that doesn't match to device from config then recover device for it based on defaults
		if !addedFromConfig {
			ll.Infof("restore device %s from default settings", serialNumber)
			device := &LoopBackDevice{
				SerialNumber: fmt.Sprintf("LOOPBACK%s", serialNumber),
				fileName:     fmt.Sprintf("%s/%s", imagesPath, file.Name()),
			}
			device.fillEmptyFieldsWithDefaults()
			mgr.devices = append(mgr.devices, device)
		}
	}
}

// Equals checks if device is equal to provided one. Equals doesn't compare fileName and devicePath fields and compares
// only drive specification of devices.
// Receives *LoopBackDevice to check equality
func (d *LoopBackDevice) Equals(device *LoopBackDevice) bool {
	return d.Removed == device.Removed && d.DriveType == device.DriveType &&
		d.Health == device.Health && d.Size == device.Size &&
		d.SerialNumber == device.SerialNumber && d.ProductID == device.ProductID &&
		d.VendorID == device.VendorID
}

// fillEmptyFieldsWithDefaults fills fields of LoopBackDevice which are not provided in configuration with defaults
func (d *LoopBackDevice) fillEmptyFieldsWithDefaults() {
	if d.Health == "" {
		d.Health = defaultHealth // apiV1.HealthGood
	}
	if d.VendorID == "" {
		d.VendorID = defaultVID
	}
	if d.ProductID == "" {
		d.ProductID = defaultPID
	}
	if d.DriveType == "" {
		d.DriveType = defaultDriveType // apiV1.DriveTypeHDD
	}
	if d.Size == "" {
		d.Size = defaultSize
	}
}

// readAndSetConfig reads config from path and tries to unmarshall it. If unmarshall performs successfully then
// the methods sets the config to LoopBackManager
func (mgr *LoopBackManager) readAndSetConfig(path string) {
	ll := mgr.log.WithField("method", "readAndSetConfig")

	configData, err := ioutil.ReadFile(path)
	if err != nil {
		ll.Debugf("failed to read config file %s: %v", configData, err)
	} else {
		c := &Config{}
		err = yaml.Unmarshal(configData, c)
		if err != nil {
			ll.Errorf("failed to unmarshall config: %v", err)
		} else {
			mgr.config = c
		}
	}
}

// updateDevicesFromConfig reads a config with readAndSetConfig method and updates LoopBackManager's slice of devices
// according to this config. If manager's devices weren't initialized and config is empty then the method initializes
// them with local default settings.
func (mgr *LoopBackManager) updateDevicesFromConfig() {
	ll := mgr.log.WithField("method", "updateDevicesFromConfig")
	mgr.Lock()
	defer mgr.Unlock()

	mgr.readAndSetConfig(configPath)

	if mgr.config == nil {
		// If config.yaml wasn't created then initialize devices with default config
		if len(mgr.devices) < defaultNumberOfDevices {
			mgr.createDefaultDevices(defaultNumberOfDevices - len(mgr.devices))
			for _, device := range mgr.devices {
				ll.Infof("device: %v", device)
			}
		}
		return
	}

	// If config was read from the configPath file and unmarshalled
	driveCount := mgr.config.DefaultDriveCount
	var drives []*LoopBackDevice
	for _, node := range mgr.config.Nodes {
		// If config contains node config for LoopBackManager's node then use values from it
		if node.NodeID == mgr.nodeName {
			// Node's config may not contain driveCount field. Then use mgr.config.DefaultDriveCount
			if node.DriveCount > 0 {
				driveCount = node.DriveCount
			}
			drives = node.Drives
		}
	}
	// If mgr.devices is empty when fill it with (default devices - specified devices). Specified drives will be
	// appended to the end of mgr.devices later
	if len(mgr.devices) == 0 {
		mgr.createDefaultDevices(driveCount - len(drives))
		for _, device := range mgr.devices {
			ll.Infof("device: %v", device)
		}
	}
	// If specified drives for this manager are set then override or append them
	if drives != nil {
		mgr.overrideDevicesFromNodeConfig(driveCount, drives)
	}
	// If default size from config was changed, then we change size of the drive, which are not overrode on config
	for _, device := range mgr.devices {
		var found bool
		for _, drive := range drives {
			if device.SerialNumber == drive.SerialNumber {
				found = true
				break
			}
		}
		if !found {
			if mgr.config != nil && mgr.config.DefaultDriveSize != "" &&
				device.Size != mgr.config.DefaultDriveSize && device.devicePath != "" {
				ll.Infof("Size of device changes from %s to %s", device.Size, mgr.config.DefaultDriveSize)
				mgr.deleteLoopbackDevice(device)
				device.Size = mgr.config.DefaultDriveSize
				device.devicePath = ""
			}
		}
	}
	// If driveCount for specified node was increased but drives are not specified then add default devices
	mgr.createDefaultDevices(driveCount - len(mgr.devices))
}

// overrideDevicesFromNodeConfig overrides existing devices with provided as a parameter. If manager already has the
// device with same serialNumber then override its fields with read from config and fill empty fields with defaults.
// If manager doesn't have the device then append it to manager if there is free space for it.
// Receives deviceCount which represents what amount of devices manager should have and slice of devices to override
func (mgr *LoopBackManager) overrideDevicesFromNodeConfig(deviceCount int, devices []*LoopBackDevice) {
	ll := mgr.log.WithField("method", "overrideDevicesFromNodeConfig")
	for _, device := range devices {
		overrode := false
		for i, mgrDevice := range mgr.devices {
			// If manager contains device with serialNumber which is equal to provided then override it
			if strings.EqualFold(mgrDevice.SerialNumber, device.SerialNumber) {
				overrode = true
				// If drive specification of mgr device is not equal to provided (vid, pid, size...) then override it
				if !mgrDevice.Equals(device) {
					ll.Infof("Found device with serial number %s in manager. Override it", mgrDevice.SerialNumber)
					// If mgr device is already bound to loop device then check if provided configuration changes size.
					// If not then we may not rebind device and save existing devicePath
					if mgrDevice.devicePath != "" {
						switch {
						// Drive expand is not supported
						case device.Size != mgrDevice.Size || device.Size != "":
							ll.Warningf("Size of device changed or not set. Resizing not supported.")
							device.Size = mgrDevice.Size
						default:
							device.devicePath = mgrDevice.devicePath
						}
						device.fileName = mgrDevice.fileName
					}
					device.fillEmptyFieldsWithDefaults()
					ll.Infof("override existing device %s with device: %v", device.SerialNumber, device)
					mgr.devices[i] = device
				}
			}
		}
		// If provided device wasn't overrode it means that it is the new device. Try to append it
		if !overrode {
			// If mgr already has deviceCount of devices and user tries to provide new one in config then don't append it
			// and print warning because it is not known which device should be deleted
			if len(mgr.devices) >= deviceCount {
				ll.Warnf("There is no space for devices that doesn't match to existing ones. Increase driveCount")
			} else {
				// If serial number of device is not provided then generate it
				if device.SerialNumber == "" {
					deviceID := uuid.New().ID()
					device.SerialNumber = fmt.Sprintf("LOOPBACK%d", deviceID)
					device.fileName = fmt.Sprintf(imagesFolder+"/%s-%d.img", mgr.nodeID, deviceID)
				} else {
					device.fileName = fmt.Sprintf(imagesFolder+"/%s-%s.img", mgr.nodeID, device.SerialNumber)
				}
				// If device Size is not specified then use default size from config
				if mgr.config != nil && device.Size == "" && mgr.config.DefaultDriveSize != "" {
					device.Size = mgr.config.DefaultDriveSize
				}
				device.fillEmptyFieldsWithDefaults()
				ll.Infof("append non-default device: %v", device)
				mgr.devices = append(mgr.devices, device)
			}
		}
	}
}

// createDefaultDevices initialized LoopBackManager's devices with default devices
// Receives deviceCount that represents amount of devices to create
func (mgr *LoopBackManager) createDefaultDevices(deviceCount int) {
	for i := 0; i < deviceCount; i++ {
		deviceID := uuid.New().ID()
		device := &LoopBackDevice{
			SerialNumber: fmt.Sprintf("LOOPBACK%d", deviceID),
			fileName:     fmt.Sprintf(imagesFolder+"/%s-%d.img", mgr.nodeID, deviceID),
		}
		// If device Size is not specified then use default size from config
		if device.Size == "" && mgr.config != nil && mgr.config.DefaultDriveSize != "" {
			device.Size = mgr.config.DefaultDriveSize
		}
		device.fillEmptyFieldsWithDefaults()
		mgr.devices = append(mgr.devices, device)
	}
}

// deleteLoopbackDevice detach specified loopback device and delete according file
func (mgr *LoopBackManager) deleteLoopbackDevice(device *LoopBackDevice) {
	ll := mgr.log.WithField("method", "deleteLoopbackDevice")
	_, _, err := mgr.exec.RunCmd(fmt.Sprintf(detachLoopBackDeviceCmdTmpl, device.devicePath))
	if err != nil {
		ll.Errorf("Unable to detach loopback device %s", device.devicePath)
	}
	_, _, err = mgr.exec.RunCmd(fmt.Sprintf(deleteFileCmdTmpl, device.fileName))
	if err != nil {
		ll.Errorf("Unable to delete file %s", device.fileName)
	}
}

// Init creates files and register them as loopback devices
// Returns error if something went wrong
func (mgr *LoopBackManager) Init() {
	ll := mgr.log.WithField("method", "Init")
	mgr.Lock()
	defer mgr.Unlock()
	fsOps := fs.NewFSImpl(mgr.exec)
	// go through the list of devices and register if needed
	for i := 0; i < len(mgr.devices); i++ {
		// If device has devicePath it means that it already bounded to loop device. Skip it.
		if mgr.devices[i].devicePath != "" {
			continue
		}
		// wil create files in home dir. we might need to store them on host to test FI
		file := mgr.devices[i].fileName
		sizeBytes, err := util.StrToBytes(mgr.devices[i].Size)
		if err != nil {
			ll.Errorf("Failed to convert device size to bytes. Continue for next device")
			continue
		}
		sizeMb, _ := util.ToSizeUnit(sizeBytes, util.BYTE, util.MBYTE)
		// skip creation if file exists (manager restarted)
		if _, err := os.Stat(file); err != nil {
			freeBytes, err := fsOps.GetFSSpace(rootPath)
			if err != nil {
				ll.Fatal("Failed to check root fs space")
			}
			bytes, err := util.StrToBytes(threshold)
			if err != nil {
				ll.Fatalf("Parsing threshold %s failed", threshold)
			}
			if freeBytes < bytes {
				ll.Fatal("Not enough space on root fs")
			}
			_, stderr, errcode := mgr.exec.RunCmd(fmt.Sprintf(createFileCmdTmpl, file, sizeMb))
			if errcode != nil {
				ll.Fatalf("Unable to create file %s with size %d MB: %s", file, sizeMb, stderr)
			}
		}
	}

	loopDeviceMapping, err := mgr.GetBackFileToLoopMap()
	if err != nil {
		ll.Fatal(err.Error())
	}

	for i := 0; i < len(mgr.devices); i++ {
		// check that loopback device exists
		fileName := mgr.devices[i].fileName
		var found bool
		for file, loopDevs := range loopDeviceMapping {
			if strings.Contains(fileName, file) && len(loopDevs) > 0 {
				mgr.devices[i].devicePath = loopDevs[0]
				found = true
				break
			}
		}
		if !found {
			// check that system has unused device for troubleshooting purposes
			_, _, err = mgr.exec.RunCmd(findUnusedLoopBackDeviceCmdTmpl)
			if err != nil {
				ll.Error("System doesn't have unused loopback devices")
			}

			// create new device
			stdout, stderr, errcode := mgr.exec.RunCmd(fmt.Sprintf(setupLoopBackDeviceCmdTmpl, fileName))
			if errcode != nil {
				ll.Fatalf("Unable to create loopback device for %s: %s", fileName, stderr)
			}
			mgr.devices[i].devicePath = strings.TrimSuffix(stdout, "\n")
			mgr.devices[i].Removed = false
		}
	}
}

// GetDrivesList returns list of loopback devices as *api.Drive slice
// Returns *api.Drive slice or error if something went wrong
func (mgr *LoopBackManager) GetDrivesList() ([]*api.Drive, error) {
	mgr.Lock()
	defer mgr.Unlock()
	drives := make([]*api.Drive, 0, len(mgr.devices))
	for i := 0; i < len(mgr.devices); i++ {
		var driveStatus string
		if mgr.devices[i].Removed {
			driveStatus = apiV1.DriveStatusOffline
		} else {
			driveStatus = apiV1.DriveStatusOnline
		}
		sizeBytes, _ := util.StrToBytes(mgr.devices[i].Size)
		drive := &api.Drive{
			VID:          mgr.devices[i].VendorID,
			PID:          mgr.devices[i].ProductID,
			SerialNumber: mgr.devices[i].SerialNumber,
			Health:       strings.ToUpper(mgr.devices[i].Health),
			Type:         strings.ToUpper(mgr.devices[i].DriveType),
			Size:         sizeBytes,
			Status:       driveStatus,
			Path:         mgr.devices[i].devicePath,
		}
		drives = append(drives, drive)
	}
	return drives, nil
}

// Locate implements Locate method of DriveManager interface
func (mgr *LoopBackManager) Locate(serialNumber string, action int32) (int32, error) {
	for i, device := range mgr.devices {
		if device.SerialNumber == serialNumber {
			switch action {
			case apiV1.LocateStart:
				mgr.devices[i].LED = int(apiV1.LocateStatusOn)
				return apiV1.LocateStatusOn, nil
			case apiV1.LocateStop:
				mgr.devices[i].LED = int(apiV1.LocateStatusOff)
				return apiV1.LocateStatusOff, nil
			case apiV1.LocateStatus:
				return int32(mgr.devices[i].LED), nil
			}
		}
	}
	return -1, status.Error(codes.InvalidArgument, "Wrong arguments for Locate methods")
}

// LocateNode implements LocateNode method of DriveManager interface
func (mgr *LoopBackManager) LocateNode(action int32) error {
	// not implemented
	return nil
}

// GetBackFileToLoopMap return mapping between backing file and loopback devices
// Multiple loopback devices can be created from on backing file.
func (mgr *LoopBackManager) GetBackFileToLoopMap() (map[string][]string, error) {
	// check that loopback device exists
	stdout, stderr, err := mgr.exec.RunCmd(readLoopBackDevicesMappingCmd)
	errMsg := "Unable to to retrieve loopback device list"
	if err != nil {
		mgr.log.Errorf("%s: %s", errMsg, stderr)
		return nil, err
	}

	result := make(map[string][]string)
	for _, dataLine := range strings.Split(stdout, "\n")[1:] {
		if len(dataLine) == 0 {
			continue
		}
		dataFields := strings.SplitN(dataLine, " ", 2)
		if len(dataFields) != 2 {
			err := fmt.Errorf("%s: unexpected data format", errMsg)
			mgr.log.Error(err.Error())
			return nil, err
		}
		loopDevice, backFile := dataFields[0], strings.TrimSpace(dataFields[1])
		result[backFile] = append(result[backFile], loopDevice)
	}

	return result, nil
}

// CleanupLoopDevices detaches loop devices that are occupied by LoopBackManager
func (mgr *LoopBackManager) CleanupLoopDevices() {
	for i, device := range mgr.devices {
		mgr.deleteLoopbackDevice(device)
		mgr.devices[i].Removed = true
	}
}

// UpdateOnConfigChange triggers update configuration and init of devices.
func (mgr *LoopBackManager) UpdateOnConfigChange(watcher *fsnotify.Watcher) {
	ll := mgr.log.WithField("method", "UpdateOnConfigChange")
	err := watcher.Add(configPath)
	if err != nil {
		ll.Fatalf("can't add config to file watcher %s", err)
	}
	mgr.updateDevicesFromConfig()
	mgr.Init()
	for {
		event, ok := <-watcher.Events
		if !ok {
			ll.Info("file watcher is closed")
			return
		}
		ll.Debugf("event %s came ", event.Op)

		switch event.Op {
		case fsnotify.Chmod:
			continue
		case fsnotify.Remove:
			err = watcher.Remove(configPath)
			if err != nil {
				ll.Debugf("can't remove config to file watcher %s", err)
			}
			err = watcher.Add(configPath)
			if err != nil {
				ll.Fatalf("can't add config to file watcher %s", err)
			}
		default:
			ll.Warnf("file event %s", event.Op)
		}

		ll.Debugf("triggering devices update on %s event", event.Op)
		mgr.updateDevicesFromConfig()
		mgr.Init()
	}
}
