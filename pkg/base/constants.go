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

// Package base is for basic methods which can be used by all CSI components
package base

import "time"

// CtxKey variable type uses for keys in context WithValue
type CtxKey string

// PluginVersion is a version of current CSI plugin
var PluginVersion = ""

// ProjectPath is the current path of a project
var ProjectPath = ""

const (
	// RequestUUID is the constant for context request
	RequestUUID CtxKey = "RequestUUID"
	// PluginName is a name of current CSI plugin
	PluginName = "csi-baremetal"
	// DefaultDriveMgrEndpoint is the default gRPC endpoint for drivemgr
	DefaultDriveMgrEndpoint = "tcp://:8888"
	// DefaultHealthIP is the default gRPC IP for Health server
	DefaultHealthIP = ""
	// DefaultHealthPort is the default gRPC port for Health Server
	DefaultHealthPort = 9999
	// DefaultExtenderPort is the default http port for scheduler extender
	DefaultExtenderPort = 8889

	// KubeletRootPath is the pods' path on the node
	KubeletRootPath = "/var/lib/kubelet/pods"

	// HostRootPath is root mount
	HostRootPath = "/hostroot"

	// DefaultTimeoutForVolumeOperations is the timeout in which we expect that any operation with volume should be finished
	DefaultTimeoutForVolumeOperations = 10 * time.Minute

	// DefaultTimeoutForVolumeUpdate is the timeout in which we expect that any update with volume
	DefaultTimeoutForVolumeUpdate = 30 * time.Second

	// DefaultRequeueForVolume is the interval for volume reconcile
	DefaultRequeueForVolume = 5 * time.Second

	// DefaultFsType FS type that used by default
	DefaultFsType = "xfs"

	// StorageTypeKey key from volume_context in CreateVolumeRequest of NodePublishVolumeRequest
	StorageTypeKey = "storageType"
	// SizeKey key from volume_context in CreateVolumeRequest of NodePublishVolumeRequest
	SizeKey = "size"
	// DefaultNamespace represents default namespace in Kubernetes
	DefaultNamespace = "default"
)
