package commands

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/kasbench/globeco-portfolio-accounting-service/internal/config"
	"github.com/kasbench/globeco-portfolio-accounting-service/pkg/logger"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// StatusFlags holds flags for the status command
type StatusFlags struct {
	URL     string
	Timeout time.Duration
	Verbose bool
}

// NewStatusCommand creates a new status command
func NewStatusCommand() *cobra.Command {
	flags := &StatusFlags{}

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check service status and health",
		Long: `Check the status and health of the portfolio accounting service.

The status command will:
1. Check if the service is reachable
2. Verify health check endpoints
3. Display service version and environment
4. Show database and cache connectivity
5. Display service statistics and metrics

This is useful for monitoring and troubleshooting the service.`,
		Example: `  # Check service status using default URL
  portfolio-cli status

  # Check status with custom service URL
  portfolio-cli status --url http://localhost:8087

  # Check status with verbose output
  portfolio-cli status --verbose

  # Check status with custom timeout
  portfolio-cli status --timeout 30s`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatusCommand(cmd.Context(), flags)
		},
	}

	// Add flags
	cmd.Flags().StringVar(&flags.URL, "url", "", "service URL (default from config)")
	cmd.Flags().DurationVar(&flags.Timeout, "timeout", 10*time.Second, "request timeout")
	cmd.Flags().BoolVarP(&flags.Verbose, "verbose", "v", false, "verbose output")

	return cmd
}

// runStatusCommand executes the status command
func runStatusCommand(ctx context.Context, flags *StatusFlags) error {
	logger := GetGlobalLogger()
	config := GetGlobalConfig()

	if logger == nil {
		return fmt.Errorf("logger not initialized")
	}

	if config == nil {
		return fmt.Errorf("configuration not loaded")
	}

	// Determine service URL
	serviceURL := flags.URL
	if serviceURL == "" {
		serviceURL = fmt.Sprintf("http://%s:%d", config.Server.Host, config.Server.Port)
	}

	logger.Info("Checking service status",
		zap.String("url", serviceURL),
		zap.Duration("timeout", flags.Timeout),
		zap.Bool("verbose", flags.Verbose),
	)

	// Create status checker
	checker := NewStatusChecker(config, logger, flags.Timeout)

	// Check service status
	status, err := checker.CheckStatus(ctx, serviceURL, flags.Verbose)
	if err != nil {
		logger.Error("Status check failed", zap.Error(err))
		return fmt.Errorf("status check failed: %w", err)
	}

	// Print status results
	printStatusResults(status, flags.Verbose)

	// Return error if service is not healthy
	if !status.Healthy {
		return fmt.Errorf("service is not healthy")
	}

	return nil
}

// ServiceStatus holds the service status information
type ServiceStatus struct {
	URL         string
	Healthy     bool
	Reachable   bool
	Version     string
	Environment string
	Uptime      time.Duration
	Database    DatabaseStatus
	Cache       CacheStatus
	External    ExternalServicesStatus
	Metrics     ServiceMetrics
	Timestamp   time.Time
}

// DatabaseStatus holds database connectivity status
type DatabaseStatus struct {
	Connected    bool
	Version      string
	Connections  int
	ResponseTime time.Duration
}

// CacheStatus holds cache connectivity status
type CacheStatus struct {
	Connected    bool
	Cluster      string
	Members      int
	ResponseTime time.Duration
}

// ExternalServicesStatus holds external services status
type ExternalServicesStatus struct {
	PortfolioService ServiceHealth
	SecurityService  ServiceHealth
}

// ServiceHealth represents the health of an external service
type ServiceHealth struct {
	Available    bool
	ResponseTime time.Duration
	LastCheck    time.Time
}

// ServiceMetrics holds service metrics
type ServiceMetrics struct {
	RequestCount    int64
	ErrorCount      int64
	AverageResponse time.Duration
	ActiveRequests  int
}

// StatusChecker handles service status checking
type StatusChecker struct {
	config  *config.Config
	logger  logger.Logger
	timeout time.Duration
	client  *http.Client
}

