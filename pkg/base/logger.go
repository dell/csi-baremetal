package base

import (
	"os"

	// TODO: implement own formatter
	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/sirupsen/logrus"
)

// Attempt to init logrus logger with output path passed in the parameter
// If path is incorrect then init logger with stdout
func InitLogger(logPath string, logLevel logrus.Level) (*logrus.Logger, error) {
	logger := logrus.New()
	logger.SetFormatter(&nested.Formatter{
		HideKeys:    true,
		FieldsOrder: []string{"component", "method", "volumeID"},
	})
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
