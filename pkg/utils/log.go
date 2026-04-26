// Package utils contains shared helpers used by the host and entrypoint CLIs.
package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mbabic84/kilo-docker/pkg/constants"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	logFile     *lumberjack.Logger
	logFileOnce sync.Once
	logMutex    sync.Mutex
)

type LogOpt func(*bool)

func WithOutput() LogOpt {
	return func(b *bool) { *b = true }
}

func getLogFile() *lumberjack.Logger {
	logFileOnce.Do(func() {
		logDir := constants.GetKiloConfigDir()
		logSubDir := filepath.Join(logDir, "logs")
		if err := os.MkdirAll(logSubDir, 0o700); err != nil {
			return
		}
		logFile = &lumberjack.Logger{
			Filename:   filepath.Join(logSubDir, "kilo-docker.log"),
			MaxSize:    10,
			MaxBackups: 5,
			MaxAge:     30,
			Compress:   true,
		}
	})
	return logFile
}

func logToFile(format string, args ...interface{}) {
	f := getLogFile()
	if f == nil {
		return
	}
	logMutex.Lock()
	defer logMutex.Unlock()
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, args...)
	msg = strings.TrimRight(msg, "\n")

	instanceID := os.Getenv("KILO_CONTAINER_NAME")
	if instanceID == "" {
		instanceID = "host"
	}
	_, _ = fmt.Fprintf(f, "[%s] [%s] %s\n", timestamp, instanceID, msg)
}

func Log(format string, args ...interface{}) {
	output, logArgs := splitLogArgs(args...)

	var msg string
	if len(logArgs) > 0 {
		msg = fmt.Sprintf(format, logArgs...)
	} else {
		msg = format
	}
	if output {
		fmt.Fprint(os.Stderr, msg)
	}
	logToFile("[LOG] "+format, logArgs...)
}

func LogError(format string, args ...interface{}) {
	logWithPrefix("Error: ", "[ERROR] ", format, args...)
}

func LogWarn(format string, args ...interface{}) {
	logWithPrefix("Warning: ", "[WARN] ", format, args...)
}

func splitLogArgs(args ...interface{}) (bool, []interface{}) {
	output := false
	logArgs := make([]interface{}, 0, len(args))
	for _, arg := range args {
		if opt, ok := arg.(LogOpt); ok {
			opt(&output)
		} else {
			logArgs = append(logArgs, arg)
		}
	}
	return output, logArgs
}

func logWithPrefix(userPrefix, filePrefix, format string, args ...interface{}) {
	output, logArgs := splitLogArgs(args...)

	var msg string
	if len(logArgs) > 0 {
		msg = fmt.Sprintf(userPrefix+format, logArgs...)
	} else {
		msg = fmt.Sprintf("%s%s", userPrefix, format)
	}
	if output {
		fmt.Fprint(os.Stderr, msg)
	}
	logToFile(filePrefix+format, logArgs...)
}