// NewStatusChecker creates a new status checker
func NewStatusChecker(cfg *config.Config, lg logger.Logger, timeout time.Duration) *StatusChecker {
	return &StatusChecker{
		config:  cfg,
		logger:  lg,
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// CheckStatus checks the service status
func (s *StatusChecker) CheckStatus(ctx context.Context, serviceURL string, verbose bool) (*ServiceStatus, error) {
	s.logger.Info("Starting service status check",
		zap.String("url", serviceURL),
		zap.Bool("verbose", verbose),
	)

	status := &ServiceStatus{
		URL:       serviceURL,
		Healthy:   false,
		Reachable: false,
		Timestamp: time.Now(),
	}

	// TODO: Implement comprehensive status checking
	// - Check basic connectivity
	// - Call health endpoints
	// - Check database connectivity
	// - Check cache connectivity
	// - Check external services
	// - Collect metrics
	s.logger.Debug("Service status check - basic implementation")

	// Basic status for now
	status.Healthy = false
	status.Reachable = false

	s.logger.Info("Service status check completed",
		zap.String("url", serviceURL),
		zap.Bool("healthy", status.Healthy),
		zap.Bool("reachable", status.Reachable),
	)

	return status, nil
}

// checkBasicConnectivity checks if the service is reachable
func (s *StatusChecker) checkBasicConnectivity(ctx context.Context, serviceURL string) (bool, error) {
	healthURL := fmt.Sprintf("%s/api/v1/health", serviceURL)

	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// printStatusResults prints the status results to stdout
func printStatusResults(status *ServiceStatus, verbose bool) {
	fmt.Printf("\n=== Service Status ===\n")
	fmt.Printf("URL: %s\n", status.URL)
	fmt.Printf("Timestamp: %s\n", status.Timestamp.Format(time.RFC3339))

	// Overall health
	fmt.Printf("Overall Health: ")
	if status.Healthy {
		fmt.Printf("✅ HEALTHY\n")
	} else {
		fmt.Printf("❌ UNHEALTHY\n")
	}

	fmt.Printf("Reachable: ")
	if status.Reachable {
		fmt.Printf("✅ YES\n")
	} else {
		fmt.Printf("❌ NO\n")
	}

	// Service info
	if status.Version != "" {
		fmt.Printf("Version: %s\n", status.Version)
	}
	if status.Environment != "" {
		fmt.Printf("Environment: %s\n", status.Environment)
	}
	if status.Uptime > 0 {
		fmt.Printf("Uptime: %v\n", status.Uptime)
	}

	// Database status
	if verbose || !status.Database.Connected {
		fmt.Printf("\n--- Database ---\n")
		fmt.Printf("Connected: ")
		if status.Database.Connected {
			fmt.Printf("✅ YES\n")
		} else {
			fmt.Printf("❌ NO\n")
		}
		if status.Database.Version != "" {
			fmt.Printf("Version: %s\n", status.Database.Version)
		}
		if status.Database.Connections > 0 {
			fmt.Printf("Active Connections: %d\n", status.Database.Connections)
		}
		if status.Database.ResponseTime > 0 {
			fmt.Printf("Response Time: %v\n", status.Database.ResponseTime)
		}
	}

	// Cache status
	if verbose || !status.Cache.Connected {
		fmt.Printf("\n--- Cache ---\n")
		fmt.Printf("Connected: ")
		if status.Cache.Connected {
			fmt.Printf("✅ YES\n")
		} else {
			fmt.Printf("❌ NO\n")
		}
		if status.Cache.Cluster != "" {
			fmt.Printf("Cluster: %s\n", status.Cache.Cluster)
		}
		if status.Cache.Members > 0 {
			fmt.Printf("Cluster Members: %d\n", status.Cache.Members)
		}
		if status.Cache.ResponseTime > 0 {
			fmt.Printf("Response Time: %v\n", status.Cache.ResponseTime)
		}
	}

	// External services
	if verbose {
		fmt.Printf("\n--- External Services ---\n")
		fmt.Printf("Portfolio Service: ")
		if status.External.PortfolioService.Available {
			fmt.Printf("✅ AVAILABLE")
		} else {
			fmt.Printf("❌ UNAVAILABLE")
		}
		if status.External.PortfolioService.ResponseTime > 0 {
			fmt.Printf(" (%v)", status.External.PortfolioService.ResponseTime)
		}
		fmt.Printf("\n")

		fmt.Printf("Security Service: ")
		if status.External.SecurityService.Available {
			fmt.Printf("✅ AVAILABLE")
		} else {
			fmt.Printf("❌ UNAVAILABLE")
		}
		if status.External.SecurityService.ResponseTime > 0 {
			fmt.Printf(" (%v)", status.External.SecurityService.ResponseTime)
		}
		fmt.Printf("\n")
	}

	// Metrics
	if verbose && (status.Metrics.RequestCount > 0 || status.Metrics.ErrorCount > 0) {
		fmt.Printf("\n--- Metrics ---\n")
		fmt.Printf("Total Requests: %d\n", status.Metrics.RequestCount)
		fmt.Printf("Total Errors: %d\n", status.Metrics.ErrorCount)
		if status.Metrics.RequestCount > 0 {
			errorRate := float64(status.Metrics.ErrorCount) / float64(status.Metrics.RequestCount) * 100
			fmt.Printf("Error Rate: %.2f%%\n", errorRate)
		}
		if status.Metrics.AverageResponse > 0 {
			fmt.Printf("Average Response Time: %v\n", status.Metrics.AverageResponse)
		}
		fmt.Printf("Active Requests: %d\n", status.Metrics.ActiveRequests)
	}

	fmt.Printf("\n")
}
