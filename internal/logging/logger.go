package logging

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

var Logger *logrus.Logger

func init() {
	Logger = logrus.New()

	// Set log level based on environment
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "error" // Default to error level to reduce verbosity
	}

	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		level = logrus.ErrorLevel
	}
	Logger.SetLevel(level)

	// Custom formatter with colors and location info
	Logger.SetFormatter(&CustomFormatter{})

	// Set output to stdout
	Logger.SetOutput(os.Stdout)

	// Disable verbose exit behavior that dumps registers
	Logger.ExitFunc = func(int) {}
}

// CustomFormatter extends logrus.TextFormatter to include location info
type CustomFormatter struct {
	logrus.TextFormatter
}

// Format implements logrus.Formatter
func (f *CustomFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	// Add location info
	if pc, file, line, ok := runtime.Caller(8); ok {
		funcName := runtime.FuncForPC(pc).Name()
		// Clean up the function name
		if idx := strings.LastIndex(funcName, "/"); idx != -1 {
			funcName = funcName[idx+1:]
		}
		if idx := strings.LastIndex(funcName, "."); idx != -1 {
			funcName = funcName[idx+1:]
		}

		// Get just the filename, not full path
		filename := filepath.Base(file)

		entry.Data["file"] = filename
		entry.Data["line"] = line
		entry.Data["func"] = funcName
	}

	// Set TextFormatter options
	f.TextFormatter.ForceColors = true
	f.TextFormatter.FullTimestamp = true

	return f.TextFormatter.Format(entry)
}

// GetLogger returns the configured logger instance
func GetLogger() *logrus.Logger {
	return Logger
}
