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
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	sm "eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/storagemanager"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/util"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
)

// ECSCSIDriver is a struct for CSI plugin
type ECSCSIDriver struct {
	name     string
	version  string
	endpoint string
	nodeID   string
	LVMMode  bool
	ready    bool

	SS  sm.StorageSubsystem
	srv *grpc.Server
}

// TODO: generate version
const (
	version = "0.0.2"
)

// Mutex is a sync primitive to synchronize CreateVolume calls
var Mutex = &sync.Mutex{}

// NodeAllocatedDisks is a map for storing disk and allocation status
// k - hostname, v: k - disk, v -a allocated
var NodeAllocatedDisks = make(map[string]map[util.HalDisk]bool)

// NodeAllocatedDisksInitialized is a flag for storing
// status NodeAllocatedDisks initialization
var NodeAllocatedDisksInitialized = false

var logger = util.CreateLogger("driver")

// GetNodeAllocatedDisks is a function for getting allocated disks from node
func GetNodeAllocatedDisks() {
	ll := logger.WithField("method", "GetNodeAllocatedDisks")
	pods, _ := util.GetNodeServicePods()

	var netTransport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 50 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 15 * time.Second,
	}
	commonClient := http.Client{Timeout: 60 * time.Second, Transport: netTransport}
	for i := range pods {
		NodeAllocatedDisks[pods[i].NodeName] = make(map[util.HalDisk]bool)

		disksURL := fmt.Sprintf("http://%s:9999/disks", pods[i].PodIP)

		ll.Infof("Sending GET %s", disksURL)
		response, err := commonClient.Get(disksURL)
		if err != nil {
			ll.Fatalf("The HTTP request to pod %v failed with error %+v. Exit", pods[i], err)
		} else {
			ll.Infof("Received: %v", response)
			disks := make([]util.HalDisk, 0)
			data, _ := ioutil.ReadAll(response.Body)
			err = json.Unmarshal(data, &disks)
			if err != nil {
				ll.Error(err)
			}
			for j := range disks {
				NodeAllocatedDisks[pods[i].NodeName][disks[j]] = false
			}
			ll.Infof("From - %s, got disks: %v ", pods[i].PodIP, disks)
		}

		if response != nil {
			err = response.Body.Close()
			if err != nil {
				ll.Error(err)
			}
		}
	}

	logger.WithFields(logrus.Fields{
		"method": "Driver - GetNodeAllocatedDisks",
		"node":   NodeAllocatedDisks,
	}).Info("Driver - GetNodeAllocatedDisks called")
}

// NewDriver is function for creating CSI driver
func NewDriver(endpoint, driverName, nodeID string, lvmMode bool) (*ECSCSIDriver, error) {
	logger.Info("Creating driver for endpoint ", endpoint)

	var ss sm.StorageSubsystem
	if lvmMode {
		ss = &sm.LVMVolumeManager{}
	} else {
		ss = nil
	}

	return &ECSCSIDriver{
		name:     driverName,
		version:  version,
		endpoint: endpoint,
		nodeID:   nodeID,
		LVMMode:  lvmMode,
		ready:    false,
		SS:       ss,
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

	logger.WithField("socket", addr).Info("removing socket")

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
			logger.WithError(err).WithField("method", info.FullMethod).Error("method failed")
		}

		return resp, err
	}

	d.srv = grpc.NewServer(grpc.UnaryInterceptor(errHandler))
	csi.RegisterIdentityServer(d.srv, d)
	csi.RegisterControllerServer(d.srv, d)
	csi.RegisterNodeServer(d.srv, d)

	d.ready = true // we're now ready to go!

	logger.WithField("addr", addr).Info("server started")

	return d.srv.Serve(listener)
}

// Stop is a function for stopping CSI
func (d *ECSCSIDriver) Stop() {
	// TODO: do we need to add lock before calling stop?
	d.ready = false
	d.srv.Stop()
	logger.Info("Driver stopped. Ready: ", d.ready)
}

func AllocateDisk(allDisks map[string]map[util.HalDisk]bool,
	preferredNode string, requestedCapacity int64) (int64, string, string) {
	ll := logger.WithField("method", "AllocateDisk")
	ll.Infof("Got disks map: %v, preferredNode %s, capacity %d", allDisks, preferredNode, requestedCapacity)

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
				ll.Infof("Try to Pick on node %s disk %s", node, disk.Path)
				picked := tryToPick(disk, requestedCapacity)
				if picked {
					pickedDisks[disk] = node
				}
			}
		}
	}

	// choose disk with the minimal size
	for disk, node := range pickedDisks {
		capacity := getCapacityInBytes(disk.Capacity)
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
		ll.Infof("Disk found on the node - %s", nodeID)
	} else {
		ll.Info("All disks are allocated on requested nodes")
	}

	return allocatedCapacity, nodeID, volumeID
}

// get capacity size in bytes
func getCapacityInBytes(capacity string) int64 {
	//expected formats of disk capacity: "4K", "7T", "64G"
	//extract units ("K", "G", "T", "M") from string
	logger.Infof("getCapacityInBytes: [%s]", capacity)
	diskUnit := capacity[len(capacity)-1:]
	// extract size 4, 7, 64
	diskUnitSize, err := strconv.ParseFloat(capacity[:len(capacity)-1], 64)
	// return zero capacity when unable to decode
	if err != nil {
		logger.Errorf("Error during converting string '%s' to float: %q", capacity, err)
		return 0
	}
	// calculate size in bytes
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
	capacityBytes := getCapacityInBytes(disk.Capacity)
	// check whether it matches
	if requiredBytes > capacityBytes {
		logger.WithField("method", "tryToPick").Info("Required bytes more than disk capacity: ", disk.Path)
		return false
	}

	return true
}

func ReleaseDisk(volumeID string, disks map[string]map[util.HalDisk]bool) error {
	Mutex.Lock()
	defer Mutex.Unlock()
	//volumeid is nodeId_path
	deletedDisk := strings.Split(volumeID, "_") // TODO: handle index out of range error or implement struct for ID

	if len(deletedDisk) != 2 {
		return fmt.Errorf("invalid volumeID: %v", volumeID)
	}

	node := deletedDisk[0]
	diskPath := deletedDisk[1]

	for disk := range disks[node] {
		if disk.Path == diskPath {
			disks[node][disk] = false
			break
		}
	}

	logger.Info("Disks state after resetting cache: ", disks[node])

	return nil
}
