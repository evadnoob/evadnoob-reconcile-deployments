package logging

import (
	"fmt"

	"go.uber.org/zap"
)

// New setup a configured logger.
// uses development logger for now, can be changed to production logger later.
// Also logs to a file so the logs can be saved and shared if needed, as well
// as to keep stdout clean and provide useful information.
func New(applicationName string, quiet bool) *zap.SugaredLogger {
	cfg := zap.NewDevelopmentConfig()
	if quiet {
		cfg.OutputPaths = []string{
			fmt.Sprintf("%s.log", applicationName),
		}
		cfg.ErrorOutputPaths = []string{
			fmt.Sprintf("%s.log", applicationName),
		}
	}
	logger, _ := cfg.Build(zap.WithCaller(true))
	sugar := logger.Sugar()
	return sugar
}
