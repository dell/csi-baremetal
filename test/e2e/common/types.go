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

package common

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	apiV1 "github.com/dell/csi-baremetal/api/v1"
)

var (
	DriveGVR = schema.GroupVersionResource{
		Group:    apiV1.CSICRsGroupVersion,
		Version:  apiV1.Version,
		Resource: "drives",
	}

	ACGVR = schema.GroupVersionResource{
		Group:    apiV1.CSICRsGroupVersion,
		Version:  apiV1.Version,
		Resource: "availablecapacities",
	}

	ACRGVR = schema.GroupVersionResource{
		Group:    apiV1.CSICRsGroupVersion,
		Version:  apiV1.Version,
		Resource: "availablecapacityreservations",
	}

	VolumeGVR = schema.GroupVersionResource{
		Group:    apiV1.CSICRsGroupVersion,
		Version:  apiV1.Version,
		Resource: "volumes",
	}

	LVGGVR = schema.GroupVersionResource{
		Group:    apiV1.CSICRsGroupVersion,
		Version:  apiV1.Version,
		Resource: "logicalvolumegroups",
	}

	CsibmnodeGVR = schema.GroupVersionResource{
		Group:    apiV1.CSICRsGroupVersion,
		Version:  apiV1.Version,
		Resource: "nodes",
	}
)
