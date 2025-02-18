package controller

import (
	"context"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/assert"
)

func Test_GetPluginInfo(t *testing.T) {
	t.Run("Test GetPluginInfo", func(t *testing.T) {
		ctx := context.Background()

		defaultIdentityServer := NewIdentityServer("Test", "1.0.0")
		response, err := defaultIdentityServer.GetPluginInfo(ctx, nil)
		assert.Nil(t, err)

		assert.Equal(t, response.Name, "Test")
		assert.Equal(t, response.VendorVersion, "1.0.0")
	})
}

func Test_GetPluginCapabilities(t *testing.T) {
	t.Run("Test GetPluginCapabilities", func(t *testing.T) {
		ctx := context.Background()

		defaultIdentityServer := NewIdentityServer("Test", "1.0.0")
		response, err := defaultIdentityServer.GetPluginCapabilities(ctx, nil)
		assert.Nil(t, err)

		assert.Equal(t, response.Capabilities[0].GetService().Type, csi.PluginCapability_Service_CONTROLLER_SERVICE)
		assert.Equal(t, response.Capabilities[1].GetService().Type, csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS)
		assert.Equal(t, response.Capabilities[2].GetVolumeExpansion().Type, csi.PluginCapability_VolumeExpansion_ONLINE)
		assert.Equal(t, response.Capabilities[3].GetVolumeExpansion().Type, csi.PluginCapability_VolumeExpansion_OFFLINE)
	})
}

func Test_Probe(t *testing.T) {
	t.Run("Test Probe", func(t *testing.T) {
		ctx := context.Background()

		defaultIdentityServer := NewIdentityServer("Test", "1.0.0")
		response, err := defaultIdentityServer.Probe(ctx, nil)
		assert.Nil(t, err)

		assert.True(t, response.Ready.Value)
	})
}
