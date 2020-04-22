package base

import (
	"os"

	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/sirupsen/logrus"
)

// InitLogger attempts to init logrus logger with output path passed in the parameter
// If path is incorrect or "" then init logger with stdout
// Receives logPath which is the file to write logs and logrus.Level which is level of logging (For example DEBUG, INFO)
// Returns created logrus.Logger or error if something went wrong
func InitLogger(logPath string, logLevel logrus.Level) (*logrus.Logger, error) {
	logger := logrus.New()
	if os.Getenv("LOG_FORMAT") == "text" {
		logger.SetFormatter(&nested.Formatter{
			HideKeys:    true,
			NoColors:    true,
			FieldsOrder: []string{"component", "method", "volumeID"},
		})
	} else {
		logger.SetFormatter(&logrus.JSONFormatter{})
	}
	logger.SetLevel(logLevel)
	if logPath != "" {
		file, err := os.Create(logPath)
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
