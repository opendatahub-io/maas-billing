package logger

import (
	"context"
)

// NOTE: This file provides context-based logger passing, which is an alternative
// to dependency injection. The recommended approach is dependency injection
// (passing logger as a struct field), as it is more explicit and testable.
// This context-based approach is provided for cases where dependency injection
// is not practical (e.g., middleware, deeply nested calls).

type contextKey string

const loggerKey contextKey = "logger"

// WithLogger adds a logger to the context.
// Prefer dependency injection (passing logger as constructor parameter) when possible.
func WithLogger(ctx context.Context, logger *Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// FromContext retrieves a logger from the context.
// Returns a default logger if none is found.
// Prefer dependency injection (passing logger as constructor parameter) when possible.
func FromContext(ctx context.Context) *Logger {
	if logger, ok := ctx.Value(loggerKey).(*Logger); ok {
		return logger
	}
	// Return a default logger if none is in context
	return NewFromEnv()
}
