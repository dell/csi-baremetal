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

package capacityplanner

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	apiV1 "github.com/dell/csi-baremetal/api/v1"
	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
)

func TestACReader(t *testing.T) {
	ctx := context.Background()
	logger := testLogger.WithField("component", "test")
	client := getKubeClient(t)
	testACs := []*accrd.AvailableCapacity{
		getTestAC(testNode1, testSmallSize, apiV1.StorageClassHDD),
		getTestAC(testNode2, testLargeSize, apiV1.StorageClassSSD),
		getTestAC(testNode1, testSmallSize, apiV1.StorageClassAny),
	}
	createACsInAPi(t, client, testACs)
	reader := NewACReader(client, logger, true)
	resp, err := reader.ReadCapacity(ctx)
	assert.Nil(t, err)
	assert.Len(t, resp, len(testACs))
}

func TestACRReader(t *testing.T) {
	ctx := context.Background()
	logger := testLogger.WithField("component", "test")
	client := getKubeClient(t)
	testACRs := []*acrcrd.AvailableCapacityReservation{
		getTestACR(testSmallSize, apiV1.StorageClassHDD, nil),
		getTestACR(testLargeSize, apiV1.StorageClassSSD, nil),
		getTestACR(testSmallSize, apiV1.StorageClassAny, nil),
	}
	createACRsInAPi(t, client, testACRs)
	reader := NewACRReader(client, logger, true)
	resp, err := reader.ReadReservations(ctx)
	assert.Nil(t, err)
	assert.Len(t, resp, len(testACRs))
}

/*func TestUnreservedACReader(t *testing.T) {
	ctx := context.Background()
	logger := testLogger.WithField("component", "test")
	client := getKubeClient(t)
	testACs := []*accrd.AvailableCapacity{
		getTestAC(testNode1, testSmallSize, apiV1.StorageClassHDD),
		getTestAC(testNode2, testLargeSize, apiV1.StorageClassSSD),
		getTestAC(testNode1, testSmallSize, apiV1.StorageClassAny),
	}
	testACRs := []*acrcrd.AvailableCapacityReservation{
		getTestACR(testSmallSize, apiV1.StorageClassHDD, testACs[:1]),
		getTestACR(testLargeSize, apiV1.StorageClassSSD, testACs[1:2]),
	}
	createACsInAPi(t, client, testACs)
	createACRsInAPi(t, client, testACRs)
	reader := NewUnreservedACReader(logger,
		NewACReader(client, logger, true),
		NewACRReader(client, logger, true))
	resp, err := reader.ReadCapacity(ctx)
	assert.Nil(t, err)
	assert.Len(t, resp, 1)
	assert.Equal(t, *testACs[2], resp[0])
}*/
