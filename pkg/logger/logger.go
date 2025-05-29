package logger

import (
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
)

// Logger interface defines logging methods
type Logger interface {
	Debug(msg string, fields ...zap.Field)
	Info(msg string, fields ...zap.Field)
	Warn(msg string, fields ...zap.Field)
	Error(msg string, fields ...zap.Field)
	Fatal(msg string, fields ...zap.Field)
	With(fields ...zap.Field) Logger
	Sync() error
}

// zapLogger wraps zap.Logger to implement our Logger interface
type zapLogger struct {
	logger *zap.Logger
}

// Config holds logger configuration
type Config struct {
	Level      string // debug, info, warn, error
	Format     string // json, console
	Output     string // stdout, stderr, file path
	Structured bool   // whether to use structured logging
}

// New creates a new logger instance
func New(config Config) (Logger, error) {
	zapConfig := zap.NewProductionConfig()

	// Set log level
	level, err := zap.ParseAtomicLevel(config.Level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level %s: %w", config.Level, err)
	}
	zapConfig.Level = level

	// Set encoding format
	if config.Format == "console" || !config.Structured {
		zapConfig.Encoding = "console"
		zapConfig.EncoderConfig = zap.NewDevelopmentEncoderConfig()
	} else {
		zapConfig.Encoding = "json"
		zapConfig.EncoderConfig = zap.NewProductionEncoderConfig()
	}

	// Set output paths
	switch config.Output {
	case "stdout", "":
		zapConfig.OutputPaths = []string{"stdout"}
	case "stderr":
		zapConfig.OutputPaths = []string{"stderr"}
	default:
		// Assume it's a file path
		zapConfig.OutputPaths = []string{config.Output}
	}

	logger, err := zapConfig.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build logger: %w", err)
	}

	return &zapLogger{logger: logger}, nil
}

// NewDevelopment creates a development logger with console output
func NewDevelopment() Logger {
	logger, _ := zap.NewDevelopment()
	return &zapLogger{logger: logger}
}

// NewProduction creates a production logger with JSON output
func NewProduction() Logger {
	logger, _ := zap.NewProduction()
	return &zapLogger{logger: logger}
}

// NewNoop creates a no-op logger for testing
func NewNoop() Logger {
	return &zapLogger{logger: zap.NewNop()}
}

// Debug logs a debug message
func (l *zapLogger) Debug(msg string, fields ...zap.Field) {
	l.logger.Debug(msg, fields...)
}

// Info logs an info message
func (l *zapLogger) Info(msg string, fields ...zap.Field) {
	l.logger.Info(msg, fields...)
}

// Warn logs a warning message
func (l *zapLogger) Warn(msg string, fields ...zap.Field) {
	l.logger.Warn(msg, fields...)
}

// Error logs an error message
func (l *zapLogger) Error(msg string, fields ...zap.Field) {
	l.logger.Error(msg, fields...)
}

// Fatal logs a fatal message and exits
func (l *zapLogger) Fatal(msg string, fields ...zap.Field) {
	l.logger.Fatal(msg, fields...)
}

// With creates a child logger with the given fields
func (l *zapLogger) With(fields ...zap.Field) Logger {
	return &zapLogger{logger: l.logger.With(fields...)}
}

// Sync flushes any buffered log entries
func (l *zapLogger) Sync() error {
	return l.logger.Sync()
}

// Global logger instance
var globalLogger Logger

// InitGlobal initializes the global logger
func InitGlobal(config Config) error {
	logger, err := New(config)
	if err != nil {
		return err
	}
	globalLogger = logger
	return nil
}

// GetGlobal returns the global logger instance
func GetGlobal() Logger {
	if globalLogger == nil {
		// Fallback to development logger if not initialized
		globalLogger = NewDevelopment()
	}
	return globalLogger
}

// Convenience functions for global logger
func Debug(msg string, fields ...zap.Field) {
	GetGlobal().Debug(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	GetGlobal().Info(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	GetGlobal().Warn(msg, fields...)
}

func LogError(msg string, fields ...zap.Field) {
	GetGlobal().Error(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	GetGlobal().Fatal(msg, fields...)
	os.Exit(1)
}

func With(fields ...zap.Field) Logger {
	return GetGlobal().With(fields...)
}

func Sync() error {
	return GetGlobal().Sync()
}

// Utility functions for common fields
func String(key, val string) zap.Field {
	return zap.String(key, val)
}

func Int(key string, val int) zap.Field {
	return zap.Int(key, val)
}

func Int64(key string, val int64) zap.Field {
	return zap.Int64(key, val)
}

func Float64(key string, val float64) zap.Field {
	return zap.Float64(key, val)
}

func Bool(key string, val bool) zap.Field {
	return zap.Bool(key, val)
}

func Err(err error) zap.Field {
	return zap.Error(err)
}

func Duration(key string, val time.Duration) zap.Field {
	return zap.Duration(key, val)
}

func Any(key string, val interface{}) zap.Field {
	return zap.Any(key, val)
}
