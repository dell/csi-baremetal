package command

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dell/csi-baremetal/pkg/mocks"
)

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

// ExecutorWithMetrics is a wrapper for CmdExecutor

func TestExecutorWithMetrics_RunCmdWithMetrics(t *testing.T) {
	cmdExec := &mocks.GoMockExecutor{}
	cmdExec.On("RunCmd", "cmd").Return("", "", nil).Times(1)
	exec := NewExecutorWithMetrics(cmdExec)
	stdout, stderr, err := exec.RunCmdWithMetrics("cmd", "cmd", "test")
	assert.Nil(t, err)
	assert.Equal(t, stdout, "")
	assert.Equal(t, stderr, "")
}

func TestExecutorWithMetrics_RunCmdWithAttemptsAndMetrics(t *testing.T) {
	cmdExec := &mocks.GoMockExecutor{}
	cmdExec.On("RunCmdWithAttempts", "cmd", 1, time.Second).Return("", "", nil).Times(1)
	exec := NewExecutorWithMetrics(cmdExec)
	stdout, stderr, err := exec.RunCmdWithAttemptsAndMetrics("cmd", 1, time.Second, "cmd", "test")
	assert.Nil(t, err)
	assert.Equal(t, stdout, "")
	assert.Equal(t, stderr, "")
}
