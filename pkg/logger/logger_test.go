package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewDevelopment(t *testing.T) {
	t.Run("Development logger creation", func(t *testing.T) {
		logger := NewDevelopment()
		assert.NotNil(t, logger)
		assert.IsType(t, &zapLogger{}, logger)
	})
}

func TestNewProduction(t *testing.T) {
	t.Run("Production logger creation", func(t *testing.T) {
		logger := NewProduction()
		assert.NotNil(t, logger)
		assert.IsType(t, &zapLogger{}, logger)
	})
}

func TestNewNoop(t *testing.T) {
	t.Run("Noop logger creation", func(t *testing.T) {
		logger := NewNoop()
		assert.NotNil(t, logger)
		assert.IsType(t, &zapLogger{}, logger)
	})

	t.Run("Noop logger does not panic", func(t *testing.T) {
		logger := NewNoop()

		// Should not panic when called
		assert.NotPanics(t, func() {
			logger.Info("test message")
			logger.Error("test error")
			logger.Debug("test debug")
			logger.Warn("test warn")
		})
	})
}

func TestNew_WithConfig(t *testing.T) {
	t.Run("Valid development config", func(t *testing.T) {
		config := Config{
			Level:      "debug",
			Format:     "console",
			Output:     "stdout",
			Structured: false,
		}

		logger, err := New(config)
		assert.NoError(t, err)
		assert.NotNil(t, logger)
		assert.IsType(t, &zapLogger{}, logger)
	})

	t.Run("Valid production config", func(t *testing.T) {
		config := Config{
			Level:      "info",
			Format:     "json",
			Output:     "stdout",
			Structured: true,
		}

		logger, err := New(config)
		assert.NoError(t, err)
		assert.NotNil(t, logger)
		assert.IsType(t, &zapLogger{}, logger)
	})

	t.Run("Invalid log level", func(t *testing.T) {
		config := Config{
			Level:  "invalid",
			Format: "json",
			Output: "stdout",
		}

		logger, err := New(config)
		assert.Error(t, err)
		assert.Nil(t, logger)
		assert.Contains(t, err.Error(), "invalid log level")
	})
}

func TestZapLogger_Methods(t *testing.T) {
	logger := NewDevelopment()

	t.Run("Info logging", func(t *testing.T) {
		assert.NotPanics(t, func() {
			logger.Info("Test info message")
			logger.Info("Test info with field", zap.String("key", "value"))
		})
	})

	t.Run("Error logging", func(t *testing.T) {
		assert.NotPanics(t, func() {
			logger.Error("Test error message")
			logger.Error("Test error with field", zap.String("error", "test"))
		})
	})

	t.Run("Debug logging", func(t *testing.T) {
		assert.NotPanics(t, func() {
			logger.Debug("Test debug message")
			logger.Debug("Test debug with field", zap.String("debug", "test"))
		})
	})

	t.Run("Warn logging", func(t *testing.T) {
		assert.NotPanics(t, func() {
			logger.Warn("Test warn message")
			logger.Warn("Test warn with field", zap.String("warn", "test"))
		})
	})
}

func TestZapLogger_WithFields(t *testing.T) {
	logger := NewDevelopment()

	t.Run("With single field", func(t *testing.T) {
		childLogger := logger.With(zap.String("component", "test"))
		assert.NotNil(t, childLogger)
		assert.IsType(t, &zapLogger{}, childLogger)

		assert.NotPanics(t, func() {
			childLogger.Info("Test message with component field")
		})
	})

	t.Run("With multiple fields", func(t *testing.T) {
		childLogger := logger.With(
			zap.String("component", "test"),
			zap.String("operation", "unit-test"),
			zap.Int("count", 42),
		)
		assert.NotNil(t, childLogger)

		assert.NotPanics(t, func() {
			childLogger.Info("Test message with multiple fields")
		})
	})
}

func TestZapLogger_Sync(t *testing.T) {
	logger := NewDevelopment()

	t.Run("Sync does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			err := logger.Sync()
			// Sync may return an error on some systems (especially in tests)
			// but it should not panic
			_ = err
		})
	})
}

func TestGlobalLogger(t *testing.T) {
	t.Run("GetGlobal returns logger", func(t *testing.T) {
		logger := GetGlobal()
		assert.NotNil(t, logger)
		assert.Implements(t, (*Logger)(nil), logger)
	})

	t.Run("InitGlobal with valid config", func(t *testing.T) {
		config := Config{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		}

		err := InitGlobal(config)
		assert.NoError(t, err)

		logger := GetGlobal()
		assert.NotNil(t, logger)
	})

	t.Run("InitGlobal with invalid config", func(t *testing.T) {
		config := Config{
			Level:  "invalid",
			Format: "json",
			Output: "stdout",
		}

		err := InitGlobal(config)
		assert.Error(t, err)
	})
}

func TestConvenienceFunctions(t *testing.T) {
	// Initialize with noop logger to avoid output during tests
	config := Config{
		Level:  "debug",
		Format: "json",
		Output: "stdout",
	}
	err := InitGlobal(config)
	require.NoError(t, err)

	t.Run("Global logging functions", func(t *testing.T) {
		assert.NotPanics(t, func() {
			Debug("test debug")
			Info("test info")
			Warn("test warn")
			LogError("test error")
		})
	})

	t.Run("Global With function", func(t *testing.T) {
		assert.NotPanics(t, func() {
			childLogger := With(zap.String("test", "value"))
			assert.NotNil(t, childLogger)
			childLogger.Info("test with child logger")
		})
	})

	t.Run("Global Sync function", func(t *testing.T) {
		assert.NotPanics(t, func() {
			err := Sync()
			_ = err // Sync may return error in tests
		})
	})
}

func TestUtilityFunctions(t *testing.T) {
	t.Run("Field utility functions", func(t *testing.T) {
		// Test that utility functions create proper zap fields
		stringField := String("key", "value")
		assert.Equal(t, zap.String("key", "value"), stringField)

		intField := Int("count", 42)
		assert.Equal(t, zap.Int("count", 42), intField)

		int64Field := Int64("bignum", 123456789)
		assert.Equal(t, zap.Int64("bignum", 123456789), int64Field)

		float64Field := Float64("pi", 3.14159)
		assert.Equal(t, zap.Float64("pi", 3.14159), float64Field)

		boolField := Bool("enabled", true)
		assert.Equal(t, zap.Bool("enabled", true), boolField)
	})

	t.Run("Error field utility", func(t *testing.T) {
		testErr := assert.AnError
		errField := Err(testErr)
		assert.Equal(t, zap.Error(testErr), errField)
	})
}

func TestLoggerInterface_Compliance(t *testing.T) {
	t.Run("zapLogger implements Logger interface", func(t *testing.T) {
		var logger Logger = NewDevelopment()
		assert.NotNil(t, logger)

		// Test that all interface methods are available
		assert.NotPanics(t, func() {
			logger.Info("test")
			logger.Error("test")
			logger.Debug("test")
			logger.Warn("test")
			logger.With(zap.String("test", "value"))
			logger.Sync()
		})
	})

	t.Run("Noop logger implements Logger interface", func(t *testing.T) {
		var logger Logger = NewNoop()
		assert.NotNil(t, logger)

		// Test that all interface methods are available
		assert.NotPanics(t, func() {
			logger.Info("test")
			logger.Error("test")
			logger.Debug("test")
			logger.Warn("test")
			logger.With(zap.String("test", "value"))
			logger.Sync()
		})
	})
}
