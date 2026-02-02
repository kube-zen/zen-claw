package logging

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/neves/zen-sdk/pkg/logging"
)

// Logger wraps the Zen SDK logger for consistent logging
type Logger struct {
	logger *logging.Logger
}

// NewLogger creates a new logger instance
func NewLogger() *Logger {
	// Initialize with default settings
	logger := logging.NewLogger(logging.Config{
		Level: "info",
		Format: "json",
	})
	
	return &Logger{logger: logger}
}

// Info logs an info message
func (l *Logger) Info(message string, args ...interface{}) {
	l.logger.Info(fmt.Sprintf(message, args...))
}

// Error logs an error message
func (l *Logger) Error(message string, args ...interface{}) {
	l.logger.Error(fmt.Sprintf(message, args...))
}

// Debug logs a debug message
func (l *Logger) Debug(message string, args ...interface{}) {
	l.logger.Debug(fmt.Sprintf(message, args...))
}

// Warn logs a warning message
func (l *Logger) Warn(message string, args ...interface{}) {
	l.logger.Warn(fmt.Sprintf(message, args...))
}

// WithFields adds structured fields to log entries
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	// This would integrate with Zen SDK's structured logging
	return l
}

// LogError logs an error with stack trace if available
func (l *Logger) LogError(err error, message string, args ...interface{}) {
	if err != nil {
		l.logger.Error(fmt.Sprintf("%s: %v", fmt.Sprintf(message, args...), err))
	} else {
		l.logger.Info(fmt.Sprintf(message, args...))
	}
}

// GetLogger returns the underlying logger
func (l *Logger) GetLogger() *logging.Logger {
	return l.logger
}