package logging

// Logger interface defines the logging methods
type Logger interface {
	Info(message string, args ...interface{})
	Error(message string, args ...interface{})
	Debug(message string, args ...interface{})
	Warn(message string, args ...interface{})
	LogError(err error, message string, args ...interface{})
	WithFields(fields map[string]interface{}) Logger
	GetLogger() interface{}
}

// NewLogger creates a new logger instance using the simple logger
func NewLogger() Logger {
	// Initialize with default settings
	return NewSimpleLogger(Config{
		Level: "info",
		Format: "text",
	})
}