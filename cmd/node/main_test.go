/*
Copyright © 2024 Dell Inc. or its subsidiaries. All Rights Reserved.

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

package main

import (
	"context"
	"testing"
	"net/http"
	"io"
	"time"

	"encoding/json"
	"github.com/stretchr/testify/assert"

	"github.com/dell/csi-baremetal/pkg/base/rpc"
	"github.com/dell/csi-baremetal/pkg/metrics"
	"github.com/dell/csi-baremetal/pkg/mocks"
	"github.com/sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"
)


var (
	tEnableMetrics  = true
	tEnableSmart    = true
	tMetricsPath    = "/metrics"
	tSmartPath      = "/smart"
	tMetricsAddress = "localhost:8787"
	tLogger         = logrus.New()
	tCsiEndpoint    = "unix:///tmp/csi.sock"
	maxRetries      = 5
	smartInfo       = mocks.SmartInfo {
			"S3FXNA0J400001": {
				"device": {
					"DevPath": "/dev/sg0",
					"Vendor": "SAMSUNG",
					"modelName": "MZILS1T6HEJH0D3",
					"SerialNum": "S3FXNA0J400001",
				},
			},
			"S3FXNA0J400002": {
				"device": {
					"DevPath": "/dev/sg0",
					"Vendor": "SAMSUNG",
					"modelName": "MZILS1T6HEJH0D3",
					"SerialNum": "S3FXNA0J400002",
				},
			},
			"S3FXNA0J400003": {
				"device": {
					"DevPath": "/dev/sg0",
					"Vendor": "SAMSUNG",
					"modelName": "MZILS1T6HEJH0D3",
					"SerialNum": "S3FXNA0J400003",
				},
			},
		}
)

func getSmartInfo(serialNumber string) (*http.Response, error) {
	var resp *http.Response
	var err error

	for count := 0; count < maxRetries; count++ {
		resp, err = http.Get("http://" + tMetricsAddress + tSmartPath + "/" + serialNumber)
		if err == nil {
			break
		}
		time.Sleep(time.Second)	
	}
	return resp, err
}

func getAllDrivesSmartInfo() (*http.Response, error) {
	var resp *http.Response
	var err error

	for count := 0; count < maxRetries; count++ {
		resp, err = http.Get("http://" + tMetricsAddress + tSmartPath)
		if err == nil {
			break
		}
		time.Sleep(time.Second)	
	}
	return resp, err
}

func TestNode_GetAllDrivesSmartInfo(t *testing.T) {
	type test struct {
		smartInfo  mocks.SmartInfo
		statusCode int
	}

	tests := []test{
		{ smartInfo: smartInfo, statusCode: http.StatusOK },
		{ smartInfo: nil,       statusCode: http.StatusNotFound },
	}

	for _, tc := range tests {
		expectedSmartInfo, err := json.Marshal(tc.smartInfo)
		http.DefaultServeMux = new(http.ServeMux)
		clientToDriveMgr := mocks.NewMockDriveMgrClient(nil, tc.smartInfo)
		srv := enableHTTPServers(false,
			tEnableSmart,
			&tMetricsAddress,
			&tMetricsPath,
			&tSmartPath,
			clientToDriveMgr,
			nil,
			tLogger)
		ctx, shutdownRelease := context.WithTimeout(context.Background(), 10 * time.Second)
		defer shutdownRelease()
	
		resp, err := getAllDrivesSmartInfo()
		assert.Nil(t, err)
		assert.Equal(t, tc.statusCode, resp.StatusCode)
	
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		assert.Nil(t, err)
	
		var respSmartInfo map[string]interface{}
		json.Unmarshal(body, &respSmartInfo)
	
		switch resp.StatusCode {
		case http.StatusOK:
			assert.Equal(t, string(expectedSmartInfo), respSmartInfo["smartInfo"])
		case http.StatusNotFound:
			assert.Nil(t, respSmartInfo)
		default:
			assert.Fail(t, "Unexpected status code: %d", resp.StatusCode )
		}

		srv.Shutdown(ctx)
	}
}

func TestNode_GetSmartInfo(t *testing.T) {
	type test struct {
		serialNumber string
		statusCode   int
	}

	tests := []test{
		{ serialNumber: "S3FXNA0J400001", statusCode: http.StatusOK },
		{ serialNumber: "S3FXNA0J400002", statusCode: http.StatusOK },
		{ serialNumber: "S3FXNA0J400003", statusCode: http.StatusOK },
		{ serialNumber: "S3FXNA0J400004", statusCode: http.StatusNotFound },
	}

	for _, tc := range tests {
		expectedSmartInfo, err := json.Marshal(smartInfo[tc.serialNumber])
		http.DefaultServeMux = new(http.ServeMux)
		clientToDriveMgr := mocks.NewMockDriveMgrClient(nil, smartInfo)
		srv := enableHTTPServers(false,
			tEnableSmart,
			&tMetricsAddress,
			&tMetricsPath,
			&tSmartPath,
			clientToDriveMgr,
			nil,
			tLogger)
		ctx, shutdownRelease := context.WithTimeout(context.Background(), 10 * time.Second)
		defer shutdownRelease()

		resp, err := getSmartInfo(tc.serialNumber)
		assert.Nil(t, err)
		assert.Equal(t, tc.statusCode, resp.StatusCode)

		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		assert.Nil(t, err)

		var respSmartInfo map[string]string
		json.Unmarshal(body, &respSmartInfo)

		switch resp.StatusCode {
		case http.StatusOK:
			assert.Equal(t, string(expectedSmartInfo), respSmartInfo["smartInfo"])
		case http.StatusNotFound:
			assert.Nil(t, respSmartInfo)
		default:
			assert.Fail(t, "Unexpected status code: %d", resp.StatusCode )
		}

		srv.Shutdown(ctx)
	}
}

func TestNode_GetAllDrivesSmartMetricsUds(t *testing.T) {
	type test struct {
		smartInfo  mocks.SmartInfo
		statusCode int
	}

	tests := []test{
		{ smartInfo: smartInfo, statusCode: http.StatusOK },
		{ smartInfo: nil,       statusCode: http.StatusNotFound },
	}

	for _, tc := range tests {
		expectedSmartInfo, err := json.Marshal(tc.smartInfo)
		http.DefaultServeMux = new(http.ServeMux)
		clientToDriveMgr := mocks.NewMockDriveMgrClient(nil, tc.smartInfo)
		csiUDSServer := rpc.NewServerRunner(nil, tCsiEndpoint, tEnableMetrics, tLogger)
		srv := enableHTTPServers(tEnableMetrics,
			tEnableSmart,
			&tMetricsAddress,
			&tMetricsPath,
			&tSmartPath,
			clientToDriveMgr,
			csiUDSServer,
			tLogger)
		ctx, shutdownRelease := context.WithTimeout(context.Background(), 10 * time.Second)
		defer shutdownRelease()
	
		resp, err := getAllDrivesSmartInfo()
		assert.Nil(t, err)
		assert.Equal(t, tc.statusCode, resp.StatusCode)
	
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		assert.Nil(t, err)
	
		var respSmartInfo map[string]interface{}
		json.Unmarshal(body, &respSmartInfo)
	
		switch resp.StatusCode {
		case http.StatusOK:
			assert.Equal(t, string(expectedSmartInfo), respSmartInfo["smartInfo"])
		case http.StatusNotFound:
			assert.Nil(t, respSmartInfo)
		default:
			assert.Fail(t, "Unexpected status code: %d", resp.StatusCode )
		}

		prometheus.Unregister(metrics.BuildInfo)
		srv.Shutdown(ctx)
	}
}

func TestNode_HTTPServer_Disabled(t *testing.T) {
	http.DefaultServeMux = new(http.ServeMux)
	srv := enableHTTPServers(false,
		false,
		&tMetricsAddress,
		&tMetricsPath,
		&tSmartPath,
		nil,
		nil,
		tLogger)
	assert.Nil(t, srv)
}

func TestNode_MockDriveMgrClientFail(t *testing.T) {
	http.DefaultServeMux = new(http.ServeMux)
	clientToDriveMgr := mocks.MockDriveMgrClientFail{}
	srv := enableHTTPServers(false,
		tEnableSmart,
		&tMetricsAddress,
		&tMetricsPath,
		&tSmartPath,
		&clientToDriveMgr,
		nil,
		tLogger)
	ctx, shutdownRelease := context.WithTimeout(context.Background(), 10 * time.Second)
	defer shutdownRelease()

	resp, err := getSmartInfo("XXX")
	assert.Nil(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	resp, err = getAllDrivesSmartInfo()
	assert.Nil(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	srv.Shutdown(ctx)
}
