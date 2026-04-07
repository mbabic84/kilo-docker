package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mbabic84/kilo-docker/pkg/constants"
)

var (
	logFile     *os.File
	logFileOnce sync.Once
	logMutex    sync.Mutex
)

type LogOpt func(*bool)

func WithOutput() LogOpt {
	return func(b *bool) { *b = true }
}

// getLogFile opens the log file for writing, creating it if necessary.
// The log file is stored in ~/.config/kilo/kilo-docker.log to persist across container recreations.
func getLogFile() *os.File {
	logFileOnce.Do(func() {
		logDir := constants.GetKiloConfigDir()
		if err := os.MkdirAll(logDir, 0o755); err != nil {
			return
		}
		logPath := filepath.Join(logDir, "kilo-docker.log")
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return
		}
		logFile = f
	})
	return logFile
}

// logToFile writes a message to the log file with a timestamp.
func logToFile(format string, args ...interface{}) {
	f := getLogFile()
	if f == nil {
		return
	}
	logMutex.Lock()
	defer logMutex.Unlock()
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintf(f, "[%s] %s\n", timestamp, msg)
}

// Log prints a message to file, and optionally to stderr.
// By default, logs to file only. Use WithOutput() to also write to stderr.
func Log(format string, args ...interface{}) {
	output := false
	logArgs := make([]interface{}, 0, len(args))
	for _, arg := range args {
		if opt, ok := arg.(LogOpt); ok {
			opt(&output)
		} else {
			logArgs = append(logArgs, arg)
		}
	}

	var msg string
	if len(logArgs) > 0 {
		msg = fmt.Sprintf("[kilo-docker] "+format, logArgs...)
	} else {
		msg = fmt.Sprintf("[kilo-docker] %s", format)
	}
	if output {
		fmt.Fprint(os.Stderr, msg)
	}
	logToFile("[LOG] "+format, logArgs...)
}

// LogError prints an error message to stderr with [kilo-docker] prefix and logs to file.
func LogError(format string, args ...interface{}) {
	var msg string
	if len(args) > 0 {
		msg = fmt.Sprintf("[kilo-docker] Error: "+format, args...)
	} else {
		msg = fmt.Sprintf("[kilo-docker] Error: %s", format)
	}
	fmt.Fprint(os.Stderr, msg)
	logToFile("[ERROR] "+format, args...)
}

// LogWarn prints a warning message to stderr with [kilo-docker] prefix and logs to file.
func LogWarn(format string, args ...interface{}) {
	var msg string
	if len(args) > 0 {
		msg = fmt.Sprintf("[kilo-docker] Warning: "+format, args...)
	} else {
		msg = fmt.Sprintf("[kilo-docker] Warning: %s", format)
	}
	fmt.Fprint(os.Stderr, msg)
	logToFile("[WARN] "+format, args...)
}
