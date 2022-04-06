// SPDX-License-Identifier: AGPL-3.0-only
// Provenance-includes-location: https://github.com/cortexproject/cortex/blob/master/pkg/util/log/log.go
// Provenance-includes-license: Apache-2.0
// Provenance-includes-copyright: The Cortex Authors.

package log

import (
	"fmt"
	"os"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/server"
)

var (
	// Logger is a shared go-kit logger.
	// TODO: Change all components to take a non-global logger via their constructors.
	// Prefer accepting a non-global logger as an argument.
	Logger = log.NewNopLogger()
)

// InitLogger initialises the global gokit logger (util_log.Logger) and overrides the
// default logger for the server.
func InitLogger(cfg *server.Config) {
	l := NewDefaultLogger(cfg.LogLevel, cfg.LogFormat)
	// when using util_log.Logger, skip 3 stack frames.
	Logger = log.With(l, "caller", log.Caller(3))

	// cfg.Log wraps log function, skip 4 stack frames to get caller information.
	// this works in go 1.12, but doesn't work in versions earlier.
	// it will always shows the wrapper function generated by compiler
	// marked <autogenerated> in old versions.
	cfg.Log = logging.GoKit(log.With(l, "caller", log.Caller(4)))
}

// NewDefaultLogger creates a new gokit logger with the configured level and format
func NewDefaultLogger(l logging.Level, format logging.Format) log.Logger {
	var logger log.Logger
	if format.String() == "json" {
		logger = log.NewJSONLogger(log.NewSyncWriter(os.Stderr))
	} else {
		logger = log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	}

	// return a Logger without caller information, shouldn't use directly
	return log.With(level.NewFilter(logger, l.Gokit), "ts", log.DefaultTimestampUTC)
}

// CheckFatal prints an error and exits with error code 1 if err is non-nil
func CheckFatal(location string, err error) {
	if err != nil {
		logger := level.Error(Logger)
		if location != "" {
			logger = log.With(logger, "msg", "error "+location)
		}
		// %+v gets the stack trace from errors using github.com/pkg/errors
		_ = logger.Log("err", fmt.Sprintf("%+v", err))
		os.Exit(1)
	}
}

// Error logs an error and panics if the log call itself had an error.
func Error(logger log.Logger, keyvals ...interface{}) {
	if err := level.Error(logger).Log(keyvals); err != nil {
		panic(fmt.Sprintf("error writing to log: %v", err))
	}
}

// Warn logs a warning and does nothing if the log call itself had an error.
func Warn(logger log.Logger, keyvals ...interface{}) {
	_ = level.Warn(logger).Log(keyvals)
}

// Info logs an info and does nothing if the log call itself had an error.
func Info(logger log.Logger, keyvals ...interface{}) {
	_ = level.Info(logger).Log(keyvals)
}
