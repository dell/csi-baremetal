package controller

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/sirupsen/logrus"
)

// NewIdentityServer is the creator for identityServer struct
// Receives name of the driver, driver version and readiness state of the driver
// Returns csi.IdentityServer because identityServer struct implements it
func NewIdentityServer(name string, version string, readiness bool) csi.IdentityServer {
	return &identityServer{name, version, readiness}
}

// identityServer is the implementation of IdentityServer interface
type identityServer struct {
	name      string
	version   string
	readiness bool
}

// GetPluginInfo is the implementation of CSI Spec GetPluginInfo.
// This method returns information about CSI driver: its name and version.
// Receives golang context and CSI Spec GetPluginInfoRequest
// Returns CSI Spec GetPluginInfoResponse and nil error
func (s *identityServer) GetPluginInfo(context.Context, *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	return &csi.GetPluginInfoResponse{
		Name:          s.name,
		VendorVersion: s.version,
	}, nil
}

// GetPluginCapabilities is the implementation of CSI Spec GetPluginCapabilities. This method returns information about
// capabilities of  CSI driver. CONTROLLER_SERVICE and VOLUME_ACCESSIBILITY_CONSTRAINTS for now.
// Receives golang context and CSI Spec GetPluginCapabilitiesRequest
// Returns CSI Spec GetPluginCapabilitiesResponse and nil error
func (s *identityServer) GetPluginCapabilities(context.Context, *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
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
func (s *identityServer) Probe(context.Context, *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	return &csi.ProbeResponse{Ready: &wrappers.BoolValue{Value: s.readiness}}, nil
}
