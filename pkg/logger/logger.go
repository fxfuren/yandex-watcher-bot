package logger

import (
	"fmt"
	"log"
	"os"
	"time"
)

// Level represents the log level
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelCritical
)

var (
	currentLevel = LevelInfo
	logger       = log.New(os.Stdout, "", 0)
)

// SetLevel sets the minimum log level
func SetLevel(level Level) {
	currentLevel = level
}

func logf(level Level, format string, args ...interface{}) {
	if level < currentLevel {
		return
	}

	var prefix string
	switch level {
	case LevelDebug:
		prefix = "DEBUG"
	case LevelInfo:
		prefix = "INFO"
	case LevelWarn:
		prefix = "WARN"
	case LevelError:
		prefix = "ERROR"
	case LevelCritical:
		prefix = "CRITICAL"
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)
	logger.Printf("[%s] %s: %s", timestamp, prefix, message)
}

// Debug logs a debug message
func Debug(msg string, keysAndValues ...interface{}) {
	logf(LevelDebug, "%s", formatMessage(msg, keysAndValues...))
}

// Info logs an info message
func Info(msg string, keysAndValues ...interface{}) {
	logf(LevelInfo, "%s", formatMessage(msg, keysAndValues...))
}

// Warn logs a warning message
func Warn(msg string, keysAndValues ...interface{}) {
	logf(LevelWarn, "%s", formatMessage(msg, keysAndValues...))
}

// Error logs an error message
func Error(msg string, keysAndValues ...interface{}) {
	logf(LevelError, "%s", formatMessage(msg, keysAndValues...))
}

// Critical logs a critical message
func Critical(msg string, keysAndValues ...interface{}) {
	logf(LevelCritical, "%s", formatMessage(msg, keysAndValues...))
}

func formatMessage(msg string, keysAndValues ...interface{}) string {
	if len(keysAndValues) == 0 {
		return msg
	}

	result := msg
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 < len(keysAndValues) {
			result += fmt.Sprintf(" %v=%v", keysAndValues[i], keysAndValues[i+1])
		}
	}
	return result
}
