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

package util

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/dell/csi-baremetal/pkg/base"
)

// AddCommonFields read common fields from ctx and add them to logger
func AddCommonFields(ctx context.Context, logger *logrus.Entry, method string) *logrus.Entry {
	return logger.WithFields(logrus.Fields{
		"volumeID": ctx.Value(base.RequestUUID),
		"method":   method,
	})
}
