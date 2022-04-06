package logger

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/dell/csi-baremetal/pkg/base"
	"github.com/sirupsen/logrus"
)

// FunctionKey holds the function field
const FunctionKey = "function"

// FileKey holds the file field
const FileKey = "file"

const (
	logrusStackJump          = 4
	logrusFieldlessStackJump = 6
)

// RuntimeFormatter decorates log entries with function name and package name (optional) and line number (optional)
type RuntimeFormatter struct {
	ChildFormatter logrus.Formatter
	MaxLevel       logrus.Level
}

// Format the current log entry by adding the function name and line number of the caller.
func (f *RuntimeFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	data := logrus.Fields{}
	if f.MaxLevel >= entry.Level {
		function, file, line := f.getCurrentPosition(entry)
		packageEnd := strings.LastIndex(function, ".")
		functionName := function[packageEnd+1:]

		// setting function name
		data[FunctionKey] = functionName
		// setting modified filepath: removing base project path and redundant delimiter
		adaptedFilepath := strings.TrimPrefix(file, base.ProjectPath)
		if adaptedFilepath != file { // if current file's path is a subdirectory of passed project path - then trim prefix delimiter
			adaptedFilepath = strings.TrimPrefix(adaptedFilepath, "/")
		}
		data[FileKey] = adaptedFilepath + ":" + line
	}
	for k, v := range entry.Data {
		data[k] = v
	}

	entry.Data = data

	return f.ChildFormatter.Format(entry)
}

func (f *RuntimeFormatter) getCurrentPosition(entry *logrus.Entry) (string, string, string) {
	skip := logrusStackJump
	if len(entry.Data) == 0 {
		skip = logrusFieldlessStackJump
	}
start:
	pc, file, line, _ := runtime.Caller(skip)
	lineNumber := fmt.Sprintf("%d", line)
	function := runtime.FuncForPC(pc).Name()
	if strings.LastIndex(function, "sirupsen/logrus.") != -1 {
		skip++
		goto start
	}
	return function, file, lineNumber
}
