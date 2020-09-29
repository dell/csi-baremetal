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
	"strings"

	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/sirupsen/logrus"
)

const (
	//DebugLevel represents debug level for logger
	DebugLevel = "debug"
	//TraceLevel represents trace level for logger
	TraceLevel = "trace"
	//InfoLevel represents info level for logger
	InfoLevel = "info"
)

// InitLogger attempts to init logrus logger with output path passed in the parameter
// If path is incorrect or "" then init logger with stdout
// Receives logPath which is the file to write logs and logrus.Level which is level of logging (For example DEBUG, INFO)
// Returns created logrus.Logger or error if something went wrong
func InitLogger(logPath string, logLevel string) (*logrus.Logger, error) {
	logger := logrus.New()
	// TODO: should be configured in helm chart https://github.com/dell/csi-baremetal/issues/83
	if os.Getenv("LOG_FORMAT") == "text" {
		logger.SetFormatter(&nested.Formatter{
			HideKeys:    true,
			NoColors:    true,
			FieldsOrder: []string{"component", "method", "volumeID"},
		})
	} else {
		logger.SetFormatter(&logrus.JSONFormatter{})
	}

	var level logrus.Level
	// set log level
	switch strings.ToLower(logLevel) {
	case InfoLevel:
		level = logrus.InfoLevel
	case DebugLevel:
		level = logrus.DebugLevel
	case TraceLevel:
		level = logrus.TraceLevel
	default:
		level = logrus.InfoLevel
	}

	logger.SetLevel(level)

	// set output
	if logPath != "" {
		file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			logger.SetOutput(os.Stdout)
			return logger, err
		}
		logger.SetOutput(file)
		return logger, nil
	}
	logger.SetOutput(os.Stdout)

	return logger, nil
}
