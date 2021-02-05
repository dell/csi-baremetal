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
package command

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/dell/csi-baremetal/pkg/metrics/common"
)

// ExecutorWithMetrics is a wrapper for CmdExecutor
type ExecutorWithMetrics struct {
	CmdExecutor
}

// NewExecutorWithMetrics is a constructor for ExecutorWithMetrics
func NewExecutorWithMetrics(exec CmdExecutor) *ExecutorWithMetrics {
	return &ExecutorWithMetrics{CmdExecutor: exec}
}

// RunCmdWithMetrics runs RunCmd function with SystemCMDDuration metrics
// Receive cmd interface for RunCmd, name of cmd and called method name for metric labels
// Returns stdout, stderr and error in case of failed execution
func (e *ExecutorWithMetrics) RunCmdWithMetrics(cmd interface{}, name, method string) (string, string, error) {
	defer common.SystemCMDDuration.EvaluateDuration(prometheus.Labels{
		"name":   name,
		"method": method})()
	return e.RunCmd(cmd)
}

// RunCmdWithAttemptsAndMetrics runs RunCmdWithAttempts function with SystemCMDDuration metrics
// Receive cmd interface, number of attempts and timeout for RunCmdWithAttempts,
// name of cmd and called method name for metric labels
// Returns stdout, stderr and error in case of failed execution
func (e *ExecutorWithMetrics) RunCmdWithAttemptsAndMetrics(cmd interface{},
	attempts int,
	timeout time.Duration,
	name, method string) (string, string, error) {
	defer common.SystemCMDDuration.EvaluateDuration(prometheus.Labels{
		"name":   name,
		"method": method})()
	return e.RunCmdWithAttempts(cmd, attempts, timeout)
}
