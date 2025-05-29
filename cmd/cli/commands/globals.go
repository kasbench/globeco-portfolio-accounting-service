package commands

import (
	"github.com/kasbench/globeco-portfolio-accounting-service/internal/config"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
)

// Global variables for shared configuration and logger
var (
	globalConfig *config.Config
	globalLogger logger.Logger
)

// SetGlobalConfig sets the global configuration for all commands
func SetGlobalConfig(cfg *config.Config) {
	globalConfig = cfg
}

// SetGlobalLogger sets the global logger for all commands
func SetGlobalLogger(lg logger.Logger) {
	globalLogger = lg
}

// GetGlobalConfig returns the global configuration
func GetGlobalConfig() *config.Config {
	return globalConfig
}

// GetGlobalLogger returns the global logger
func GetGlobalLogger() logger.Logger {
	return globalLogger
}
