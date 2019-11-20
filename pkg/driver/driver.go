// Package driver implements CSI specification
package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
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

func checkDiskCanBeUsed(disk util.HalDisk, allocated bool, requestCapacity int64) bool {
	//expected formats of disk capacity: "4K", "7T", "64G", extract units ("K", "G", "T", "M") from string
	unit := disk.Capacity[len(disk.Capacity)-1:]
	requiredBytes := util.FormatCapacity(requestCapacity, unit)

	capacity, err := strconv.ParseFloat(disk.Capacity[:len(disk.Capacity)-1], 64)
	if err != nil {
		logrus.Errorf("Error during converting string to int: %q", err)
		return false
	}

	if float64(requiredBytes) > capacity {
		logrus.Info("Required bytes more than disk capacity: ", disk.Path)
		return false
	}

	if !allocated && disk.PartitionCount == 0 {
		//if a disk is not allocated and its capacity is enough then use the disk
		return true
	}

	return false
}
