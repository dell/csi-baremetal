// Copyright © 2019 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// This software contains the intellectual property of Dell Inc.
// or is licensed to Dell Inc. from third parties. Use of this software
// and the intellectual property contained therein is expressly limited to the
// terms and conditions of the License Agreement under which it is provided by or
// on behalf of Dell Inc. or its subsidiaries.

package util

import (
	"fmt"
	"os"
	"path"
	"runtime"

	"github.com/sirupsen/logrus"
)

const (
	logFolder = "/var/log"
	logFile   = logFolder + "/csi.log"
)

var logger = CreateLogger("util")

// CreateLogger is a wrapper for logrus with adding package field
func CreateLogger(packageName string) *logrus.Entry {
	if os.Getenv("LOG_RECEIVER") == "Dev" {
		logrus.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:    true,
			CallerPrettyfier: callerPrettyfier,
		})
		logrus.Info("log receiver is Dev mode")
	} else {
		logrus.SetFormatter(&logrus.JSONFormatter{
			CallerPrettyfier: callerPrettyfier,
		})
		logrus.Info("log receiver is Elasticsearch")
	}

	if os.Getenv("LOG_DIRECTION") == "file" {
		logrus.Infof("Log direction is file")
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.ModePerm)
		if err != nil {
			logrus.Fatalf("Failed to open log file %s for output: %s", logFile, err)
		}

		logrus.SetOutput(f)
		logrus.RegisterExitHandler(func() {
			if f == nil {
				return
			}

			err := f.Close()
			if err != nil {
				logrus.Fatalf("Failed to close log file %s for output: %s", logFile, err)
			}
		})
	}

	logrus.SetReportCaller(true)

	if packageName != "" {
		return logrus.WithField("package", packageName)
	}

	return logrus.WithField("package", nil)
}

func callerPrettyfier(f *runtime.Frame) (function, file string) {
	return fmt.Sprintf("%s() ", path.Base(f.Function)), fmt.Sprintf("%s:%d", path.Base(f.File), f.Line)
}
