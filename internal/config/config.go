package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for our application
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Cache    CacheConfig    `mapstructure:"cache"`
	Kafka    KafkaConfig    `mapstructure:"kafka"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Metrics  MetricsConfig  `mapstructure:"metrics"`
	Tracing  TracingConfig  `mapstructure:"tracing"`
	External ExternalConfig `mapstructure:"external"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Host                    string        `mapstructure:"host"`
	Port                    int           `mapstructure:"port"`
	ReadTimeout             time.Duration `mapstructure:"read_timeout"`
	WriteTimeout            time.Duration `mapstructure:"write_timeout"`
	IdleTimeout             time.Duration `mapstructure:"idle_timeout"`
	GracefulShutdownTimeout time.Duration `mapstructure:"graceful_shutdown_timeout"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	Database        string        `mapstructure:"database"`
	SSLMode         string        `mapstructure:"ssl_mode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	MigrationsPath  string        `mapstructure:"migrations_path"`
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	Enabled     bool          `mapstructure:"enabled"`
	Hosts       []string      `mapstructure:"hosts"`
	ClusterName string        `mapstructure:"cluster_name"`
	Username    string        `mapstructure:"username"`
	Password    string        `mapstructure:"password"`
	TTL         time.Duration `mapstructure:"ttl"`
	Timeout     time.Duration `mapstructure:"timeout"`
}

// KafkaConfig holds Kafka configuration
type KafkaConfig struct {
	Enabled   bool          `mapstructure:"enabled"`
	Brokers   []string      `mapstructure:"brokers"`
	Topic     string        `mapstructure:"topic"`
	GroupID   string        `mapstructure:"group_id"`
	BatchSize int           `mapstructure:"batch_size"`
	Timeout   time.Duration `mapstructure:"timeout"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Output     string `mapstructure:"output"`
	Structured bool   `mapstructure:"structured"`
}

// MetricsConfig holds metrics configuration
type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
	Port    int    `mapstructure:"port"`
}

// TracingConfig holds tracing configuration
type TracingConfig struct {
	Enabled     bool    `mapstructure:"enabled"`
	ServiceName string  `mapstructure:"service_name"`
	Endpoint    string  `mapstructure:"endpoint"`
	SampleRate  float64 `mapstructure:"sample_rate"`
}

// ExternalConfig holds external service configuration
type ExternalConfig struct {
	PortfolioService ServiceConfig `mapstructure:"portfolio_service"`
	SecurityService  ServiceConfig `mapstructure:"security_service"`
}

// ServiceConfig holds individual service configuration
type ServiceConfig struct {
	Host                    string        `mapstructure:"host"`
	Port                    int           `mapstructure:"port"`
	Timeout                 time.Duration `mapstructure:"timeout"`
	MaxRetries              int           `mapstructure:"max_retries"`
	RetryBackoff            time.Duration `mapstructure:"retry_backoff"`
	CircuitBreakerThreshold int           `mapstructure:"circuit_breaker_threshold"`
}

// Load loads configuration from multiple sources
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/etc/globeco-portfolio-accounting/")

	// Set defaults
	setDefaults()

	// Enable environment variable support
	viper.AutomaticEnv()
	viper.SetEnvPrefix("GLOBECO_PA") // GLOBECO_PA_SERVER_PORT
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Read configuration file (optional)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found is OK, we'll use defaults and env vars
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// Server defaults
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8087)
	viper.SetDefault("server.read_timeout", "30s")
	viper.SetDefault("server.write_timeout", "30s")
	viper.SetDefault("server.idle_timeout", "120s")
	viper.SetDefault("server.graceful_shutdown_timeout", "30s")

	// Database defaults
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.user", "globeco")
	viper.SetDefault("database.password", "password")
	viper.SetDefault("database.database", "portfolio_accounting")
	viper.SetDefault("database.ssl_mode", "disable")
	viper.SetDefault("database.max_open_conns", 25)
	viper.SetDefault("database.max_idle_conns", 5)
	viper.SetDefault("database.conn_max_lifetime", "15m")
	viper.SetDefault("database.migrations_path", "migrations")

	// Cache defaults
	viper.SetDefault("cache.enabled", true)
	viper.SetDefault("cache.hosts", []string{"localhost:5701"})
	viper.SetDefault("cache.cluster_name", "globeco-portfolio-accounting")
	viper.SetDefault("cache.ttl", "1h")
	viper.SetDefault("cache.timeout", "5s")

	// Kafka defaults
	viper.SetDefault("kafka.enabled", true)
	viper.SetDefault("kafka.brokers", []string{"localhost:9092"})
	viper.SetDefault("kafka.topic", "portfolio-accounting-events")
	viper.SetDefault("kafka.group_id", "portfolio-accounting-service")
	viper.SetDefault("kafka.batch_size", 100)
	viper.SetDefault("kafka.timeout", "10s")

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.output", "stdout")
	viper.SetDefault("logging.structured", true)

	// Metrics defaults
	viper.SetDefault("metrics.enabled", true)
	viper.SetDefault("metrics.path", "/metrics")
	viper.SetDefault("metrics.port", 9090)

	// Tracing defaults
	viper.SetDefault("tracing.enabled", true)
	viper.SetDefault("tracing.service_name", "globeco-portfolio-accounting-service")
	viper.SetDefault("tracing.endpoint", "http://localhost:14268/api/traces")
	viper.SetDefault("tracing.sample_rate", 0.1)

	// External services defaults
	viper.SetDefault("external.portfolio_service.host", "globeco-portfolio-service-kafka")
	viper.SetDefault("external.portfolio_service.port", 8001)
	viper.SetDefault("external.portfolio_service.timeout", "30s")
	viper.SetDefault("external.portfolio_service.max_retries", 3)
	viper.SetDefault("external.portfolio_service.retry_backoff", "1s")
	viper.SetDefault("external.portfolio_service.circuit_breaker_threshold", 5)

	viper.SetDefault("external.security_service.host", "globeco-security-service")
	viper.SetDefault("external.security_service.port", 8000)
	viper.SetDefault("external.security_service.timeout", "30s")
	viper.SetDefault("external.security_service.max_retries", 3)
	viper.SetDefault("external.security_service.retry_backoff", "1s")
	viper.SetDefault("external.security_service.circuit_breaker_threshold", 5)
}

// DatabaseConnectionString returns the database connection string
func (c *DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode)
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}

	if c.Database.Port <= 0 || c.Database.Port > 65535 {
		return fmt.Errorf("invalid database port: %d", c.Database.Port)
	}

	if c.Cache.Enabled && len(c.Cache.Hosts) == 0 {
		return fmt.Errorf("cache hosts are required when cache is enabled")
	}

	if c.Kafka.Enabled && len(c.Kafka.Brokers) == 0 {
		return fmt.Errorf("kafka brokers are required when kafka is enabled")
	}

	return nil
}
