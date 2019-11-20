package driver

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/sirupsen/logrus"
)

// GetPluginInfo is a function for getting info about plugin
func (d *ECSCSIDriver) GetPluginInfo(ctx context.Context,
	req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	logrus.Info("IdentityServer: GetPluginInfo() call")

	return &csi.GetPluginInfoResponse{
		Name:          d.name,
		VendorVersion: d.version,
	}, nil
}

// GetPluginCapabilities is a function for getting plugin capabilities
func (d *ECSCSIDriver) GetPluginCapabilities(ctx context.Context,
	req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
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
		"response": resp,
		"method":   "get_plugin_capabilities",
	}).Info("get plugin capabitilies called")

	return resp, nil
}

// Probe is a function for querying readiness of the plugin
func (d *ECSCSIDriver) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	logrus.Info("IdentityServer: Probe() call")

	return &csi.ProbeResponse{
		Ready: &wrappers.BoolValue{
			Value: d.ready,
		},
	}, nil
}
