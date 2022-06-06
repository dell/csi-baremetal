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

package recorder

import (
	"errors"
	"fmt"
	"testing"
	"time"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	ref "k8s.io/client-go/tools/reference"

	"github.com/dell/csi-baremetal/pkg/events/mocks"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	apiV1 "github.com/dell/csi-baremetal/api/v1"
)

func TestSimpleRecorder_Eventf(t *testing.T) {
	fixedtime := time.Now()
	metaFixedtime := metav1.NewTime(fixedtime)
	testPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			SelfLink:  "/api/v1/namespaces/baz/pods/foo",
			Name:      "foo",
			Namespace: "baz",
			UID:       "bar",
		},
	}

	testAPIDrive := api.Drive{
		UUID:         "drive1-uuid",
		SerialNumber: "drive1-sn",
		NodeId:       "node1",
		Health:       apiV1.HealthGood,
		Type:         apiV1.DriveTypeHDD,
		Size:         1024 * 1024,
		Status:       apiV1.DriveStatusOnline,
	}
	testDriveCR := &drivecrd.Drive{
		TypeMeta:   metav1.TypeMeta{Kind: "Drive", APIVersion: apiV1.APIV1Version},
		ObjectMeta: metav1.ObjectMeta{Name: testAPIDrive.UUID},
		Spec:       testAPIDrive,
	}

	testRef, err := ref.GetPartialReference(scheme.Scheme, testPod, "spec.containers[2]")
	if err != nil {
		t.Fatal(err)
	}

	type fields struct {
		sink   *mocks.EventSink
		scheme *runtime.Scheme
		source v1.EventSource
		lg     Logger
	}
	type args struct {
		object     runtime.Object
		eventtype  string
		reason     string
		messageFmt string
		args       []interface{}
	}
	tests := []struct {
		name          string
		fields        fields
		args          args
		expectedEvent *v1.Event
	}{
		{
			name: "simple flow",
			fields: fields{
				sink:   new(mocks.EventSink),
				scheme: runtime.NewScheme(),
				source: v1.EventSource{
					Component: "eventTest",
				},
				lg: logrus.New(),
			},
			args: args{
				object:     testRef,
				eventtype:  "Awesome",
				reason:     "Started",
				messageFmt: "some verbose message: %s",
				args:       []interface{}{"this is argument"},
			},
			expectedEvent: &v1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("foo.%x", fixedtime.UnixNano()),
					Namespace: "baz",
				},
				InvolvedObject: v1.ObjectReference{
					Kind:       "Pod",
					Name:       "foo",
					Namespace:  "baz",
					UID:        "bar",
					APIVersion: "v1",
					FieldPath:  "spec.containers[2]",
				},
				FirstTimestamp: metaFixedtime,
				LastTimestamp:  metaFixedtime,
				Reason:         "Started",
				Message:        "some verbose message: this is argument",
				Source:         v1.EventSource{Component: "eventTest"},
				Count:          1,
				Type:           "Awesome",
			},
		},
		{
			name: "non default ns event for drive",
			fields: fields{
				sink:   new(mocks.EventSink),
				scheme: runtime.NewScheme(),
				source: v1.EventSource{
					Component: "eventTest",
				},
				lg: logrus.New(),
			},
			args: args{
				object:     testDriveCR,
				eventtype:  "Health",
				reason:     "Failed",
				messageFmt: "some verbose message: %s",
				args:       []interface{}{"this is argument"},
			},
			expectedEvent: &v1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("drive1-uuid.%x", fixedtime.UnixNano()),
					Namespace: "atlantic",
				},
				InvolvedObject: v1.ObjectReference{
					Kind:       "Drive",
					Name:       "drive1-uuid",
					Namespace:  "atlantic",
					APIVersion: "csi-baremetal.dell.com/v1",
				},
				FirstTimestamp: metaFixedtime,
				LastTimestamp:  metaFixedtime,
				Reason:         "Failed",
				Message:        "some verbose message: this is argument",
				Source:         v1.EventSource{Component: "eventTest"},
				Count:          1,
				Type:           "Health",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.fields.sink.On("Create", tt.expectedEvent).Return(tt.expectedEvent, nil)
			sr := New(tt.fields.sink, tt.fields.scheme, tt.fields.source, tt.fields.lg)
			sr.fixedTime = &fixedtime

			if tt.args.object.GetObjectKind().GroupVersionKind().Kind == "Drive" {
				os.Setenv("NAMESPACE","atlantic")
				t.Log("csi is installed in namespace: atlantic")
			}
			sr.Eventf(tt.args.object, tt.args.eventtype, tt.args.reason, tt.args.messageFmt, tt.args.args...)
			sr.Wait()

			tt.fields.sink.AssertExpectations(t)
		})
	}
}

func Test_recordToSink(t *testing.T) {
	type args struct {
		sink          *mocks.EventSink
		event         *v1.Event
		sleepDuration time.Duration
		lg            Logger
	}
	tests := []struct {
		name       string
		args       args
		sinkError  error
		callsCount int
	}{
		{
			name: "Happy path",
			args: args{
				sink:          new(mocks.EventSink),
				event:         &v1.Event{},
				sleepDuration: 0,
				lg:            &NoOpLogger{},
			},
			sinkError:  nil,
			callsCount: 1,
		},
		{
			name: "Simple error should be max retried",
			args: args{
				sink:          new(mocks.EventSink),
				event:         &v1.Event{},
				sleepDuration: 0,
				lg:            &NoOpLogger{},
			},
			sinkError:  errors.New("some unknown error"),
			callsCount: maxTriesPerEvent,
		},
		{
			name: "Status error shouldn't be retried",
			args: args{
				sink:          new(mocks.EventSink),
				event:         &v1.Event{},
				sleepDuration: 0,
				lg:            &NoOpLogger{},
			},
			sinkError:  &k8serrors.StatusError{},
			callsCount: 1,
		},
		{
			name: "UnexpectedObject error should be max retried",
			args: args{
				sink:          new(mocks.EventSink),
				event:         &v1.Event{},
				sleepDuration: 0,
				lg:            &NoOpLogger{},
			},
			sinkError:  &k8serrors.UnexpectedObjectError{},
			callsCount: maxTriesPerEvent,
		},
		{
			name: "RequestConstruction error shouldn't be retried",
			args: args{
				sink:          new(mocks.EventSink),
				event:         &v1.Event{},
				sleepDuration: 0,
				lg:            &NoOpLogger{},
			},
			sinkError:  &restclient.RequestConstructionError{},
			callsCount: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// setup mock
			tt.args.sink.On("Create", mock.Anything).Return(nil, tt.sinkError)

			recordToSink(tt.args.sink, tt.args.event, tt.args.sleepDuration, tt.args.lg)

			// verify
			tt.args.sink.AssertExpectations(t)
			tt.args.sink.AssertNumberOfCalls(t, "Create", tt.callsCount)
		})
	}
}

