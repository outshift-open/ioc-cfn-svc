package logger

import (
	"os"

	"github.com/go-errors/errors"
	"github.com/namsral/flag"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logLevelFlag = flag.String(
		"log_level",
		"info", // default value
		"log level. supported: debug, info, warn, error, dpanic, panic, fatal",
	)

	logEncodingFlag = flag.String(
		"log_encoding",
		"console", // default value
		"log encoding. supported: json, console",
	)

	logLevel = zapcore.InfoLevel
	std      *zap.SugaredLogger
)

func init() {
	// the calling package is responsible for first calling flag.Parse()
	// flag.Parse()

	var err error
	logLevel, err = zapcore.ParseLevel(*logLevelFlag)
	if err != nil {
		panic(err)
	}

	switch *logEncodingFlag {
	//nolint:goconst // allow "json" and "console" strings
	case "json", "console":
	default:
		panic(errors.Errorf("log_encoding %s unsupported", *logEncodingFlag))
	}

	std = newLogger().Sugar().Named("app")
}

// expose functions for std logger
func Debug(args ...interface{})               { std.Debug(args...) }
func Info(args ...interface{})                { std.Info(args...) }
func Warn(args ...interface{})                { std.Warn(args...) }
func Error(args ...interface{})               { std.Error(args...) }
func Fatal(args ...interface{})               { std.Fatal(args...) }
func Debugf(tmpl string, args ...interface{}) { std.Debugf(tmpl, args...) }
func Infof(tmpl string, args ...interface{})  { std.Infof(tmpl, args...) }
func Warnf(tmpl string, args ...interface{})  { std.Warnf(tmpl, args...) }
func Errorf(tmpl string, args ...interface{}) { std.Errorf(tmpl, args...) }
func Fatalf(tmpl string, args ...interface{}) { std.Fatalf(tmpl, args...) }
func Sync() error                             { return std.Sync() }

// ensure zap logger implements this Logger interface
var _ Logger = (*zap.SugaredLogger)(nil)

type Logger interface {
	Debug(...interface{})
	Info(...interface{})
	Warn(...interface{})
	Error(...interface{})
	Fatal(...interface{})
	Debugf(string, ...interface{})
	Infof(string, ...interface{})
	Warnf(string, ...interface{})
	Errorf(string, ...interface{})
	Fatalf(string, ...interface{})
	Sync() error
}

// Default will return the pre initialized sugared logger
func Default() *zap.SugaredLogger {
	return std
}

// SubPkg returns a new logger extended off the default logger. this means that
// you can call Sync() on the default logger to synchronize all
func SubPkg(name string) *zap.SugaredLogger {
	return std.Named(name)
}

func newLogger() *zap.Logger {
	// split logs between high/low priority for logs above configured log_level
	highPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.ErrorLevel && lvl >= logLevel
	})
	lowPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl < zapcore.ErrorLevel && lvl >= logLevel
	})

	// custom configurations
	config := zap.NewProductionEncoderConfig()
	config.EncodeTime = zapcore.ISO8601TimeEncoder

	// high-priority output to stderr, low-priority output to stdout
	// Lock to make writing to stderr/out concurrently safe
	consoleErrors := zapcore.Lock(os.Stderr)
	consoleDebugging := zapcore.Lock(os.Stdout)

	var encoder zapcore.Encoder
	if *logEncodingFlag == "json" {
		encoder = zapcore.NewJSONEncoder(config)
	} else if *logEncodingFlag == "console" {
		encoder = zapcore.NewConsoleEncoder(config)
	}

	core := zapcore.NewTee(
		zapcore.NewCore(encoder, consoleErrors, highPriority),
		zapcore.NewCore(encoder, consoleDebugging, lowPriority),
	)
	return zap.New(core,
		zap.AddCaller(),
		//zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
}

func ErrorWrap(f func() error) {
	err := f()
	if err != nil {
		std.Warnf("error during deferred callback: %s", err)
	}
}
