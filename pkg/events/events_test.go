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
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/dell/csi-baremetal/pkg/events/mocks"
)

func TestNew(t *testing.T) {
	type args struct {
		component string
		node      string
		eventInt  v1core.EventInterface
		scheme    *runtime.Scheme
		log       *logrus.Logger
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "No Scheme should return error",
			args: args{
				component: "csi-component",
				node:      "abc",
				eventInt:  new(mocks.EventInterface),
				scheme:    nil,
				log:       logrus.New(),
			},
			wantErr: true,
		},
		{
			name: "Happy path way",
			args: args{
				component: "csi-component",
				node:      "abc",
				eventInt:  new(mocks.EventInterface),
				scheme:    runtime.NewScheme(),
				log:       logrus.New(),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.args.component, tt.args.node, tt.args.eventInt, tt.args.scheme, tt.args.log)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestRecorder_Eventf(t *testing.T) {
	type fields struct {
		eventRecorder      *mocks.EventRecorder
		reasonSymptomCodes map[string]string
		Wait               func()
	}
	type args struct {
		object     runtime.Object
		eventtype  string
		reason     string
		messageFmt string
		args       []interface{}
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		funcCalled string
	}{
		{
			name:       "Simple event",
			funcCalled: "Eventf",
			fields: fields{
				eventRecorder:      new(mocks.EventRecorder),
				reasonSymptomCodes: nil,
				Wait:               func() {},
			},
			args: args{
				object:     &v1.Pod{},
				eventtype:  "Normal",
				reason:     "Stopped",
				messageFmt: "This is the event %v",
				args:       []interface{}{1},
			},
		},
		{
			name:       "Labels check",
			funcCalled: "LabeledEventf",
			fields: fields{
				eventRecorder:      new(mocks.EventRecorder),
				reasonSymptomCodes: map[string]string{"Stopped": "CSI-00"},
				Wait:               func() {},
			},
			args: args{
				object:     &v1.Pod{},
				eventtype:  "Normal",
				reason:     "Stopped",
				messageFmt: "This is the event %v",
				args:       []interface{}{1},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//setup mocks
			tt.fields.eventRecorder.On(tt.funcCalled, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
			r := &Recorder{
				eventRecorder:      tt.fields.eventRecorder,
				reasonSymptomCodes: tt.fields.reasonSymptomCodes,
				Wait:               tt.fields.Wait,
			}
			r.Eventf(tt.args.object, tt.args.eventtype, tt.args.reason, tt.args.messageFmt, tt.args.args...)
			r.Wait()

			tt.fields.eventRecorder.AssertExpectations(t)
		})
	}
}
