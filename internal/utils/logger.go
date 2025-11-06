// internal/utils/logger.go
package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// LogLevel represents the logging level
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARNING
	ERROR
	FATAL
)

// Logger represents a structured logger
type Logger struct {
	mu       sync.Mutex
	file     *os.File
	level    LogLevel
	enabled  bool
}

// LogEntry represents a log entry
type LogEntry struct {
	Level     string    `json:"level"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
	File      string    `json:"file"`
	Line      int       `json:"line"`
	Func      string    `json:"func"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

var (
	globalLogger *Logger
	loggerOnce   sync.Once
)

// GetLogger returns the global logger instance
func GetLogger() *Logger {
	loggerOnce.Do(func() {
		globalLogger = &Logger{
			level:   INFO,
			enabled: true,
		}
	})
	return globalLogger
}

// InitLogger initializes the logger with a log file
func InitLogger(logFile string) error {
	logger := GetLogger()
	
	// Ensure log directory exists
	logDir := filepath.Dir(logFile)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	logger.mu.Lock()
	defer logger.mu.Unlock()
	
	// Close previous file if exists
	if logger.file != nil {
		logger.file.Close()
	}
	
	logger.file = file
	return nil
}

// SetLogLevel sets the minimum level for logging
func (l *Logger) SetLogLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// Enable enables or disables logging
func (l *Logger) Enable(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enabled = enabled
}

// log writes a log entry
func (l *Logger) log(level LogLevel, message string, fields map[string]interface{}) {
	if !l.enabled || level < l.level {
		return
	}

	// Get caller information
	_, file, line, ok := runtime.Caller(2) // Skip log function and caller
	funcName := ""
	if ok {
		// Extract function name
		pc, _, _, _ := runtime.Caller(2)
		if fn := runtime.FuncForPC(pc); fn != nil {
			funcName = fn.Name()
			// Extract just the function name without package path
			if idx := strings.LastIndex(funcName, "/"); idx >= 0 {
				funcName = funcName[idx+1:]
			}
		}
		// Extract just filename without full path
		if idx := strings.LastIndex(file, "/"); idx >= 0 {
			file = file[idx+1:]
		}
	}

	// Create log entry
	entry := LogEntry{
		Level:     l.levelToString(level),
		Timestamp: time.Now(),
		Message:   message,
		File:      file,
		Line:      line,
		Func:      funcName,
		Fields:    fields,
	}

	// Format log entry
	logLine := fmt.Sprintf("[%s] %s %s:%d:%s - %s",
		entry.Level,
		entry.Timestamp.Format("2006-01-02 15:04:05.000"),
		entry.File,
		entry.Line,
		entry.Func,
		entry.Message)

	// Add fields if present
	if len(entry.Fields) > 0 {
		logLine += " |"
		for key, value := range entry.Fields {
			logLine += fmt.Sprintf(" %s=%v", key, value)
		}
	}

	logLine += "\n"

	// Write to file and stdout
	l.mu.Lock()
	defer l.mu.Unlock()

	// Write to file if available
	if l.file != nil {
		l.file.WriteString(logLine)
		l.file.Sync() // Ensure immediate write
	}

	// Always write to stdout
	os.Stdout.WriteString(logLine)

	// For fatal errors, exit
	if level == FATAL {
		os.Exit(1)
	}
}

// levelToString converts log level to string
func (l *Logger) levelToString(level LogLevel) string {
	switch level {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARNING:
		return "WARNING"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Debug logs a debug message
func (l *Logger) Debug(message string, fields map[string]interface{}) {
	l.log(DEBUG, message, fields)
}

// Info logs an info message
func (l *Logger) Info(message string, fields map[string]interface{}) {
	l.log(INFO, message, fields)
}

// Warn logs a warning message
func (l *Logger) Warn(message string, fields map[string]interface{}) {
	l.log(WARNING, message, fields)
}

// Error logs an error message
func (l *Logger) Error(message string, fields map[string]interface{}) {
	l.log(ERROR, message, fields)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(message string, fields map[string]interface{}) {
	l.log(FATAL, message, fields)
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log(DEBUG, fmt.Sprintf(format, args...), nil)
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...interface{}) {
	l.log(INFO, fmt.Sprintf(format, args...), nil)
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.log(WARNING, fmt.Sprintf(format, args...), nil)
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log(ERROR, fmt.Sprintf(format, args...), nil)
}

// Fatalf logs a formatted fatal message and exits
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.log(FATAL, fmt.Sprintf(format, args...), nil)
}
