package controller

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func NewIdentityServer() csi.IdentityServer {
	return &identityServer{}
}

// interface implementation for IdentityServer
type identityServer struct {
}

func (s identityServer) GetPluginInfo(context.Context, *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}

func (s identityServer) GetPluginCapabilities(context.Context, *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}

func (s identityServer) Probe(context.Context, *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented yet")
}
