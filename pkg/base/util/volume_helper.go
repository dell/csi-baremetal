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

// Package util contains common utilities
package util

import (
	"errors"
	"strings"
)

const (
	pvcPrefix = "pvc-"
	csiPrefix = "csi-"

	// VolumeInfoKey is the constant for context request
	VolumeInfoKey CtxKey = "VolumeInfo"
	// ClaimNamespaceKey is a key from volume_context in CreateVolumeRequest of NodePublishVolumeRequest
	ClaimNamespaceKey = "csi.storage.k8s.io/pvc/namespace"
	// ClaimNameKey is a key from volume_context in CreateVolumeRequest of NodePublishVolumeRequest
	ClaimNameKey = "csi.storage.k8s.io/pvc/name"
	// PodNamespaceKey to read pod namespace from PodInfoOnMount feature
	PodNamespaceKey = "csi.storage.k8s.io/pod.namespace"
	// PodNameKey to read pod name from PodInfoOnMount feature
	PodNameKey = "csi.storage.k8s.io/pod.name"
)

// CtxKey variable type uses for keys in context WithValue
type CtxKey string

// VolumeInfo holds information about Kubernetes PVC
type VolumeInfo struct {
	Namespace string
	Name      string
}

// NewVolumeInfo receives parameters from CreateVolumeRequest and returns new VolumeInfo
func NewVolumeInfo(parameters map[string]string) (*VolumeInfo, error) {
	claimNamespace, ok := parameters[ClaimNamespaceKey]
	if !ok {
		return nil, errors.New("persistent volume claim namespace is not set in request")
	}
	// PVC name
	claimName, ok := parameters[ClaimNameKey]
	if !ok {
		return nil, errors.New("persistent volume claim name is not set in request")
	}

	return &VolumeInfo{claimNamespace, claimName}, nil
}

// IsDefaultNamespace returns true when namespace is not defined and false otherwise
func (v *VolumeInfo) IsDefaultNamespace() bool {
	return v.Namespace == ""
}

// GetVolumeUUID extracts UUID from volume ID: pvc-<UUID>
// Method will remove pvcPrefix `pvc-` and return UUID
func GetVolumeUUID(volumeID string) (string, error) {
	// check that volume ID is correct
	if volumeID == "" {
		return "", errors.New("volume ID is empty")
	}

	// trim pvcPrefix
	uuid := strings.TrimPrefix(volumeID, pvcPrefix)
	// return error if volume UUID is empty
	if uuid == "" {
		return "", errors.New("volume UUID is empty")
	}
	// is PV UUID RFC 4122 compatible?
	return uuid, nil
}

// HasNameWithPrefix check whether slice has a string
// with pvcPrefix pvc or not
func HasNameWithPrefix(names []string) bool {
	for _, name := range names {
		if strings.HasPrefix(name, pvcPrefix) || strings.HasPrefix(name, csiPrefix) {
			return true
		}
	}
	return false
}
