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

package events_test

import (
	"log"

	"github.com/sirupsen/logrus"

	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/pkg/base/k8s"
	"github.com/dell/csi-baremetal/pkg/events"
)

func Example() {
	// We need event interface
	// this would work only inside of a k8s cluster
	k8SClientset, err := k8s.GetK8SClientset()
	if err != nil {
		log.Fatalf("fail to create kubernetes client, error: %s", err)
		return
	}
	eventInter := k8SClientset.CoreV1().Events("current_ns")

	// get the Scheme
	// in our case we should use Scheme that aware of CR
	// if your events are based on default objects you can use runtime.NewScheme()
	scheme, err := k8s.PrepareScheme()
	if err != nil {
		log.Fatalf("fail to prepare kubernetes scheme, error: %s", err)
		return
	}

	logr := logrus.New()

	eventRecorder, err := events.New("baremetal-csi-node", "434aa7b1-8b8a-4ae8-92f9-1cc7e09a9030", eventInter, scheme, logr)
	if err != nil {
		log.Fatalf("fail to create events recorder, error: %s", err)
		return
	}
	// Wait till all events are sent/handled
	defer eventRecorder.Wait()

	// Send event
	drive := new(drivecrd.Drive)
	eventRecorder.Eventf(drive, "Critical", "DriveIsDead", "drive &s is dead", drive.GetName())
}
