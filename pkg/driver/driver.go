// Package driver implements CSI specification
package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"sync"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/util"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// ECSCSIDriver is a struct for CSI plugin
type ECSCSIDriver struct {
	name     string
	version  string
	endpoint string
	nodeID   string

	srv *grpc.Server

	ready bool
}

// TODO: generate version
const (
	version = "0.0.1"
)

// Mutex is a sync primitive to synchronize CreateVolume calls
var Mutex = &sync.Mutex{}

// NodeAllocatedDisks is a map for storing disk and allocation status
// k - hostname, v: k - disk, v -a allocated
var NodeAllocatedDisks = make(map[string]map[util.HalDisk]bool)

// NodeAllocatedDisksInitialized is a flag for storing
// status NodeAllocatedDisks initialization
var NodeAllocatedDisksInitialized = false

// GetNodeAllocatedDisks is a function for getting allocated disks from node
func GetNodeAllocatedDisks() {
	//nodes, _ := util.GetNodes()
	//nodes:= [3]string{"localhost", "localhost1", "localhost2",}
	//
	//pods := [3]string{"localhost", "localhost", "localhost",}
	pods, _ := util.GetPods()

	for i := range pods {
		NodeAllocatedDisks[pods[i].NodeName] = make(map[util.HalDisk]bool)
		//NodeAllocatedDisks[pods[i]] = make(map[util.HalDisk]bool)

		url := fmt.Sprintf("http://%s:9999/disks", pods[i].PodIP)
		//url := fmt.Sprintf("http://%s:9999/disks", pods[i])
		response, err := http.Get(url)
		if err != nil {
			fmt.Printf("The HTTP request to pod %s failed with error %s\n", pods[i], err)
		} else {
			disks := make([]util.HalDisk, 0)
			data, _ := ioutil.ReadAll(response.Body)
			err = json.Unmarshal(data, &disks)
			if err != nil {
				logrus.Error(err)
			}
			for j := range disks {
				//NodeAllocatedDisks[nodes[i]][disks[i]]= false
				NodeAllocatedDisks[pods[i].NodeName][disks[j]] = false
			}
			logrus.Info("Get disks: ", disks, "from - ", pods[i])
		}

		err = response.Body.Close()
		if err != nil {
			logrus.Error(err)
		}
	}

	logrus.WithFields(logrus.Fields{
		"method": "Driver - GetNodeAllocatedDisks",
		"node":   NodeAllocatedDisks,
	}).Info("Driver - GetNodeAllocatedDisks called")
}

// NewDriver is function for creating CSI driver
func NewDriver(endpoint, driverName, nodeID string) (*ECSCSIDriver, error) {
	logrus.Info("Creating driver for endpoint ", endpoint)

	return &ECSCSIDriver{
		name:     driverName,
		version:  version,
		endpoint: endpoint,
		nodeID:   nodeID,
		ready:    false,
	}, nil
}

// Run is a function for running CSI
func (d *ECSCSIDriver) Run() error {
	u, err := url.Parse(d.endpoint)
	if err != nil {
		return fmt.Errorf("unable to parse address: %q", err)
	}

	addr := path.Join(u.Host, filepath.FromSlash(u.Path))
	if u.Host == "" {
		addr = filepath.FromSlash(u.Path)
	}

	// CSI plugins talk only over UNIX sockets currently
	if u.Scheme != "unix" {
		return fmt.Errorf("currently only unix domain sockets are supported, have: %s", u.Scheme)
	}

	logrus.WithField("socket", addr).Info("removing socket")

	if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove unix domain socket file %s, error: %s", addr, err)
	}

	listener, err := net.Listen(u.Scheme, addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	// log response errors for better observability
	errHandler := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (interface{}, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			logrus.WithError(err).WithField("method", info.FullMethod).Error("method failed")
		}

		return resp, err
	}

	d.srv = grpc.NewServer(grpc.UnaryInterceptor(errHandler))
	csi.RegisterIdentityServer(d.srv, d)
	csi.RegisterControllerServer(d.srv, d)
	csi.RegisterNodeServer(d.srv, d)

	d.ready = true // we're now ready to go!

	logrus.WithField("addr", addr).Info("server started")

	return d.srv.Serve(listener)
}

// Stop is a function for stopping CSI
func (d *ECSCSIDriver) Stop() {
	// TODO: do we need to add lock before calling stop?
	d.ready = false
	d.srv.Stop()
	logrus.Info("Driver stopped. Ready: ", d.ready)
}

func AllocateDisk(allDisks map[string]map[util.HalDisk]bool,
	preferredNode string, requestedCapacity int64) (int64, string, string) {
	// flag to inform that some disks were picked
	var isDiskFound = false
	// picked disk
	var allocatedDisk util.HalDisk
	// volume identifier
	var volumeID string
	// node identifier
	var nodeID string
	// allocated capacity
	var allocatedCapacity int64 = math.MaxInt64

	var selectedDisks = make(map[string]map[util.HalDisk]bool)

	if preferredNode != "" {
		selectedDisks[preferredNode] = allDisks[preferredNode]
	} else {
		selectedDisks = allDisks
	}

	// check all disks
	var pickedDisks = make(map[util.HalDisk]string)
	// pick appropriate
	for node := range selectedDisks {
		for disk, allocated := range selectedDisks[node] {
			if !allocated {
				picked := tryToPick(disk, requestedCapacity)
				if picked {
					pickedDisks[disk] = node
				}
			}
		}
	}

	// choose disk with the minimal size
	for disk, node := range pickedDisks {
		capacity := getCapacityInBytes(disk)
		if capacity < allocatedCapacity {
			allocatedDisk = disk
			allocatedCapacity = capacity
			volumeID = node + "_" + disk.Path
			nodeID = node
			// todo - refactor, no need to update it each time
			isDiskFound = true
		}
	}

	// update map
	if isDiskFound {
		allDisks[nodeID][allocatedDisk] = true
	} else {
		// no capacity allocated
		allocatedCapacity = 0
	}

	if isDiskFound {
		logrus.Info("Disk found on the node - ", nodeID)
	} else {
		logrus.Info("All disks are allocated on requested nodes")
	}

	return allocatedCapacity, nodeID, volumeID
}

// get capacity size in bytes
func getCapacityInBytes(disk util.HalDisk) int64 {
	//expected formats of disk capacity: "4K", "7T", "64G"
	//extract units ("K", "G", "T", "M") from string
	diskUnit := disk.Capacity[len(disk.Capacity)-1:]
	// extract size 4, 7, 64
	diskUnitSize, err := strconv.ParseInt(disk.Capacity[:len(disk.Capacity)-1], 0, 64)
	// return null capacity when unable to decode
	if err != nil {
		logrus.Errorf("Error during converting string to int: %q", err)
		return 0
	}
	// calcucate size in bytes
	return util.FormatCapacity(diskUnitSize, diskUnit)
}

// try to pick the disk. requiredBytes - minimum size of disk.
// return true in case of success, false otherwise
func tryToPick(disk util.HalDisk, requiredBytes int64) bool {
	// skip disk if it has partitions already
	if disk.PartitionCount != 0 {
		return false
	}

	// calc capacity
	capacityBytes := getCapacityInBytes(disk)
	// check whether it matches
	if requiredBytes > capacityBytes {
		logrus.Info("Required bytes more than disk capacity: ", disk.Path)
		return false
	}

	return true
}
