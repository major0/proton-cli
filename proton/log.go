package proton

import (
	"fmt"
	"log/slog"
)

// Logger defines a leveled logging interface compatible with resty.Logger.
type Logger interface {
	Errorf(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
}

// Errorf logs a formatted message at error level.
func Errorf(format string, args ...interface{}) {
	slog.Error(fmt.Sprintf(format, args...))
}

// Warnf logs a formatted message at warning level.
func Warnf(format string, args ...interface{}) {
	slog.Warn(fmt.Sprintf(format, args...))
}

// Debugf logs a formatted message at debug level.
func Debugf(format string, args ...interface{}) {
	slog.Debug(fmt.Sprintf(format, args...))
}

// Infof logs a formatted message at info level.
func Infof(format string, args ...interface{}) {
	slog.Info(fmt.Sprintf(format, args...))
}
