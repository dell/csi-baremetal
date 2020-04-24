// Package loopbackmgr contains HWManager for test purposes based on loop devices
package loopbackmgr

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/sirupsen/logrus"

	api "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/generated/v1"
	apiV1 "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/api/v1"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base"
)

const (
	// todo AK8S-635 number of devices and other settings must be set via config map in runtime
	numberOfDevices   = 3
	threshold         = "1Gi"
	defaultFileName   = "loopback"
	tmpFolder         = "/tmp"
	createFileCmdTmpl = "dd if=/dev/zero of=%s bs=1M count=%d"
	// requires root privileges
	losetupCmd                      = "losetup"
	checkLoopBackDeviceCmdTmpl      = losetupCmd + " -j %s"
	setupLoopBackDeviceCmdTmpl      = losetupCmd + " -fP %s"
	detachLoopBackDeviceCmdTmpl     = losetupCmd + " -d %s"
	findUnusedLoopBackDeviceCmdTmpl = losetupCmd + " -f"
)

/*
LoopBackManager is created for testing purposes only!
It allows to deploy CSI driver on your laptop with minikube or kind.
Developer can simulate different number of drives, their type (HDD, SSD, NVMe, etc.), health, size,
topology (accessibility), etc.
*/
type LoopBackManager struct {
	log      *logrus.Entry
	exec     base.CmdExecutor
	hostname string
	devices  [numberOfDevices]LoopBackDevice
}

// LoopBackDevice struct contains fields to describe a loop device bound with a file
type LoopBackDevice struct {
	fileName     string
	vendorID     string
	productID    string
	serialNumber string
	// need to have unit64
	sizeMb int64
	// for example, /dev/loop0
	devicePath string
}

// NewLoopBackManager is the constructor for LoopBackManager
// Receives CmdExecutor to execute os commands such as 'losetup' and logrus logger
// Returns an instance of LoopBackManager
func NewLoopBackManager(exec base.CmdExecutor, logger *logrus.Logger) *LoopBackManager {
	var devices [numberOfDevices]LoopBackDevice

	// read hostname variable - this is pod's name.
	// since pod might restart and change name better to user real hostname
	hostname := os.Getenv("HOSTNAME")
	if hostname == "" {
		/* if not defined set to default - will not break anything but
		might not be very convinient for troubkeshooting:
		/sbin/losetup  | grep baremetal-csi-node
		/dev/loop19         0      0         0  0 /tmp/baremetal-csi-node-w787v-0.img                  0
		/dev/loop17         0      0         0  0 /tmp/baremetal-csi-node-99zcp-0.img                  0
		/dev/loop18         0      0         0  0 /tmp/baremetal-csi-node-xj8gw-0.img                  0
		/dev/loop10         0      0         0  0 /tmp/baremetal-csi-node-dfwvv-0.img                  0
		*/
		hostname = defaultFileName
	}

	for i := 0; i < numberOfDevices; i++ {
		// file names must be different for every hwmgr instance
		devices[i].fileName = fmt.Sprintf(tmpFolder+"/%s-%d.img", hostname, i)
		devices[i].vendorID = "Test"
		devices[i].productID = "Loopback"
		// todo is it ok so have same SN on different nodes?
		devices[i].serialNumber = fmt.Sprintf("LOOPBACK%d", i)
		devices[i].sizeMb = 100 //100 MB
		devices[i].devicePath = fmt.Sprintf("/dev/loop%d", i)
	}

	// are there any other ways to mock executor?
	exec.SetLogger(logger)
	return &LoopBackManager{
		log:      logger.WithField("component", "LoopBackManager"),
		exec:     exec,
		hostname: hostname,
		devices:  devices,
	}
}

