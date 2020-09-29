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

package base

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitLoggerStdOut(t *testing.T) {
	logger, err := InitLogger("", InfoLevel)
	if err != nil {
		t.Errorf("Logger initialized with error: %s", err.Error())
	}

	assert.Equal(t, logger.Out, os.Stdout, "Logger output was't set correctly")
}

func TestInitLoggerCorrectPath(t *testing.T) {
	logPath := "/tmp/logs.log"
	logger, err := InitLogger(logPath, InfoLevel)
	if err != nil {
		t.Errorf("Logger initialized with error: %s", err.Error())
	}

	outputFile, ok := logger.Out.(*os.File)

	assert.True(t, ok, "Can't convert logger output to the file")

	assert.Equal(t, outputFile.Name(), logPath, "Logger output was't set correctly")
}

func TestInitLoggerWrongPath(t *testing.T) {
	logPath := "////"
	logger, err := InitLogger(logPath, InfoLevel)
	if err == nil {
		t.Errorf("Logger should be initialized with an error")
	}

	assert.Equal(t, logger.Out, os.Stdout, "Logger's defalut output should be set to the stdout")
}
