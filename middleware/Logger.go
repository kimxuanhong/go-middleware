package middleware

import (
	"context"
	"fmt"
	"github.com/kimxuanhong/go-logger/logger"
)

// Logger interface defines the logging methods
type Logger interface {
	LogRequest(entry LogEntry)
	LogResponse(entry LogEntry)
	LogError(requestID string, err error)
}

// DefaultLogger implements Logger interface using standard log package
type DefaultLogger struct {
	logger logger.Logger
}

// NewDefaultLogger creates a new DefaultLogger
func NewDefaultLogger() *DefaultLogger {
	return &DefaultLogger{
		logger: logger.DefaultLogger(),
	}
}

// NewLogger creates a new NewLogger
func NewLogger(config *logger.Config) *DefaultLogger {
	return &DefaultLogger{
		logger: logger.NewLogger(config),
	}
}

// LogRequest implements Logger interface for DefaultLogger
func (l *DefaultLogger) LogRequest(entry LogEntry) {
	message := fmt.Sprintf("%s %s - %d in %v\nClientIP: %s, UserAgent: %s\nRequest: %s\n",
		entry.Method, entry.Path,
		entry.StatusCode,
		formatDuration(entry.ProcessTime),
		entry.ClientIP,
		entry.UserAgent,
		compactJSON(entry.Request),
	)
	ctx := context.WithValue(context.Background(), logger.RequestIDKey, entry.RequestID)
	l.logger.WithContext(ctx).Info(message)
}

// LogResponse implements Logger interface for DefaultLogger
func (l *DefaultLogger) LogResponse(entry LogEntry) {
	message := fmt.Sprintf("%s %s - %d in %v\nClientIP: %s, UserAgent: %s\nResponse: %s\n",
		entry.Method, entry.Path,
		entry.StatusCode,
		formatDuration(entry.ProcessTime),
		entry.ClientIP,
		entry.UserAgent,
		compactJSON(entry.Response),
	)
	ctx := context.WithValue(context.Background(), logger.RequestIDKey, entry.RequestID)
	l.logger.WithContext(ctx).Info("[REQUEST] %v", message)
}

// LogError implements Logger interface for DefaultLogger
func (l *DefaultLogger) LogError(requestID string, err error) {
	ctx := context.WithValue(context.Background(), logger.RequestIDKey, requestID)
	l.logger.WithContext(ctx).Error("[ERROR] %v", err)
}
