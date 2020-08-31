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
	// TODO: should be configured in helm chart AK8S-1260
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
