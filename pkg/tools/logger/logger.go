package logger

import (
	"os"
	"sync"

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

	atomicLevel = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	std         *zap.SugaredLogger

	// packageLevels stores per-package atomic log levels (only for overrides)
	// Key is package name (e.g., "app", "mcp"), value is the atomic level
	packageLevels = make(map[string]*zap.AtomicLevel)
	// registeredPackages tracks all packages that called SubPkg
	registeredPackages = make(map[string]bool)
	packageLevelMu     sync.RWMutex
)

func init() {
	// the calling package is responsible for first calling flag.Parse()
	// flag.Parse()

	parsedLevel, err := zapcore.ParseLevel(*logLevelFlag)
	if err != nil {
		panic(err)
	}
	atomicLevel.SetLevel(parsedLevel)

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

// SubPkg returns a new logger for a specific package.
// By default, it uses ROOT level. Package-specific level can be set via SetPackageLevel().
// The logger's level can be changed at runtime via SetPackageLevel().
func SubPkg(name string) *zap.SugaredLogger {
	// Lock (exclusive) - blocks all other readers and writers
	// Used here because we may write to packageLevels and registeredPackages maps
	packageLevelMu.Lock()
	defer packageLevelMu.Unlock()

	// Track this package as registered
	registeredPackages[name] = true

	// Always create package-specific AtomicLevel if not exists
	// Initialize to ROOT level - SetPackageLevel can change it later
	if _, exists := packageLevels[name]; !exists {
		level := zap.NewAtomicLevelAt(atomicLevel.Level())
		packageLevels[name] = &level
	}

	return newLoggerWithLevel(packageLevels[name]).Sugar().Named(name)
}

func newLogger() *zap.Logger {
	return newLoggerWithLevel(&atomicLevel)
}

func newLoggerWithLevel(level *zap.AtomicLevel) *zap.Logger {
	// Use provided atomicLevel for runtime level changes
	// Split logs: errors to stderr, everything else to stdout
	highPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.ErrorLevel && level.Enabled(lvl)
	})
	lowPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl < zapcore.ErrorLevel && level.Enabled(lvl)
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

// GetLevel returns the current ROOT log level as a string
func GetLevel() string {
	return atomicLevel.Level().String()
}

// IsRegisteredPackage returns true if the package has called SubPkg
func IsRegisteredPackage(name string) bool {
	// RLock (shared) - allows multiple concurrent readers, blocks writers
	// Used here because we only read from registeredPackages map
	packageLevelMu.RLock()
	defer packageLevelMu.RUnlock()
	return registeredPackages[name]
}

// GetPackageLevel returns the log level for a specific package.
// Returns ROOT level if package has no specific level set.
func GetPackageLevel(name string) string {
	if name == "ROOT" || name == "" {
		return atomicLevel.Level().String()
	}
	// RLock allows concurrent reads while blocking writes
	packageLevelMu.RLock()
	defer packageLevelMu.RUnlock()
	if pkgLevel, exists := packageLevels[name]; exists {
		return pkgLevel.Level().String()
	}
	return atomicLevel.Level().String()
}

// GetAllLevels returns a map of all registered package levels including ROOT
// Shows effective level for each package (override if set, otherwise ROOT)
func GetAllLevels() map[string]string {
	// RLock (shared) - allows multiple concurrent readers, blocks writers
	// Used here because we only read from packageLevels and registeredPackages maps
	packageLevelMu.RLock()
	defer packageLevelMu.RUnlock()

	levels := make(map[string]string)
	levels["ROOT"] = atomicLevel.Level().String()

	// Show all registered packages with their effective level
	for name := range registeredPackages {
		if pkgLevel, hasOverride := packageLevels[name]; hasOverride {
			levels[name] = pkgLevel.Level().String()
		} else {
			// No override - uses ROOT level
			levels[name] = atomicLevel.Level().String()
		}
	}
	return levels
}

// SetLevel sets the log level for ROOT or a specific package.
// Use "ROOT" or empty string to set the global level for all loggers.
// Use a package name to set level for that specific package only.
func SetLevel(level string) error {
	return SetPackageLevel("ROOT", level)
}

// SetPackageLevel sets the log level for a specific package or ROOT.
// If moduleName is "ROOT" or empty, sets the global level.
// Otherwise sets the level for the specified package.
func SetPackageLevel(moduleName, level string) error {
	newLevel, err := zapcore.ParseLevel(level)
	if err != nil {
		return err
	}

	if moduleName == "ROOT" || moduleName == "" {
		// AtomicLevel.SetLevel() is thread-safe (uses atomic.Int32 internally)
		// Changes take effect immediately for all loggers using this level
		atomicLevel.SetLevel(newLevel)

		// Also update all package levels to match ROOT
		// Lock (exclusive) - blocks all readers and writers while iterating and updating
		packageLevelMu.Lock()
		for _, pkgLevel := range packageLevels {
			pkgLevel.SetLevel(newLevel)
		}
		packageLevelMu.Unlock()
		return nil
	}

	// Lock (exclusive) - blocks all readers and writers
	// Used here because we may write to packageLevels map
	packageLevelMu.Lock()
	defer packageLevelMu.Unlock()

	// Set package-specific level
	// AtomicLevel.SetLevel() is thread-safe - changes are immediate
	if pkgLevel, exists := packageLevels[moduleName]; exists {
		pkgLevel.SetLevel(newLevel)
	} else {
		// Create new package level if it doesn't exist yet
		level := zap.NewAtomicLevelAt(newLevel)
		packageLevels[moduleName] = &level
	}
	return nil
}
