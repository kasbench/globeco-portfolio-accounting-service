package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Status represents the health status
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDegraded  Status = "degraded"
)

// Check represents a health check
type Check struct {
	Name        string        `json:"name"`
	Status      Status        `json:"status"`
	Message     string        `json:"message,omitempty"`
	Duration    time.Duration `json:"duration"`
	LastChecked time.Time     `json:"last_checked"`
	Error       string        `json:"error,omitempty"`
}

// Response represents the overall health response
type Response struct {
	Status  Status           `json:"status"`
	Checks  map[string]Check `json:"checks"`
	Version string           `json:"version,omitempty"`
	Uptime  time.Duration    `json:"uptime"`
}

// CheckFunc is a function that performs a health check
type CheckFunc func(ctx context.Context) error

// Checker manages health checks
type Checker struct {
	checks    map[string]CheckFunc
	mu        sync.RWMutex
	startTime time.Time
	version   string
}

// NewChecker creates a new health checker
func NewChecker(version string) *Checker {
	return &Checker{
		checks:    make(map[string]CheckFunc),
		startTime: time.Now(),
		version:   version,
	}
}

// AddCheck adds a health check
func (c *Checker) AddCheck(name string, checkFunc CheckFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[name] = checkFunc
}

// RemoveCheck removes a health check
func (c *Checker) RemoveCheck(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.checks, name)
}

// Check performs all health checks
func (c *Checker) Check(ctx context.Context) *Response {
	c.mu.RLock()
	checks := make(map[string]CheckFunc, len(c.checks))
	for name, checkFunc := range c.checks {
		checks[name] = checkFunc
	}
	c.mu.RUnlock()

	results := make(map[string]Check)
	overallStatus := StatusHealthy

	// Run checks concurrently
	var wg sync.WaitGroup
	resultCh := make(chan Check, len(checks))

	for name, checkFunc := range checks {
		wg.Add(1)
		go func(name string, checkFunc CheckFunc) {
			defer wg.Done()

			start := time.Now()
			lastChecked := start

			// Create a timeout context for individual checks
			checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			err := checkFunc(checkCtx)
			duration := time.Since(start)

			check := Check{
				Name:        name,
				Duration:    duration,
				LastChecked: lastChecked,
			}

			if err != nil {
				check.Status = StatusUnhealthy
				check.Error = err.Error()
				check.Message = "Check failed"
			} else {
				check.Status = StatusHealthy
				check.Message = "Check passed"
			}

			resultCh <- check
		}(name, checkFunc)
	}

	// Wait for all checks to complete
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Collect results
	for check := range resultCh {
		results[check.Name] = check
		if check.Status == StatusUnhealthy {
			overallStatus = StatusUnhealthy
		} else if check.Status == StatusDegraded && overallStatus == StatusHealthy {
			overallStatus = StatusDegraded
		}
	}

	return &Response{
		Status:  overallStatus,
		Checks:  results,
		Version: c.version,
		Uptime:  time.Since(c.startTime),
	}
}

// LivenessCheck performs a simple liveness check
func (c *Checker) LivenessCheck(ctx context.Context) *Response {
	return &Response{
		Status:  StatusHealthy,
		Checks:  map[string]Check{},
		Version: c.version,
		Uptime:  time.Since(c.startTime),
	}
}

// ReadinessCheck performs readiness checks (usually subset of health checks)
func (c *Checker) ReadinessCheck(ctx context.Context) *Response {
	// For now, readiness is the same as health
	// In the future, you might want different checks for readiness
	return c.Check(ctx)
}

// HTTPHandler returns an HTTP handler for health checks
func (c *Checker) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Set timeout for the entire health check process
		ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()

		response := c.Check(ctx)

		// Set appropriate HTTP status code
		statusCode := http.StatusOK
		if response.Status == StatusUnhealthy {
			statusCode = http.StatusServiceUnavailable
		} else if response.Status == StatusDegraded {
			statusCode = http.StatusOK // Still return 200 for degraded
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)

		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode health check response", http.StatusInternalServerError)
		}
	}
}

// LivenessHandler returns an HTTP handler for liveness checks
func (c *Checker) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		response := c.LivenessCheck(ctx)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode liveness response", http.StatusInternalServerError)
		}
	}
}

// ReadinessHandler returns an HTTP handler for readiness checks
func (c *Checker) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		response := c.ReadinessCheck(ctx)

		statusCode := http.StatusOK
		if response.Status == StatusUnhealthy {
			statusCode = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)

		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode readiness response", http.StatusInternalServerError)
		}
	}
}

// Common health check functions

// DatabaseCheck creates a database health check
func DatabaseCheck(db interface{ PingContext(context.Context) error }) CheckFunc {
	return func(ctx context.Context) error {
		return db.PingContext(ctx)
	}
}

// CacheCheck creates a cache health check
func CacheCheck(cache interface{ Ping(context.Context) error }) CheckFunc {
	return func(ctx context.Context) error {
		return cache.Ping(ctx)
	}
}

// HTTPServiceCheck creates an HTTP service health check
func HTTPServiceCheck(url string, client *http.Client) CheckFunc {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	return func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to make request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}

		return fmt.Errorf("unhealthy status code: %d", resp.StatusCode)
	}
}
