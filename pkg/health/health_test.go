package health

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStatus_String(t *testing.T) {
	testCases := []struct {
		status   Status
		expected string
	}{
		{StatusHealthy, "healthy"},
		{StatusUnhealthy, "unhealthy"},
		{StatusDegraded, "degraded"},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			assert.Equal(t, tc.expected, string(tc.status))
		})
	}
}

func TestNewChecker(t *testing.T) {
	t.Run("Create new checker with version", func(t *testing.T) {
		version := "1.0.0"
		checker := NewChecker(version)

		assert.NotNil(t, checker)
		assert.Equal(t, version, checker.version)
		assert.NotNil(t, checker.checks)
		assert.Len(t, checker.checks, 0)
	})
}

func TestChecker_AddCheck(t *testing.T) {
	t.Run("Add single check", func(t *testing.T) {
		checker := NewChecker("1.0.0")

		checkFunc := func(ctx context.Context) error {
			return nil
		}

		checker.AddCheck("database", checkFunc)

		assert.Len(t, checker.checks, 1)
		assert.Contains(t, checker.checks, "database")
	})

	t.Run("Add multiple checks", func(t *testing.T) {
		checker := NewChecker("1.0.0")

		checkFunc1 := func(ctx context.Context) error { return nil }
		checkFunc2 := func(ctx context.Context) error { return nil }

		checker.AddCheck("database", checkFunc1)
		checker.AddCheck("cache", checkFunc2)

		assert.Len(t, checker.checks, 2)
		assert.Contains(t, checker.checks, "database")
		assert.Contains(t, checker.checks, "cache")
	})
}

func TestChecker_RemoveCheck(t *testing.T) {
	t.Run("Remove existing check", func(t *testing.T) {
		checker := NewChecker("1.0.0")

		checkFunc := func(ctx context.Context) error { return nil }
		checker.AddCheck("database", checkFunc)

		assert.Len(t, checker.checks, 1)

		checker.RemoveCheck("database")

		assert.Len(t, checker.checks, 0)
		assert.NotContains(t, checker.checks, "database")
	})

	t.Run("Remove non-existent check", func(t *testing.T) {
		checker := NewChecker("1.0.0")

		// Should not panic
		assert.NotPanics(t, func() {
			checker.RemoveCheck("non-existent")
		})
	})
}

func TestChecker_Check(t *testing.T) {
	t.Run("All checks pass", func(t *testing.T) {
		checker := NewChecker("1.0.0")

		checker.AddCheck("database", func(ctx context.Context) error {
			return nil
		})
		checker.AddCheck("cache", func(ctx context.Context) error {
			return nil
		})

		ctx := context.Background()
		response := checker.Check(ctx)

		assert.NotNil(t, response)
		assert.Equal(t, StatusHealthy, response.Status)
		assert.Equal(t, "1.0.0", response.Version)
		assert.Len(t, response.Checks, 2)
		assert.Greater(t, response.Uptime.Nanoseconds(), int64(0))

		// Check individual results
		assert.Contains(t, response.Checks, "database")
		assert.Contains(t, response.Checks, "cache")
		assert.Equal(t, StatusHealthy, response.Checks["database"].Status)
		assert.Equal(t, StatusHealthy, response.Checks["cache"].Status)
	})

	t.Run("One check fails", func(t *testing.T) {
		checker := NewChecker("1.0.0")

		checker.AddCheck("database", func(ctx context.Context) error {
			return nil
		})
		checker.AddCheck("cache", func(ctx context.Context) error {
			return errors.New("connection failed")
		})

		ctx := context.Background()
		response := checker.Check(ctx)

		assert.Equal(t, StatusUnhealthy, response.Status)
		assert.Len(t, response.Checks, 2)

		assert.Equal(t, StatusHealthy, response.Checks["database"].Status)
		assert.Equal(t, StatusUnhealthy, response.Checks["cache"].Status)
		assert.Equal(t, "connection failed", response.Checks["cache"].Error)
	})

	t.Run("No checks", func(t *testing.T) {
		checker := NewChecker("1.0.0")

		ctx := context.Background()
		response := checker.Check(ctx)

		assert.Equal(t, StatusHealthy, response.Status)
		assert.Empty(t, response.Checks)
		assert.Equal(t, "1.0.0", response.Version)
	})
}

