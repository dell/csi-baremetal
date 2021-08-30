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

package mocks

import (
	"sync"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/dell/csi-baremetal/pkg/eventing"
)

type eventRecorderCalls struct {
	Object     runtime.Object
	Event      *eventing.EventDescription
	MessageFmt string
	Args       []interface{}
}

// NoOpRecorder is blank implementation of event recorder interface which stores calls to the interface methods
type NoOpRecorder struct {
	Calls []eventRecorderCalls
	m     sync.Mutex
}

// Eventf do nothing
func (n *NoOpRecorder) Eventf(object runtime.Object, event *eventing.EventDescription, messageFmt string, args ...interface{}) {
	c := eventRecorderCalls{
		Object:     object,
		Event:      event,
		MessageFmt: messageFmt,
		Args:       args,
	}
	n.m.Lock()
	n.Calls = append(n.Calls, c)
	n.m.Unlock()
}
