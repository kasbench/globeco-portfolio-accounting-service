# Implementation Plan

- [x] 1. Create core enhanced metrics middleware with OpenTelemetry integration
  - Implement `EnhancedMetricsMiddleware` struct with all three required metrics (counter, histogram, gauge)
  - Initialize OpenTelemetry metrics using existing meter provider from `otel.GetMeterProvider()`
  - Create middleware handler function that records metrics for every HTTP request
  - Implement path pattern extraction to normalize routes (e.g., `/api/v1/transaction/{id}`)
  - Use milliseconds for histogram duration with buckets [5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000]
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 3.1, 3.2, 3.3, 3.4_

- [x] 2. Integrate enhanced metrics middleware into router configuration
  - Modify `internal/api/routes/routes.go` to include enhanced metrics middleware in the middleware stack
  - Add configuration option to enable/disable enhanced metrics
  - Ensure middleware runs before existing middleware to capture all requests
  - Position middleware after recovery but before logging for proper error handling
  - _Requirements: 4.1, 4.2, 4.3, 4.4, 5.1, 5.2, 5.3, 5.4, 5.5_

- [ ] 3. Implement comprehensive error handling and logging
  - Add graceful error handling for metric recording failures
  - Implement safe metric recording that doesn't block request processing
  - Add proper logging for initialization and runtime errors
  - Ensure service continues operating even if metrics fail
  - _Requirements: 7.1, 7.2, 7.3, 7.4_

- [ ] 4. Create unit tests for enhanced metrics middleware
  - Test metric initialization and registration with OpenTelemetry
  - Verify counter increments correctly for each HTTP request
  - Test histogram records accurate request durations in milliseconds
  - Validate gauge properly tracks concurrent requests (increment/decrement)
  - Test path pattern extraction for all endpoint types
  - Verify correct label values (method, path, status)
  - Test error handling scenarios and graceful degradation
  - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5_

- [ ] 5. Create integration tests for end-to-end metrics validation
  - Test metrics flow from HTTP requests to OpenTelemetry export
  - Verify metrics appear correctly in OTLP format
  - Test concurrent request handling and in-flight gauge accuracy
  - Validate metrics work with existing OpenTelemetry configuration
  - Test coexistence with existing Prometheus metrics
  - _Requirements: 4.1, 4.2, 4.3, 6.1, 6.2, 6.3, 6.4_

- [ ] 6. Add configuration management and service initialization
  - Add enhanced metrics configuration to config struct
  - Update server initialization to configure enhanced metrics
  - Ensure proper initialization order with OpenTelemetry setup
  - Add feature flag support for enabling/disabling enhanced metrics
  - _Requirements: 4.1, 4.2, 6.4_

- [ ] 7. Performance optimization and validation
  - Optimize middleware for minimal performance overhead
  - Implement efficient label creation and path pattern caching
  - Add performance benchmarks to measure middleware impact
  - Validate memory usage and prevent metric cardinality explosion
  - _Requirements: 5.5, 7.3_

- [ ] 8. Documentation and deployment preparation
  - Update README with enhanced metrics information
  - Document configuration options and usage
  - Create example Grafana queries for the new metrics
  - Validate metrics export to OpenTelemetry Collector in test environment
  - _Requirements: 6.1, 6.2, 6.3_