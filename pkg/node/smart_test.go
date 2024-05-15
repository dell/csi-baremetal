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

package node

import (
	"context"
	"testing"

	api "github.com/dell/csi-baremetal/api/smart/generated"
	"github.com/dell/csi-baremetal/pkg/mocks"
	"github.com/stretchr/testify/assert"
)

var (
	invalidJSONClient = &mocks.MockDriveMgrClientFailJSON{MockJSON: "let's fail"}
	validJSONClient   = &mocks.MockDriveMgrClientFailJSON{MockJSON: `{"test": "ok"}`}

	validJSONSrv   = NewSmartService(validJSONClient, testLogger)
	invalidJSONSrv = NewSmartService(invalidJSONClient, testLogger)
)

func TestGetDriveSmartInfo(t *testing.T) {
	t.Run("Parse_JSON_error", func(t *testing.T) {
		res, err := invalidJSONSrv.GetDriveSmartInfo(context.Background(), api.GetDriveSmartInfoParams{})

		assert.Nil(t, err)
		assert.IsType(t, res, &api.GetDriveSmartInfoInternalServerError{})
	})

	t.Run("Parse_JSON_success", func(t *testing.T) {
		res, err := validJSONSrv.GetDriveSmartInfo(context.Background(), api.GetDriveSmartInfoParams{})

		assert.Nil(t, err)
		assert.IsType(t, res, &api.SmartMetrics{})
	})
}
