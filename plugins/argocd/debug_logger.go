package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// DebugLogger provides logging functionality for debugging ArgoCD API calls
type DebugLogger struct {
	file    *os.File
	enabled bool
}

var debugLogger *DebugLogger

// InitDebugLogger initializes the debug logger
func InitDebugLogger() error {
	// Create logs directory if it doesn't exist
	err := os.MkdirAll("logs", 0755)
	if err != nil {
		return err
	}

	// Create log file with timestamp
	timestamp := time.Now().Format("20060102-150405")
	logPath := filepath.Join("logs", fmt.Sprintf("argocd-debug-%s.log", timestamp))

	file, err := os.Create(logPath)
	if err != nil {
		return err
	}

	debugLogger = &DebugLogger{
		file:    file,
		enabled: true,
	}

	debugLogger.Log("Debug logger initialized: %s", logPath)
	return nil
}

// Close closes the debug logger file
func (l *DebugLogger) Close() {
	if l.file != nil {
		l.Log("Debug logger closed")
		l.file.Close()
		l.file = nil
	}
}

// Log writes a log message to the debug log file
func (l *DebugLogger) Log(format string, args ...interface{}) {
	if !l.enabled || l.file == nil {
		return
	}

	// Get caller info
	_, file, line, ok := runtime.Caller(1)
	callerInfo := ""
	if ok {
		callerInfo = fmt.Sprintf("%s:%d", filepath.Base(file), line)
	}

	// Format with timestamp and caller info
	timestamp := time.Now().Format("15:04:05.000")
	message := fmt.Sprintf(format, args...)

	fmt.Fprintf(l.file, "[%s] [%s] %s\n", timestamp, callerInfo, message)
	l.file.Sync()
}

// Enable enables or disables logging
func (l *DebugLogger) Enable(enabled bool) {
	l.enabled = enabled
	if enabled {
		l.Log("Debug logging enabled")
	} else {
		l.Log("Debug logging disabled")
	}
}

// GetLogger returns the global debug logger instance
func GetLogger() *DebugLogger {
	if debugLogger == nil {
		// If logger isn't initialized, try to initialize it
		err := InitDebugLogger()
		if err != nil {
			// If we can't initialize logger, create a dummy logger that does nothing
			debugLogger = &DebugLogger{
				file:    nil,
				enabled: false,
			}
		}
	}
	return debugLogger
}

// Debug is a shorthand for GetLogger().Log
func Debug(format string, args ...interface{}) {
	GetLogger().Log(format, args...)
}
