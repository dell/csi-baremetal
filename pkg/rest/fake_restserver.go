package rest

import (
	"encoding/json"
	"fmt"
	"net/http"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/lvm"
	"github.com/jarcoal/httpmock"
	"github.com/sirupsen/logrus"
)

type FakeServer struct {
	ListenAddress string // protocol://ip:port
	Endpoints     map[string]string
}

func NewFakeServer(mockHost string) *FakeServer {
	var endpoints = map[string]string{
		"lv": fmt.Sprintf("%s/lv", mockHost),
		"vg": fmt.Sprintf("%s/vg", mockHost),
	}

	return &FakeServer{
		ListenAddress: mockHost,
		Endpoints:     endpoints,
	}
}

func (fs *FakeServer) PrepareLVRespondersCode200(lv *lvm.LogicalVolume) {
	// return 200 on PUT /lv BODY
	httpmock.RegisterResponder("PUT", fs.Endpoints["lv"], func(req *http.Request) (*http.Response, error) {
		var putLV lvm.LogicalVolume
		err := json.NewDecoder(req.Body).Decode(&putLV)
		if err != nil {
			return httpmock.NewStringResponse(500, ""), nil
		}
		logrus.Infof("created LV: %v", putLV)
		return httpmock.NewStringResponse(200, ""), nil
	})
	// return 200 on DELETE /lv BODY
	httpmock.RegisterResponder("DELETE", fs.Endpoints["lv"], func(req *http.Request) (*http.Response, error) {
		err := json.NewDecoder(req.Body).Decode(lv)
		if err != nil {
			return httpmock.NewStringResponse(500, ""), nil
		}
		logrus.Infof("removed LV: %v", lv)
		return httpmock.NewStringResponse(200, ""), nil
	})
}

func (fs *FakeServer) PrepareLVRespondersCode500() {
	// return 500 on PUT /lv BODY
	httpmock.RegisterResponder("PUT", fs.Endpoints["lv"], func(req *http.Request) (*http.Response, error) {
		return httpmock.NewStringResponse(500, ""), nil
	})
	// return 500 on DELETE /lv BODY
	httpmock.RegisterResponder("DELETE", fs.Endpoints["lv"], func(req *http.Request) (*http.Response, error) {
		return httpmock.NewStringResponse(500, ""), nil
	})
}

func (fs *FakeServer) PrepareVGRespondersCode500() {
	httpmock.RegisterResponder("GET", fs.Endpoints["vg"], func(req *http.Request) (*http.Response, error) {
		return httpmock.NewStringResponse(500, ""), nil
	})
}

func (fs *FakeServer) PrepareVGRespondersCode200(vg *lvm.VolumeGroup) {
	// return 200 and volume group response on GET /vg BODY
	httpmock.RegisterResponder("GET", fs.Endpoints["vg"], func(req *http.Request) (*http.Response, error) {
		resp, err := httpmock.NewJsonResponse(200, vg)
		if err != nil {
			return httpmock.NewStringResponse(500, ""), nil
		}
		return resp, nil
	})
}
