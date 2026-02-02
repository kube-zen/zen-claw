package logging

import (
	"fmt"
	"log"
	"os"
)

// SimpleLogger is a basic logger that doesn't depend on external SDKs
type SimpleLogger struct {
	infoLog  *log.Logger
	errorLog *log.Logger
	debugLog *log.Logger
	level    string
}

// Config for logger
type Config struct {
	Level  string
	Format string
}

// NewSimpleLogger creates a new simple logger instance
func NewSimpleLogger(cfg Config) *SimpleLogger {
	if cfg.Level == "" {
		cfg.Level = "info"
	}
	
	return &SimpleLogger{
		infoLog:  log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile),
		errorLog: log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile),
		debugLog: log.New(os.Stdout, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile),
		level:    cfg.Level,
	}
}

// Info logs an info message
func (l *SimpleLogger) Info(message string, args ...interface{}) {
	if l.level == "debug" || l.level == "info" {
		l.infoLog.Printf(message, args...)
	}
}

// Error logs an error message
func (l *SimpleLogger) Error(message string, args ...interface{}) {
	l.errorLog.Printf(message, args...)
}

// Debug logs a debug message
func (l *SimpleLogger) Debug(message string, args ...interface{}) {
	if l.level == "debug" {
		l.debugLog.Printf(message, args...)
	}
}

// Warn logs a warning message
func (l *SimpleLogger) Warn(message string, args ...interface{}) {
	if l.level == "debug" || l.level == "info" || l.level == "warn" {
		log.New(os.Stdout, "WARN: ", log.Ldate|log.Ltime|log.Lshortfile).Printf(message, args...)
	}
}

// LogError logs an error with context
func (l *SimpleLogger) LogError(err error, message string, args ...interface{}) {
	if err != nil {
		l.Error("%s: %v", fmt.Sprintf(message, args...), err)
	} else {
		l.Info(message, args...)
	}
}

// WithFields creates a new logger with additional fields (simplified)
func (l *SimpleLogger) WithFields(fields map[string]interface{}) Logger {
	// For simplicity, just return the same logger
	// In a real implementation, you'd store the fields
	return l
}

// GetLogger returns the underlying logger (returns self for interface compatibility)
func (l *SimpleLogger) GetLogger() interface{} {
	return l
}