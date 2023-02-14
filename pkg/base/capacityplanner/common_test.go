/*
Copyright © 2021 Dell Inc. or its subsidiaries. All Rights Reserved.

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

import "testing"

func TestSubtractLVMMetadataSize(t *testing.T) {
	type args struct {
		size int64
	}
	tests := []struct {
		name string
		args args
		want int64
	}{
		{
			name: "500MiB",
			args: args{
				size: 524288000,
			},
			want: 520093696,
		},
		{
			name: "501MiB",
			args: args{
				size: 525336576,
			},
			want: 524288000,
		},
		{
			name: "502MiB",
			args: args{
				size: 526385152,
			},
			want: 524288000,
		},
		{
			name: "500.5MiB",
			args: args{
				size: 524812288,
			},
			want: 520093696,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SubtractLVMMetadataSize(tt.args.size); got != tt.want {
				t.Errorf("SubtractLVMMetadataSize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAlignSizeMB(t *testing.T) {
	type args struct {
		size int64
	}
	tests := []struct {
		name string
		args args
		want int64
	}{
		{
			name: "500MB",
			args: args{
				size: 500000000,
			},
			want: 499122176,
		},
		{
			name: "10GB",
			args: args{
				size: 10000000000,
			},
			want: 9999220736,
		},
		{
			name: "500MiB",
			args: args{
				size: 524288000,
			},
			want: 524288000,
		},
		{
			name: "10GiB",
			args: args{
				size: 10737418240,
			},
			want: 10737418240,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AlignSizeMB(tt.args.size); got != tt.want {
				t.Errorf("AlignSizeMB() = %v, want %v", got, tt.want)
			}
		})
	}
}
