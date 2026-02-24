package pluginapi

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Logger writes timestamped log lines to a file under ~/.omo/logs/.
type Logger struct {
	mu   sync.Mutex
	file *os.File
	name string
}

var (
	globalLogger   *Logger
	globalLoggerMu sync.RWMutex
)

// SetPluginLogger registers a logger that plugins can access via Log().
// Called by the host each time a plugin is activated.
func SetPluginLogger(l *Logger) {
	globalLoggerMu.Lock()
	defer globalLoggerMu.Unlock()
	// Close previous plugin logger if any
	if globalLogger != nil {
		globalLogger.Close()
	}
	globalLogger = l
}

// Log returns the current plugin logger.
// Returns a no-op logger if none has been set.
func Log() *Logger {
	globalLoggerMu.RLock()
	defer globalLoggerMu.RUnlock()
	if globalLogger == nil {
		return &Logger{} // no-op: file is nil so writes are silently dropped
	}
	return globalLogger
}

// ClosePluginLogger closes the current plugin logger.
func ClosePluginLogger() {
	globalLoggerMu.Lock()
	defer globalLoggerMu.Unlock()
	if globalLogger != nil {
		globalLogger.Close()
		globalLogger = nil
	}
}

// NewLogger opens (or creates) the log file for the given name.
// The file is stored at ~/.omo/logs/<name>.log and is opened in
// append mode so logs survive restarts.
func NewLogger(name string) (*Logger, error) {
	dir := LogsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create logs dir: %w", err)
	}

	path := filepath.Join(dir, name+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("open log file %s: %w", path, err)
	}

	l := &Logger{file: f, name: name}
	l.Info("logger started")
	return l, nil
}

// Info writes an informational log line.
func (l *Logger) Info(format string, args ...interface{}) {
	l.write("INF", format, args...)
}

// Warn writes a warning log line.
func (l *Logger) Warn(format string, args ...interface{}) {
	l.write("WRN", format, args...)
}

// Error writes an error log line.
func (l *Logger) Error(format string, args ...interface{}) {
	l.write("ERR", format, args...)
}

// Close flushes and closes the log file.
func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		l.file.Close()
		l.file = nil
	}
}

func (l *Logger) write(level, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return
	}
	ts := time.Now().Format("2006-01-02 15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.file, "%s [%s] %s\n", ts, level, msg)
}
