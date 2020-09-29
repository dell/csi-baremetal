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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/dell/csi-baremetal/pkg/events/mocks"
)

func TestOption_YAMLValidation(t *testing.T) {
	inputRawConfig := `
overrideRules:
  DiskFailed:
    SymptomID: DECKS-1000
    kahm/enabled: true
`
	expectedConfig := Options{LabelsOverride: map[string]map[string]string{
		"DiskFailed": {"SymptomID": "DECKS-1000", "kahm/enabled": "true"},
	}}
	cfg := Options{}
	err := yaml.Unmarshal([]byte(inputRawConfig), &cfg)
	if !assert.NoError(t, err, "not valid yaml") {
		t.FailNow()
	}
	assert.Equal(t, expectedConfig, cfg)
}

func TestNew(t *testing.T) {
	type args struct {
		component string
		node      string
		eventInt  v1core.EventInterface
		scheme    *runtime.Scheme
		opt       Options
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
				opt:       Options{},
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
				opt:       Options{},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.args.component, tt.args.node, tt.args.eventInt, tt.args.scheme, tt.args.opt)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestRecorder_Eventf(t *testing.T) {
	type fields struct {
		eventRecorder  *mocks.EventRecorder
		labelsOverride map[string]map[string]string
		Wait           func()
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
				eventRecorder:  new(mocks.EventRecorder),
				labelsOverride: nil,
				Wait:           func() {},
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
				eventRecorder: new(mocks.EventRecorder),
				labelsOverride: map[string]map[string]string{
					"Stopped": {
						"label": "key",
					},
				},
				Wait: func() {},
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
				eventRecorder:  tt.fields.eventRecorder,
				labelsOverride: tt.fields.labelsOverride,
				Wait:           tt.fields.Wait,
			}
			r.Eventf(tt.args.object, tt.args.eventtype, tt.args.reason, tt.args.messageFmt, tt.args.args...)
			r.Wait()

			tt.fields.eventRecorder.AssertExpectations(t)
		})
	}
}
