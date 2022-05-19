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
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/dell/csi-baremetal/pkg/base/command"
	ph "github.com/dell/csi-baremetal/pkg/base/linuxutils/partitionhelper"
	"github.com/dell/csi-baremetal/pkg/metrics"
)

// PartitionOperations is a high-level interface
// that encapsulates all low-level operations with partitions on node
type PartitionOperations interface {
	// PreparePartition is fully prepare partition on node for use
	PreparePartition(p Partition) (*Partition, error)
	// ReleasePartition is fully release resources that had consumed by partition on node
	ReleasePartition(p Partition) error
	// SearchPartName returns partition name
	SearchPartName(device, partUUID string) string
	ph.WrapPartition
}

const (
	// NumberOfRetriesToSyncPartTable how many times to sync fs tab
	NumberOfRetriesToSyncPartTable = 3
	// SleepBetweenRetriesToSyncPartTable default timeout between fs tab sync attempt
	SleepBetweenRetriesToSyncPartTable = 3 * time.Second
)

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
			p.Name = d.SearchPartName(p.Device, p.PartUUID)
			if p.Name == "" {
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
	_ = d.SyncPartitionTable(p.Device)

	p.Name = d.SearchPartName(p.Device, p.PartUUID)
	if p.Name == "" {
		return nil, fmt.Errorf("unable to determine partition name after it being created: %v", err)
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
func (d *PartitionOperationsImpl) SearchPartName(device, partUUID string) string {
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
	for i := 0; i < NumberOfRetriesToSyncPartTable; i++ {
		// sync partition table
		err = d.SyncPartitionTable(device)
		if err != nil {
			// log and ignore error
			ll.Warningf("Unable to sync partition table for device %s", device)
		}
		time.Sleep(SleepBetweenRetriesToSyncPartTable)
		partName, err = d.GetPartitionNameByUUID(device, partUUID)
		if err != nil {
			ll.Debugf("unable to find part name: %v", err)
			continue
		}
		break
	}

	ll.Debugf("Got partition number %s", partName)
	return partName
}
