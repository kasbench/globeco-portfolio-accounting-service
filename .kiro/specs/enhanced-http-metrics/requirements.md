# Requirements Document

## Introduction

This feature implements standardized HTTP request metrics for the portfolio accounting service that can be exported to the OpenTelemetry (Otel) Collector. The implementation must add custom metrics while preserving the existing OpenTelemetry metrics flow and leveraging the already integrated OpenTelemetry libraries in the Go service.

## Requirements

### Requirement 1

**User Story:** As a DevOps engineer, I want to monitor HTTP request counts across all endpoints, so that I can track service usage patterns and identify high-traffic endpoints.

#### Acceptance Criteria

1. WHEN an HTTP request is received THEN the system SHALL increment a counter metric named `http_requests_total`
2. WHEN recording the counter THEN the system SHALL include labels for method, path, and status
3. WHEN the method label is set THEN the system SHALL use uppercase HTTP method names (GET, POST, PUT, DELETE, etc.)
4. WHEN the path label is set THEN the system SHALL use route patterns instead of actual URLs with parameters (e.g., "/api/users/{id}" not "/api/users/123")
5. WHEN the status label is set THEN the system SHALL convert numeric HTTP status codes to strings ("200", "404", "500")

### Requirement 2

**User Story:** As a DevOps engineer, I want to measure HTTP request durations, so that I can monitor service performance and identify slow endpoints.

#### Acceptance Criteria

1. WHEN an HTTP request starts THEN the system SHALL begin timing the request duration
2. WHEN an HTTP request completes THEN the system SHALL record the duration in a histogram metric named `http_request_duration`
3. WHEN recording duration THEN the system SHALL use milliseconds as the base unit for consistency with other microservices
4. WHEN creating the histogram THEN the system SHALL use buckets [5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000] in milliseconds
5. WHEN recording the histogram THEN the system SHALL include the same labels as the counter (method, path, status)
6. WHEN timing requests THEN the system SHALL measure from request entry to response completion with microsecond accuracy

### Requirement 3

**User Story:** As a DevOps engineer, I want to monitor concurrent HTTP requests, so that I can understand service load and detect potential bottlenecks.

#### Acceptance Criteria

1. WHEN an HTTP request begins processing THEN the system SHALL increment a gauge metric named `http_requests_in_flight`
2. WHEN an HTTP request completes processing THEN the system SHALL decrement the gauge metric
3. WHEN recording the gauge THEN the system SHALL not include any labels
4. WHEN a request fails or succeeds THEN the system SHALL still decrement the gauge to maintain accuracy

### Requirement 4

**User Story:** As a system administrator, I want metrics to be available through the existing OpenTelemetry integration, so that they flow to our monitoring infrastructure without additional configuration.

#### Acceptance Criteria

1. WHEN metrics are created THEN the system SHALL register them with the existing OpenTelemetry meter provider
2. WHEN the service starts THEN the system SHALL initialize all metrics to ensure they appear in exports
3. WHEN metrics are exported THEN the system SHALL use the existing OTLP gRPC exporter configuration
4. WHEN integrating with OpenTelemetry THEN the system SHALL not break the existing metrics flow

### Requirement 5

**User Story:** As a developer, I want metrics collection to be implemented as middleware, so that all HTTP endpoints are automatically instrumented without individual endpoint modifications.

#### Acceptance Criteria

1. WHEN implementing metrics collection THEN the system SHALL use HTTP middleware that wraps all endpoints
2. WHEN a request is processed THEN the system SHALL record metrics for ALL HTTP requests including API endpoints, health checks, and error responses
3. WHEN middleware is registered THEN the system SHALL ensure it processes requests before existing middleware
4. WHEN metrics recording fails THEN the system SHALL log the error but continue processing the request
5. WHEN collecting metrics THEN the system SHALL minimize performance overhead

### Requirement 6

**User Story:** As a DevOps engineer, I want metrics to follow OpenTelemetry semantic conventions, so that they integrate seamlessly with our existing monitoring stack.

#### Acceptance Criteria

1. WHEN creating metrics THEN the system SHALL follow OpenTelemetry semantic conventions for HTTP metrics
2. WHEN exporting metrics THEN the system SHALL ensure compatibility with the existing Otel Collector configuration
3. WHEN labeling metrics THEN the system SHALL use consistent label names that may be transformed by the OTel pipeline
4. WHEN the service starts THEN the system SHALL properly register metrics at application startup

### Requirement 7

**User Story:** As a developer, I want comprehensive error handling for metrics collection, so that metrics failures don't impact service functionality.

#### Acceptance Criteria

1. WHEN metric recording encounters an error THEN the system SHALL log the error and continue request processing
2. WHEN metrics initialization fails THEN the system SHALL log the failure but allow the service to start
3. WHEN high-cardinality metrics are detected THEN the system SHALL limit unique endpoint values to prevent memory issues
4. WHEN the OpenTelemetry exporter is unavailable THEN the system SHALL handle the failure gracefully

### Requirement 8

**User Story:** As a QA engineer, I want to validate that metrics are working correctly, so that I can ensure the implementation meets requirements before deployment.

#### Acceptance Criteria

1. WHEN testing the implementation THEN the system SHALL provide a way to verify all three metrics are created and registered
2. WHEN making HTTP requests THEN the system SHALL demonstrate that counters increment correctly
3. WHEN processing requests THEN the system SHALL show that histograms record accurate durations
4. WHEN handling concurrent requests THEN the system SHALL prove that gauges properly track in-flight requests
5. WHEN examining metrics THEN the system SHALL confirm that labels contain correct values