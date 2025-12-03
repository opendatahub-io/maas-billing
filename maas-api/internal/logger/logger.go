package logger

import (
	"context"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps zap.Logger to provide a structured logging interface
// aligned with KServe conventions.
type Logger struct {
	*zap.SugaredLogger

	level zapcore.Level
}

// New creates a new logger instance with KServe-compatible configuration.
// It supports different log levels (DEBUG, INFO, WARN, ERROR) and structured output.
func New(debug bool) *Logger {
	var config zap.Config
	if debug {
		config = zap.NewDevelopmentConfig()
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config = zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
		config.EncoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
	}

	// KServe-style configuration
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.MessageKey = "message"
	config.EncoderConfig.LevelKey = "level"
	config.EncoderConfig.CallerKey = "caller"
	config.EncoderConfig.StacktraceKey = "stacktrace"
	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}

	// Build logger
	baseLogger, err := config.Build(
		zap.AddCaller(),
		zap.AddCallerSkip(1), // Skip this package in call stack
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		// Fallback to a basic logger if configuration fails
		baseLogger = zap.NewExample()
	}

	level := zapcore.InfoLevel
	if debug {
		level = zapcore.DebugLevel
	}

	return &Logger{
		SugaredLogger: baseLogger.Sugar(),
		level:         level,
	}
}

// NewFromEnv creates a logger based on environment variables.
// Checks DEBUG_MODE environment variable to determine log level.
func NewFromEnv() *Logger {
	debug := os.Getenv("DEBUG_MODE") == "true" || os.Getenv("DEBUG_MODE") == "1"
	return New(debug)
}

// WithContext returns a logger with context fields attached.
// This is useful for request-scoped logging with trace IDs, user info, etc.
func (l *Logger) WithContext(ctx context.Context) *Logger {
	// Extract common context values that might be useful for logging
	fields := []any{}

	// Add request ID if available
	if requestID := ctx.Value("request_id"); requestID != nil {
		fields = append(fields, "request_id", requestID)
	}

	// Add user info if available
	if user := ctx.Value("user"); user != nil {
		fields = append(fields, "user", user)
	}

	if len(fields) > 0 {
		return &Logger{
			SugaredLogger: l.With(fields...),
			level:         l.level,
		}
	}

	return l
}

// WithFields returns a logger with additional structured fields.
// This follows KServe's pattern of using structured logging for better observability.
func (l *Logger) WithFields(fields ...any) *Logger {
	return &Logger{
		SugaredLogger: l.With(fields...),
		level:         l.level,
	}
}

// WithError returns a logger with an error field attached.
// This is a convenience method for error logging.
func (l *Logger) WithError(err error) *Logger {
	return &Logger{
		SugaredLogger: l.With("error", err.Error()),
		level:         l.level,
	}
}

// Debug logs a debug-level message with optional fields.
// Only logged when debug mode is enabled.
func (l *Logger) Debug(msg string, fields ...any) {
	if l.level <= zapcore.DebugLevel {
		l.Debugw(msg, fields...)
	}
}

// Info logs an info-level message with optional fields.
func (l *Logger) Info(msg string, fields ...any) {
	if l.level <= zapcore.InfoLevel {
		l.Infow(msg, fields...)
	}
}

// Warn logs a warning-level message with optional fields.
func (l *Logger) Warn(msg string, fields ...any) {
	if l.level <= zapcore.WarnLevel {
		l.Warnw(msg, fields...)
	}
}

// Error logs an error-level message with optional fields.
// Automatically includes stack trace in production mode.
func (l *Logger) Error(msg string, fields ...any) {
	if l.level <= zapcore.ErrorLevel {
		l.Errorw(msg, fields...)
	}
}

// Fatal logs a fatal-level message and exits the program.
func (l *Logger) Fatal(msg string, fields ...any) {
	l.Fatalw(msg, fields...)
}

// Sync flushes any buffered log entries.
// Should be called before program exit.
func (l *Logger) Sync() error {
	return l.SugaredLogger.Sync()
}
