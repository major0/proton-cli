package proton

import (
	"fmt"
	"log/slog"
)

// Wrapper around slog.Logger to make it usable with resty.Logger
type Logger interface {
	Errorf(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
}

func Errorf(format string, args ...interface{}) {
	slog.Error(fmt.Sprintf(format, args...))
}

func Warnf(format string, args ...interface{}) {
	slog.Warn(fmt.Sprintf(format, args...))
}

func Debugf(format string, args ...interface{}) {
	slog.Debug(fmt.Sprintf(format, args...))
}

func Infof(format string, args ...interface{}) {
	slog.Info(fmt.Sprintf(format, args...))
}
