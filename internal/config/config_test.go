package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnhancedMetricsConfig(t *testing.T) {
	// Test that the EnhancedMetricsConfig struct is properly defined
	config := EnhancedMetricsConfig{
		Enabled:     true,
		ServiceName: "test-service",
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, "test-service", config.ServiceName)
}

func TestMetricsConfig_WithEnhanced(t *testing.T) {
	// Test that the MetricsConfig struct includes enhanced metrics
	config := MetricsConfig{
		Enabled: true,
		Path:    "/metrics",
		Port:    9090,
		Enhanced: EnhancedMetricsConfig{
			Enabled:     true,
			ServiceName: "test-service",
		},
	}

	assert.True(t, config.Enabled)
	assert.Equal(t, "/metrics", config.Path)
	assert.Equal(t, 9090, config.Port)
	assert.True(t, config.Enhanced.Enabled)
	assert.Equal(t, "test-service", config.Enhanced.ServiceName)
}

func TestConfig_WithEnhancedMetrics(t *testing.T) {
	// Test that the main Config struct includes enhanced metrics
	config := Config{
		Metrics: MetricsConfig{
			Enabled: true,
			Enhanced: EnhancedMetricsConfig{
				Enabled:     true,
				ServiceName: "test-service",
			},
		},
	}

	assert.True(t, config.Metrics.Enabled)
	assert.True(t, config.Metrics.Enhanced.Enabled)
	assert.Equal(t, "test-service", config.Metrics.Enhanced.ServiceName)
}