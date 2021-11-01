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

package events

import (
	"errors"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/dell/csi-baremetal/pkg/eventing"
	simple "github.com/dell/csi-baremetal/pkg/events/recorder"
)

// EventRecorder knows how to record events on behalf of an EventSource
type EventRecorder interface {
	Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{})
	LabeledEventf(object runtime.Object, labels map[string]string, eventtype, reason, messageFmt string, args ...interface{})
}

// Recorder will serve us as wrapper around EventRecorder
type Recorder struct {
	eventRecorder EventRecorder
	eventManager  *eventing.EventManager
	// Wait is blocking wait operation until all events are processed
	Wait func()
}

// EventInterface is just a local wrapper
type EventInterface interface {
	v1core.EventInterface
}

// Eventf wraps EventRecorder's Eventf method with needed labels replacement
// 'object' is the object this event is about. Event will make a reference to it.
// 'type' of this event, and can anything. Normal, Error, Critical, Epic - use it wisely.
// 'reason' is the reason this event is generated. 'reason' should be short and unique; it
// should be in UpperCamelCase format (starting with a capital letter). "reason" will be used
// to automate handling of events, so imagine people writing switch statements to handle them.
// You want to make that easy. Plus you can add labels based on reason and use for alerting.
// Edit pkg/eventing/eventing.go file to map reason with the related Symptom Code
// 'message' is intended to be human readable.
//
// The resulting event will be created in the same namespace as the reference object.
func (r *Recorder) Eventf(object runtime.Object, event *eventing.EventDescription, messageFmt string, args ...interface{}) {
	var (
		reason   = r.eventManager.GetReason(event)
		severity = r.eventManager.GetSeverity(event)
		labels   = r.eventManager.GetLabels(event)
	)

	if labels != nil {
		r.eventRecorder.LabeledEventf(object, labels, severity, reason, messageFmt, args...)
	} else {
		r.eventRecorder.Eventf(object, severity, reason, messageFmt, args...)
	}
}

// New makes Recorder for a simple usage
// implementation for v1core.EventInterface can be easily found in kubernetes.Clientset.CoreV1().Events("yourNameSpace")
// schema must know about object you will send events about, if use use something built-in try runtime.New
func New(component, nodeName string, eventInt v1core.EventInterface, scheme *runtime.Scheme, logger *logrus.Logger) (*Recorder, error) {
	lg := logger.WithField("componentName", "Events")

	if scheme == nil {
		return nil, errors.New("schema is required")
	}

	// use simple local Recorder for now
	eventRecorder := simple.New(&v1core.EventSinkImpl{Interface: eventInt}, scheme, v1.EventSource{Component: component, Host: nodeName}, lg)
	return &Recorder{
		eventRecorder: eventRecorder,
		eventManager:  &eventing.EventManager{},
		Wait:          eventRecorder.Wait,
	}, nil
}
