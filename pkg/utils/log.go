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

// migrateLogFile moves log files from the old ~/.config/kilo/logs/ directory
// to ~/.config/kilo-docker/logs/. Called once via sync.Once before the first
// log write. Uses direct stderr and file I/O (not utils.Log) to avoid a
// deadlock with getLogFile's sync.Once guard.
func migrateLogFile() {
	oldDir := filepath.Join(constants.GetHomeDir(), ".config", "kilo", "logs")
	newDir := filepath.Join(constants.GetKiloDockerConfigDir(), "logs")
	oldLog := filepath.Join(oldDir, "kilo-docker.log")
	newLog := filepath.Join(newDir, "kilo-docker.log")

	oldDirExists := false
	if info, err := os.Stat(oldDir); err == nil && info.IsDir() {
		oldDirExists = true
	}

	oldLogExists := false
	if info, err := os.Stat(oldLog); err == nil && !info.IsDir() {
		oldLogExists = true
	}

	newLogExists := false
	if _, err := os.Stat(newLog); err == nil {
		newLogExists = true
	}

	if !oldDirExists && !newLogExists {
		_ = os.MkdirAll(newDir, 0o700)
		return
	}

	if !oldLogExists {
		_ = os.MkdirAll(newDir, 0o700)
		return
	}

	if newLogExists {
		fmt.Fprintf(os.Stderr, "[kilo-docker] Log file already exists at new location, renaming old file\n")
		fmt.Fprintf(os.Stderr, "[kilo-docker]   Old: %s → %s.migrated\n", oldLog, oldLog)
		_ = os.MkdirAll(newDir, 0o700)

		w, err := os.OpenFile(oldLog, os.O_APPEND|os.O_WRONLY, 0o600)
		if err == nil {
			timestamp := time.Now().Format("2006-01-02 15:04:05")
			_, _ = fmt.Fprintf(w, "[%s] [host] [LOG] [kilo-docker] Log file migrated, future logs go to ~/.config/kilo-docker/logs/\n", timestamp)
			_ = w.Close()
		}

		if err := os.Rename(oldLog, oldLog+".migrated"); err != nil {
			fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: failed to rename old log file: %v\n", err)
		}

		oldRotated, _ := filepath.Glob(filepath.Join(oldDir, "kilo-docker-*.log.gz"))
		for _, f := range oldRotated {
			dest := filepath.Join(newDir, filepath.Base(f))
			if _, err := os.Stat(dest); os.IsNotExist(err) {
				_ = os.Rename(f, dest)
			}
		}
		return
	}

	fmt.Fprintf(os.Stderr, "[kilo-docker] Migrated log files from %s to %s\n", oldDir, newDir)

	_ = os.MkdirAll(newDir, 0o700)

	if err := os.Rename(oldLog, newLog); err != nil {
		fmt.Fprintf(os.Stderr, "[kilo-docker] Warning: failed to migrate log file: %v\n", err)
	}

	oldRotated, _ := filepath.Glob(filepath.Join(oldDir, "kilo-docker-*.log.gz"))
	for _, f := range oldRotated {
		dest := filepath.Join(newDir, filepath.Base(f))
		if _, err := os.Stat(dest); os.IsNotExist(err) {
			_ = os.Rename(f, dest)
		}
	}

	remaining, _ := filepath.Glob(filepath.Join(oldDir, "*"))
	if len(remaining) == 0 {
		_ = os.Remove(oldDir)
	}
}

func getLogFile() *lumberjack.Logger {
	logFileOnce.Do(func() {
		migrateLogFile()

		logSubDir := filepath.Join(constants.GetKiloDockerConfigDir(), "logs")
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