func TestChecker_LivenessCheck(t *testing.T) {
	t.Run("Liveness check always healthy", func(t *testing.T) {
		checker := NewChecker("1.0.0")

		// Add a failing check
		checker.AddCheck("database", func(ctx context.Context) error {
			return errors.New("database down")
		})

		ctx := context.Background()
		response := checker.LivenessCheck(ctx)

		// Liveness should always be healthy regardless of other checks
		assert.Equal(t, StatusHealthy, response.Status)
		assert.Empty(t, response.Checks)
		assert.Equal(t, "1.0.0", response.Version)
		assert.Greater(t, response.Uptime.Nanoseconds(), int64(0))
	})
}

func TestChecker_ReadinessCheck(t *testing.T) {
	t.Run("Readiness check uses same logic as health check", func(t *testing.T) {
		checker := NewChecker("1.0.0")

		checker.AddCheck("database", func(ctx context.Context) error {
			return errors.New("database not ready")
		})

		ctx := context.Background()
		response := checker.ReadinessCheck(ctx)

		assert.Equal(t, StatusUnhealthy, response.Status)
		assert.Len(t, response.Checks, 1)
		assert.Equal(t, StatusUnhealthy, response.Checks["database"].Status)
	})
}

func TestDatabaseCheck(t *testing.T) {
	t.Run("Database check with successful ping", func(t *testing.T) {
		mockDB := &mockDatabase{pingErr: nil}
		checkFunc := DatabaseCheck(mockDB)

		ctx := context.Background()
		err := checkFunc(ctx)

		assert.NoError(t, err)
	})

	t.Run("Database check with failed ping", func(t *testing.T) {
		mockDB := &mockDatabase{pingErr: errors.New("connection failed")}
		checkFunc := DatabaseCheck(mockDB)

		ctx := context.Background()
		err := checkFunc(ctx)

		assert.Error(t, err)
		assert.Equal(t, "connection failed", err.Error())
	})
}

func TestCacheCheck(t *testing.T) {
	t.Run("Cache check with successful ping", func(t *testing.T) {
		mockCache := &mockCache{pingErr: nil}
		checkFunc := CacheCheck(mockCache)

		ctx := context.Background()
		err := checkFunc(ctx)

		assert.NoError(t, err)
	})

	t.Run("Cache check with failed ping", func(t *testing.T) {
		mockCache := &mockCache{pingErr: errors.New("cache unavailable")}
		checkFunc := CacheCheck(mockCache)

		ctx := context.Background()
		err := checkFunc(ctx)

		assert.Error(t, err)
		assert.Equal(t, "cache unavailable", err.Error())
	})
}

func TestConcurrentChecks(t *testing.T) {
	t.Run("Multiple checks run concurrently", func(t *testing.T) {
		checker := NewChecker("1.0.0")

		// Add checks with delays to test concurrency
		for i := 0; i < 3; i++ {
			name := fmt.Sprintf("check-%d", i)
			checker.AddCheck(name, func(ctx context.Context) error {
				time.Sleep(50 * time.Millisecond)
				return nil
			})
		}

		ctx := context.Background()
		start := time.Now()
		response := checker.Check(ctx)
		duration := time.Since(start)

		// Should complete faster than sequential execution
		// (3 * 50ms = 150ms, but concurrent should be ~50ms + overhead)
		assert.Less(t, duration, 120*time.Millisecond)
		assert.Equal(t, StatusHealthy, response.Status)
		assert.Len(t, response.Checks, 3)
	})
}

func TestCheckTimeout(t *testing.T) {
	t.Run("Check with timeout", func(t *testing.T) {
		checker := NewChecker("1.0.0")

		checker.AddCheck("slow-check", func(ctx context.Context) error {
			select {
			case <-time.After(100 * time.Millisecond):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})

		// Create context with short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		response := checker.Check(ctx)

		// Check should timeout but overall check should still complete
		assert.NotNil(t, response)
		assert.Len(t, response.Checks, 1)
	})
}

// Mock implementations for testing
type mockDatabase struct {
	pingErr error
}

func (m *mockDatabase) PingContext(ctx context.Context) error {
	return m.pingErr
}

type mockCache struct {
	pingErr error
}

func (m *mockCache) Ping(ctx context.Context) error {
	return m.pingErr
}
