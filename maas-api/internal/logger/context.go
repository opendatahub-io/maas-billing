package logger

import (
	"context"
)

type contextKey string

const loggerKey contextKey = "logger"

// WithLogger adds a logger to the context.
func WithLogger(ctx context.Context, logger *Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// FromContext retrieves a logger from the context.
// Returns a default logger if none is found.
func FromContext(ctx context.Context) *Logger {
	if logger, ok := ctx.Value(loggerKey).(*Logger); ok {
		return logger
	}
	// Return a default logger if none is in context
	return NewFromEnv()
}
