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

// Package recorder implements a simple library for sending event to k8s
package recorder

import (
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ref "k8s.io/client-go/tools/reference"
)

const (
	maxTriesPerEvent = 12
	defaultSleep     = 10 * time.Second
)

// EventSink is a simple wrapper
type EventSink interface {
	record.EventSink
}

// SimpleRecorder is simple goroutine based recorder for events
type SimpleRecorder struct {
	sink   EventSink
	scheme *runtime.Scheme
	source v1.EventSource
	lg     Logger
	sync.WaitGroup
	// only set time in tests
	fixedTime *time.Time
}

// New is a simple constructor for SimpleRecorder
func New(sink record.EventSink, scheme *runtime.Scheme, source v1.EventSource, lg Logger) *SimpleRecorder {
	return &SimpleRecorder{sink: sink, scheme: scheme, source: source, lg: lg}
}

// makeEvent is helper to build v1.Event according parameters
func (sr *SimpleRecorder) makeEvent(ref *v1.ObjectReference, labels map[string]string, eventtype, reason, message string) *v1.Event {
	now := time.Now()
	if sr.fixedTime != nil {
		now = *sr.fixedTime
	}
	t := metav1.NewTime(now)
	namespace := ref.Namespace
	if namespace == "" {
		if key := os.Getenv("NAMESPACE"); key != "" {
			namespace = os.Getenv("NAMESPACE")
		} else {
			namespace = metav1.NamespaceDefault
		}
	}
	return &v1.Event{
		ObjectMeta: metav1.ObjectMeta{
			// make uniq name
			Name:      fmt.Sprintf("%v.%x", ref.Name, t.UnixNano()),
			Namespace: namespace,
			Labels:    labels,
		},
		InvolvedObject: *ref,
		Reason:         reason,
		Message:        message,
		FirstTimestamp: t,
		LastTimestamp:  t,
		Count:          1,
		Type:           eventtype,
	}
}

func (sr *SimpleRecorder) generateEvent(object runtime.Object, labels map[string]string, eventtype, reason, message string) {
	ref, err := ref.GetReference(sr.scheme, object)
	if err != nil {
		sr.lg.Errorf("Could not construct reference to: '%#v' due to: '%v'. Will not report event: '%v' '%v' '%v'", object, err, eventtype, reason, message)
		return
	}

	event := sr.makeEvent(ref, labels, eventtype, reason, message)
	event.Source = sr.source
	sr.Add(1)
	go func() {
		defer sr.Done()
		// NOTE: events should be a non-blocking operation
		recordToSink(sr.sink, event, defaultSleep, sr.lg)
	}()
}

// recordEvent attempts to write event to a sink. It returns true if the event
// was successfully recorded or discarded, false if it should be retried.
// It's always creates new event.
func recordEvent(sink record.EventSink, event *v1.Event, lg Logger) bool {
	event.ResourceVersion = ""
	_, err := sink.Create(event)
	if err == nil {
		return true
	}

	// If we can't contact the server, then hold everything while we keep trying.
	// Otherwise, something about the event is malformed and we should abandon it.
	switch err.(type) {
	case *restclient.RequestConstructionError:
		// We will construct the request the same next time, so don't keep trying.
		lg.Errorf("Unable to construct event '%#v': '%v' (will not retry!)", event, err)
		return true
	case *errors.StatusError:
		if errors.IsAlreadyExists(err) {
			lg.Infof("Server rejected event '%#v': '%v' (will not retry!)", event, err)
		} else {
			lg.Errorf("Server rejected event '%#v': '%v' (will not retry!)", event, err)
		}
		return true
	case *errors.UnexpectedObjectError:
		// We don't expect this; it implies the server's response didn't match a
		// known pattern. Go ahead and retry.
	default:
		// This case includes actual http transport errors. Go ahead and retry.
	}
	lg.Errorf("Unable to write event: '%v' (may retry after sleeping)", err)
	return false
}

// recordToSink is retriable recordEvent
func recordToSink(sink record.EventSink, event *v1.Event, sleepDuration time.Duration, lg Logger) {
	tries := 0
	for {
		if recordEvent(sink, event, lg) {
			break
		}
		tries++
		if tries >= maxTriesPerEvent {
			lg.Errorf("Unable to write event '%#v' (retry limit exceeded!)", event)
			break
		}
		// Randomize the first sleep so that various clients won't all be
		// synced up if the master goes down.
		if tries == 1 {
			time.Sleep(time.Duration(float64(sleepDuration) * rand.Float64()))
		} else {
			time.Sleep(sleepDuration)
		}
	}
}

// Eventf generate events with for formatted message
func (sr *SimpleRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	sr.generateEvent(object, nil, eventtype, reason, fmt.Sprintf(messageFmt, args...))
}

// LabeledEventf generate events with for formatted message and labels
func (sr *SimpleRecorder) LabeledEventf(object runtime.Object, labels map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
	sr.generateEvent(object, labels, eventtype, reason, fmt.Sprintf(messageFmt, args...))
}

// Logger is hmm... for logging.
type Logger interface {
	Errorf(format string, args ...interface{})
	Infof(format string, args ...interface{})
}

// NoOpLogger is used when no logger is provided
type NoOpLogger struct{}

// Errorf does nothing
func (n NoOpLogger) Errorf(format string, args ...interface{}) {}

// Infof does nothing
func (n NoOpLogger) Infof(format string, args ...interface{}) {}
