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

package utilwrappers

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/dell/csi-baremetal/pkg/base/command"
	"github.com/dell/csi-baremetal/pkg/base/linuxutils/fs"
	ph "github.com/dell/csi-baremetal/pkg/base/linuxutils/partitionhelper"
	"github.com/dell/csi-baremetal/pkg/metrics"
)

const (
	// NumberOfRetriesToObtainPartName how many retries to obtain partition name
	NumberOfRetriesToObtainPartName = 5
	// SleepBetweenRetriesToObtainPartName default timeout between retries to obtain partition name
	SleepBetweenRetriesToObtainPartName = 3 * time.Second
)

// PartitionOperations is a high-level interface
// that encapsulates all low-level operations with partitions on node
type PartitionOperations interface {
	// PreparePartition is fully prepare partition on node for use
	PreparePartition(p Partition) (*Partition, error)
	// ReleasePartition is fully release resources that had consumed by partition on node
	ReleasePartition(p Partition) error
	// SearchPartName returns partition name
	SearchPartName(device, partUUID string) (string, error)
	ph.WrapPartition
}

// Partition is hold all attributes of partition on block device
type Partition struct {
	Device    string
	Name      string
	Num       string
	TableType string
	Label     string
	PartUUID  string
}

// GetFullPath return full path of partition, that path could be used for file system operations
func (p *Partition) GetFullPath() string {
	return p.Device + p.Name
}

// PartitionOperationsImpl is a base implementation for PartitionOperations interface
type PartitionOperationsImpl struct {
	ph.WrapPartition
	fs.WrapFS
	log     *logrus.Entry
	metrics metrics.Statistic
}

// NewPartitionOperationsImpl constructor for PartitionOperationsImpl and returns pointer on it
func NewPartitionOperationsImpl(e command.CmdExecutor, log *logrus.Logger) *PartitionOperationsImpl {
	var partMetrics = metrics.NewMetrics(prometheus.HistogramOpts{
		Name:    "partition_operations_duration",
		Help:    "partition operations methods duration",
		Buckets: metrics.ExtendedDefBuckets,
	}, "method")
	if err := prometheus.Register(partMetrics.Collect()); err != nil {
		log.WithField("component", "NewPartitionOperationsImpl").
			Errorf("Failed to register metric: %v", err)
	}
	return &PartitionOperationsImpl{
		WrapPartition: ph.NewWrapPartitionImpl(e, log),
		WrapFS:        fs.NewFSImpl(e),
		log:           log.WithField("component", "PartitionOperations"),
		metrics:       partMetrics,
	}
}

// PreparePartition completely creates and prepares partition p on node
// After that FS could be created on partition
func (d *PartitionOperationsImpl) PreparePartition(p Partition) (*Partition, error) {
	defer d.metrics.EvaluateDurationForMethod("PreparePartition")()
	ll := d.log.WithFields(logrus.Fields{
		"method":   "PreparePartition",
		"volumeID": p.PartUUID,
	})
	ll.Debugf("Processing for partition %#v", p)

	exist, err := d.IsPartitionExists(p.Device, p.Num)
	if err != nil {
		return nil, fmt.Errorf("unable to determine partition existence: %v", err)
	}

	if exist { // check partition UUID
		currUUID, err := d.GetPartitionUUID(p.Device, p.Num)
		if err != nil {
			return nil, fmt.Errorf("partition has already exist on device %s, fail to get it UUID", p.Device)
		}
		if currUUID == p.PartUUID {
			ll.Infof("Partition has already prepared.")
			p.Name, err = d.SearchPartName(p.Device, p.PartUUID)
			if err != nil {
				return nil, fmt.Errorf("unable to determine partition name after it being created: %w", err)
			}
			return &p, nil
		}
		return nil, fmt.Errorf("partition %v has already exist but have another UUID - %s", p, currUUID)
	}

	// create partition table
	if err = d.CreatePartitionTable(p.Device, p.TableType); err != nil {
		return nil, fmt.Errorf("unable to create partition table: %v", err)
	}

	// create partition
	if err = d.CreatePartition(p.Device, p.Label, p.PartUUID); err != nil {
		return nil, fmt.Errorf("unable to create partition: %v", err)
	}

	// obtain partition name
	p.Name, err = d.SearchPartName(p.Device, p.PartUUID)
	if err != nil {
		return nil, fmt.Errorf("unable to determine partition name after it being created: %w", err)
	}

	return &p, nil
}

// ReleasePartition completely removes partition p
func (d *PartitionOperationsImpl) ReleasePartition(p Partition) error {
	defer d.metrics.EvaluateDurationForMethod("ReleasePartition")()
	d.log.WithFields(logrus.Fields{
		"method":   "ReleasePartition",
		"volumeID": p.PartUUID,
	}).Infof("Processing for %v", p)

	exist, err := d.IsPartitionExists(p.Device, p.Num)
	if err != nil {
		return fmt.Errorf("unable to determine partition existence: %v", err)
	}
	if exist {
		return d.DeletePartition(p.Device, p.Num)
	}
	return nil
}

// SearchPartName search (with retries) partition with UUID partUUID on device and returns partition name
// e.g. "1" for /dev/sda1, "p1n1" for /dev/loopbackp1n1
func (d *PartitionOperationsImpl) SearchPartName(device, partUUID string) (string, error) {
	defer d.metrics.EvaluateDurationForMethod("SearchPartName")()
	ll := d.log.WithFields(logrus.Fields{
		"method":   "SearchPartName",
		"volumeID": partUUID,
	})
	ll.Debugf("Search partition number for device %s and uuid %s", device, partUUID)

	var (
		partName string
		err      error
	)

	// get partition name
	for i := 0; i < NumberOfRetriesToObtainPartName; i++ {
		// sync partition table
		_, errStr, err := d.SyncPartitionTable(device)
		if err != nil {
			if _, ok := err.(*exec.ExitError); ok &&
				strings.Contains(errStr, "Device or resource busy") {
				ll.Warningf("Unable to sync partition table for device %s: %v due to device is busy", device, err)
			} else {
				// log and ignore error
				ll.Errorf("Unable to sync partition table for device %s: %v", device, err)
				return "", err
			}
		}
		// sleep first to avoid issues with lsblk caching
		time.Sleep(SleepBetweenRetriesToObtainPartName)
		partName, err = d.GetPartitionNameByUUID(device, partUUID)
		if err != nil {
			ll.Warningf("Unable to find part name: %v. Sleep and retry...", err)
			continue
		}
		break
	}

	// partition not found
	if partName == "" {
		ll.Errorf("Partition not found: %v", err)
		return "", err
	}

	ll.Debugf("Got partition number %s", partName)
	return partName, nil
}