// Init creates files and register them as loopback devices
// Returns error if something went wrong
func (mgr *LoopBackManager) Init() (err error) {
	// clean loop devices after hwmgr deletion
	// using defer is the bad practice because defer isn't invoking during SIGTERM or SIGINT
	// kubernetes sends SIGTERM signal to containers for pods terminating
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		mgr.CleanupLoopDevices()
		os.Exit(0)
	}()

	var device string

	rfutils := base.NewRootFsUtils(mgr.exec)
	// go through the list of devices and register if needed
	for i := 0; i < numberOfDevices; i++ {
		// wil create files in home dir. we might need to store them on host to test FI
		file := mgr.devices[i].fileName
		sizeMb := mgr.devices[i].sizeMb
		// skip creation if file exists (manager restarted)
		if _, err := os.Stat(file); err != nil {
			freeBytes, err := rfutils.CheckRootFsSpace()
			if err != nil {
				mgr.log.Fatal("Failed to check root fs space")
			}
			bytes, err := base.StrToBytes(threshold)
			if err != nil {
				mgr.log.Fatalf("Parsing threshold %s failed", threshold)
			}
			if freeBytes < bytes {
				mgr.log.Fatal("Not enough space on root fs")
			}
			_, stderr, errcode := mgr.exec.RunCmd(fmt.Sprintf(createFileCmdTmpl, file, sizeMb))
			if errcode != nil {
				mgr.log.Fatalf("Unable to create file %s with size %d MB: %s", file, sizeMb, stderr)
			}

			// check that loopback device exists. ignore error here
			device, _ = mgr.GetLoopBackDeviceName(file)
			if device != "" {
				// try to detach
				_, _, err := mgr.exec.RunCmd(fmt.Sprintf(detachLoopBackDeviceCmdTmpl, device))
				if err != nil {
					mgr.log.Errorf("Unable to detach loopback device %s for file %s", device, file)
				}
			}
		} else {
			// check that loopback device exists
			device, _ = mgr.GetLoopBackDeviceName(file)
			if device != "" {
				mgr.devices[i].devicePath = device
				// go to the next
				continue
			}
		}

		// check that system has unused device for troubleshooting purposes
		_, _, err = mgr.exec.RunCmd(findUnusedLoopBackDeviceCmdTmpl)
		if err != nil {
			mgr.log.Error("System doesn't have unused loopback devices")
		}

		// create new device
		_, stderr, errcode := mgr.exec.RunCmd(fmt.Sprintf(setupLoopBackDeviceCmdTmpl, file))
		if errcode != nil {
			mgr.log.Fatalf("Unable to create loopback device for %s: %s", file, stderr)
		}
		device, _ = mgr.GetLoopBackDeviceName(file)
		mgr.devices[i].devicePath = device
	}
	return nil
}

// GetDrivesList returns list of loopback devices as *api.Drive slice
// Returns *api.Drive slice or error if something went wrong
func (mgr *LoopBackManager) GetDrivesList() ([]*api.Drive, error) {
	drives := make([]*api.Drive, 0)
	for i := 0; i < numberOfDevices; i++ {
		drive := &api.Drive{
			VID:          mgr.devices[i].vendorID,
			PID:          mgr.devices[i].productID,
			SerialNumber: mgr.devices[i].serialNumber,
			Health:       apiV1.HealthGood,
			Type:         apiV1.DriveTypeHDD,
			Size:         mgr.devices[i].sizeMb * 1024 * 1024,
			Status:       apiV1.DriveStatusOnline,
			Path:         mgr.devices[i].devicePath,
		}
		drives = append(drives, drive)
	}
	return drives, nil
}

// GetLoopBackDeviceName checks whether device registered for file or not
// Receives file path as a string
// Returns device path or or empty string if the file is not bounded to any loop device
func (mgr *LoopBackManager) GetLoopBackDeviceName(file string) (string, error) {
	// check that loopback device exists
	stdout, stderr, err := mgr.exec.RunCmd(fmt.Sprintf(checkLoopBackDeviceCmdTmpl, file))
	if err != nil {
		mgr.log.Errorf("Unable to check loopback configuration for %s: %s", file, stderr)
		return "", err
	}

	// not the best way to find file name
	if strings.Contains(stdout, file) {
		// device already registered
		// output example: /dev/loop18: []: (/tmp/loopback-ubuntu-0.img)
		return strings.Split(stdout, ":")[0], nil
	}

	return "", nil
}

// CleanupLoopDevices detaches loop devices that are occupied by LoopBackManager
func (mgr *LoopBackManager) CleanupLoopDevices() {
	for _, device := range mgr.devices {
		_, _, err := mgr.exec.RunCmd(fmt.Sprintf(detachLoopBackDeviceCmdTmpl, device.devicePath))
		if err != nil {
			mgr.log.WithField("method", "CleanupLoopDevices").
				Errorf("Unable to detach loopback device %v", device)
		}
	}
}
