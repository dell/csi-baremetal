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

package controller

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/sirupsen/logrus"
)

// NewIdentityServer is the creator for defaultIdentityServer struct
// Receives name of the driver, driver version and readiness state of the driver
// Returns csi.IdentityServer because defaultIdentityServer struct implements it
func NewIdentityServer(name string, version string) csi.IdentityServer {
	return &defaultIdentityServer{
		name:      name,
		version:   version,
		readiness: true,
	}
}

// defaultIdentityServer is the implementation of IdentityServer interface
type defaultIdentityServer struct {
	name      string
	version   string
	readiness bool
}

// GetPluginInfo is the implementation of CSI Spec GetPluginInfo.
// This method returns information about CSI driver: its name and version.
// Receives golang context and CSI Spec GetPluginInfoRequest
// Returns CSI Spec GetPluginInfoResponse and nil error
func (s *defaultIdentityServer) GetPluginInfo(context.Context, *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	return &csi.GetPluginInfoResponse{
		Name:          s.name,
		VendorVersion: s.version,
	}, nil
}

// GetPluginCapabilities is the implementation of CSI Spec GetPluginCapabilities. This method returns information about
// capabilities of  CSI driver. CONTROLLER_SERVICE and VOLUME_ACCESSIBILITY_CONSTRAINTS for now.
// Receives golang context and CSI Spec GetPluginCapabilitiesRequest
// Returns CSI Spec GetPluginCapabilitiesResponse and nil error
func (s *defaultIdentityServer) GetPluginCapabilities(context.Context, *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	resp := &csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS,
					},
				},
			},
			{
				Type: &csi.PluginCapability_VolumeExpansion_{
					VolumeExpansion: &csi.PluginCapability_VolumeExpansion{
						Type: csi.PluginCapability_VolumeExpansion_ONLINE,
					},
				},
			},
			{
				Type: &csi.PluginCapability_VolumeExpansion_{
					VolumeExpansion: &csi.PluginCapability_VolumeExpansion{
						Type: csi.PluginCapability_VolumeExpansion_OFFLINE,
					},
				},
			},
		},
	}
	logrus.WithFields(logrus.Fields{
		"component": "identityService",
		"method":    "GetPluginCapabilities",
	}).Infof("Response: %v", resp)
	return resp, nil
}

// Probe is the implementation of CSI Spec Probe. This method checks if CSI driver is ready to serve requests
// Receives golang context and CSI Spec ProbeRequest
// Returns CSI Spec ProbeResponse and nil error
func (s *defaultIdentityServer) Probe(context.Context, *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	return &csi.ProbeResponse{Ready: &wrappers.BoolValue{Value: s.readiness}}, nil
}
