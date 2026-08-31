# GlobeCo Portfolio Accounting Service

[![Go Version](https://img.shields.io/badge/Go-1.23+-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Docker](https://img.shields.io/badge/Docker-Ready-blue.svg)](https://docker.com)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-Ready-blue.svg)](https://kubernetes.io)

Portfolio accounting microservice for the GlobeCo benchmarking suite. This service processes financial transactions and maintains real-time portfolio account balances with comprehensive validation, batch processing capabilities, and integration with external portfolio and security services.

## 🏗️ Architecture Overview

The GlobeCo Portfolio Accounting Service is built using **Clean Architecture** principles with clear separation of concerns:

```
├── cmd/                    # Application entry points
│   ├── server/            # HTTP API server
│   └── cli/               # Command-line interface
├── internal/              # Private application code
│   ├── api/               # HTTP handlers, middleware, routes
│   ├── application/       # Application services, DTOs, mappers
│   ├── domain/            # Business logic, entities, repositories
│   └── infrastructure/    # External integrations (DB, cache, etc.)
├── pkg/                   # Public packages (logger, health, validation)
├── docs/                  # API documentation (Swagger/OpenAPI)
├── deployments/           # Kubernetes manifests
└── migrations/            # Database migrations
```

## ✨ Features

### Core Functionality
- **Transaction Processing**: Create, validate, and process financial transactions
- **Balance Management**: Real-time portfolio balance calculations and updates
- **Batch Operations**: Process large transaction files with error handling
- **Portfolio Summaries**: Aggregate views of portfolio positions and cash balances

### Technical Features
- **RESTful API**: Comprehensive REST API with OpenAPI documentation
- **Data Validation**: Robust input validation with business rule enforcement
- **Optimistic Locking**: Concurrent access control for balance updates
- **Caching**: Distributed caching with Hazelcast for performance
- **Event Streaming**: Kafka integration for transaction events
- **Health Monitoring**: Kubernetes-ready health checks and metrics
- **File Processing**: CSV transaction file import with CLI tools

### Integration
- **External Services**: Portfolio and Security service integration
- **Database**: PostgreSQL with migration support
- **Observability**: Structured logging, Prometheus metrics, distributed tracing
- **Authentication**: API key-based authentication

## 🚀 Quick Start

### Prerequisites
- **Go 1.23+**
- **Docker & Docker Compose**
- **PostgreSQL 15+** (or use Docker Compose)

### 1. Clone and Setup
```bash
git clone https://github.com/kasbench/globeco-portfolio-accounting-service.git
cd globeco-portfolio-accounting-service

# Copy example configuration
cp config.yaml.example config.yaml

# Install dependencies
go mod download
```

### 2. Start with Docker Compose
```bash
# Start full development environment
./scripts/docker-compose-up.sh development

# Or start minimal infrastructure only
./scripts/docker-compose-up.sh infrastructure
```

### 3. Run Database Migrations
```bash
make migrate-up
```

### 4. Start the Service
```bash
# Start HTTP API server
go run cmd/server/main.go

# Or use CLI for file processing
go run cmd/cli/main.go --help
```

### 5. Access API Documentation
- **Swagger UI**: http://localhost:8087/swagger/index.html
- **API Info**: http://localhost:8087/api
- **Health Check**: http://localhost:8087/health

## 📖 API Documentation

The service provides a comprehensive REST API documented with OpenAPI/Swagger. Base URL: `http://localhost:8087`

### Interactive Documentation
Visit **http://localhost:8087/swagger/index.html** for interactive API exploration with try-it-out functionality, request/response examples, and schema documentation.

---

### Common Response Objects

All error responses use this structure:

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error description",
    "details": {},
    "timestamp": "2024-01-15T10:30:00Z",
    "traceId": "optional-trace-id"
  }
}
```

All paginated responses include:

```json
{
  "pagination": {
    "limit": 50,
    "offset": 0,
    "total": 150,
    "hasMore": true,
    "page": 1,
    "totalPages": 3
  }
}
```

---

### Transaction Endpoints

#### GET /api/v1/transactions

List transactions with optional filtering, pagination, and sorting.

**Query Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `portfolio_id` | string | No | — | Filter by portfolio ID (24 characters) |
| `security_id` | string | No | — | Filter by security ID (24 characters). Use `null` for cash transactions |
| `transaction_type` | string | No | — | Filter by type: `BUY`, `SELL`, `SHORT`, `COVER`, `DEP`, `WD`, `IN`, `OUT` |
| `status` | string | No | — | Filter by status: `NEW`, `PROC`, `ERROR`, `FATAL` |
| `transaction_date` | string | No | — | Filter by exact date (format: `YYYY-MM-DD`) |
| `from_date` | string | No | — | Date range start (format: `YYYY-MM-DD`) |
| `to_date` | string | No | — | Date range end (format: `YYYY-MM-DD`) |
| `offset` | int | No | `0` | Pagination offset (min: 0) |
| `limit` | int | No | `50` | Page size (min: 1, max: 1000) |
| `sortby` | string | No | `transaction_date,id` | Comma-separated sort fields: `portfolio_id`, `security_id`, `transaction_date`, `transaction_type`, `status`, `created_at` |

**Response (200 OK):**

```json
{
  "transactions": [
    {
      "id": 1,
      "portfolioId": "PORTFOLIO123456789012345",
      "securityId": "SECURITY123456789012345",
      "sourceId": "EXTERNAL_SYSTEM",
      "status": "PROC",
      "transactionType": "BUY",
      "quantity": "100.00",
      "price": "50.25",
      "transactionDate": "20240130",
      "reprocessingAttempts": 0,
      "version": 1,
      "errorMessage": null
    }
  ],
  "pagination": {
    "limit": 50,
    "offset": 0,
    "total": 1,
    "hasMore": false,
    "page": 1,
    "totalPages": 1
  }
}
```

**Status Codes:**

| Code | Description |
|------|-------------|
| 200 | Successfully retrieved transactions |
| 400 | Invalid filter parameters (`INVALID_FILTER`) |
| 500 | Internal server error (`INTERNAL_ERROR`) |

---

#### GET /api/v1/transaction/{id}

Retrieve a specific transaction by its unique ID.

**Path Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | int64 | Yes | Transaction ID |

**Response (200 OK):**

```json
{
  "id": 1,
  "portfolioId": "PORTFOLIO123456789012345",
  "securityId": "SECURITY123456789012345",
  "sourceId": "EXTERNAL_SYSTEM",
  "status": "PROC",
  "transactionType": "BUY",
  "quantity": "100.00",
  "price": "50.25",
  "transactionDate": "20240130",
  "reprocessingAttempts": 0,
  "version": 1,
  "errorMessage": null
}
```

**Status Codes:**

| Code | Description |
|------|-------------|
| 200 | Successfully retrieved transaction |
| 400 | Missing or invalid transaction ID (`MISSING_ID`, `INVALID_ID`) |
| 404 | Transaction not found (`NOT_FOUND`) |
| 500 | Internal server error (`INTERNAL_ERROR`) |

---

#### POST /api/v1/transactions

Create and process a batch of transactions (1–1000 per request).

**Request Body:** Array of transaction objects

```json
[
  {
    "portfolioId": "PORTFOLIO123456789012345",
    "securityId": "SECURITY123456789012345",
    "sourceId": "EXTERNAL_SYSTEM",
    "transactionType": "BUY",
    "quantity": "100.00",
    "price": "50.25",
    "transactionDate": "20240130"
  }
]
```

**Request Field Validation:**

| Field | Type | Required | Validation |
|-------|------|----------|------------|
| `portfolioId` | string | Yes | Exactly 24 characters |
| `securityId` | string | No | Exactly 24 characters (omit for cash transactions: `DEP`, `WD`) |
| `sourceId` | string | Yes | Max 50 characters |
| `transactionType` | string | Yes | One of: `BUY`, `SELL`, `SHORT`, `COVER`, `DEP`, `WD`, `IN`, `OUT` |
| `quantity` | decimal | Yes | Non-zero value |
| `price` | decimal | Yes | Greater than 0 (cash transactions must use `1.0`) |
| `transactionDate` | string | Yes | Date string (YYYYMMDD format) |

**Transaction Type Definitions:**

| Type | Description | Cash Impact | Security Impact |
|------|-------------|-------------|-----------------|
| `BUY` | Buy security | Decrease cash | Increase long position |
| `SELL` | Sell security | Increase cash | Decrease long position |
| `SHORT` | Short sell | Increase cash | Increase short position |
| `COVER` | Cover short | Decrease cash | Decrease short position |
| `DEP` | Cash deposit | Increase cash | No security (securityId must be null) |
| `WD` | Cash withdrawal | Decrease cash | No security (securityId must be null) |
| `IN` | Transfer in | No cash impact | Increase long position |
| `OUT` | Transfer out | No cash impact | Decrease long position |

**Response (201 Created) — All transactions succeeded:**

```json
{
  "successful": [
    {
      "id": 1,
      "portfolioId": "PORTFOLIO123456789012345",
      "securityId": "SECURITY123456789012345",
      "sourceId": "EXTERNAL_SYSTEM",
      "status": "NEW",
      "transactionType": "BUY",
      "quantity": "100.00",
      "price": "50.25",
      "transactionDate": "20240130",
      "reprocessingAttempts": 0,
      "version": 1,
      "errorMessage": null
    }
  ],
  "failed": [],
  "summary": {
    "totalRequested": 1,
    "successful": 1,
    "failed": 0,
    "successRate": 1.0
  }
}
```

**Response (207 Multi-Status) — Partial failures:**

```json
{
  "successful": [
    { "id": 1, "...": "..." }
  ],
  "failed": [
    {
      "transaction": {
        "portfolioId": "INVALID",
        "securityId": null,
        "sourceId": "SRC",
        "transactionType": "BUY",
        "quantity": "100",
        "price": "50.25",
        "transactionDate": "20240130"
      },
      "errors": [
        {
          "field": "portfolioId",
          "message": "must be exactly 24 characters",
          "value": "INVALID"
        }
      ]
    }
  ],
  "summary": {
    "totalRequested": 2,
    "successful": 1,
    "failed": 1,
    "successRate": 0.5
  }
}
```

**Status Codes:**

| Code | Description |
|------|-------------|
| 201 | All transactions created successfully |
| 207 | Partial success — some transactions failed validation |
| 400 | Invalid JSON body (`INVALID_JSON`), empty batch (`EMPTY_BATCH`), or batch exceeds 1000 (`BATCH_TOO_LARGE`) |
| 500 | Internal server error (`INTERNAL_ERROR`) |

---

### Balance Endpoints

#### GET /api/v1/balances

List portfolio balances with optional filtering, pagination, and sorting.

**Query Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `portfolio_id` | string | No | — | Filter by portfolio ID (24 characters) |
| `security_id` | string | No | — | Filter by security ID (24 characters). Use `null` for cash balances |
| `cash_only` | bool | No | — | Only return cash balances (where securityId is null) |
| `zero_balances_only` | bool | No | — | Only return balances where both long and short are zero |
| `non_zero_balances_only` | bool | No | — | Only return balances with non-zero quantities |
| `last_updated_from` | string | No | — | Filter balances updated on or after this date (format: `YYYY-MM-DD`) |
| `last_updated_to` | string | No | — | Filter balances updated on or before this date (format: `YYYY-MM-DD`) |
| `offset` | int | No | `0` | Pagination offset (min: 0) |
| `limit` | int | No | `50` | Page size (min: 1, max: 1000) |
| `sortby` | string | No | `portfolio_id,security_id` | Comma-separated sort fields: `portfolio_id`, `security_id`, `last_updated`, `quantity_long`, `quantity_short` |

**Response (200 OK):**

```json
{
  "balances": [
    {
      "id": 1,
      "portfolioId": "PORTFOLIO123456789012345",
      "securityId": "SECURITY123456789012345",
      "quantityLong": "1000.00",
      "quantityShort": "0.00",
      "lastUpdated": "2024-01-30T15:30:00Z",
      "version": 3
    },
    {
      "id": 2,
      "portfolioId": "PORTFOLIO123456789012345",
      "securityId": null,
      "quantityLong": "50000.00",
      "quantityShort": "0.00",
      "lastUpdated": "2024-01-30T15:30:00Z",
      "version": 5
    }
  ],
  "pagination": {
    "limit": 50,
    "offset": 0,
    "total": 2,
    "hasMore": false,
    "page": 1,
    "totalPages": 1
  }
}
```

**Notes:**
- A balance with `securityId: null` represents a **cash balance** for the portfolio.
- A balance with a `securityId` value represents a **security position**.
- `quantityLong` represents long positions; `quantityShort` represents short positions.
- `version` is used for optimistic locking on updates.

**Status Codes:**

| Code | Description |
|------|-------------|
| 200 | Successfully retrieved balances |
| 400 | Invalid filter parameters (`INVALID_FILTER`) |
| 500 | Internal server error (`INTERNAL_ERROR`) |

---

#### GET /api/v1/balance/{id}

Retrieve a specific balance by its unique ID.

**Path Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `id` | int64 | Yes | Balance ID |

**Response (200 OK):**

```json
{
  "id": 1,
  "portfolioId": "PORTFOLIO123456789012345",
  "securityId": "SECURITY123456789012345",
  "quantityLong": "1000.00",
  "quantityShort": "0.00",
  "lastUpdated": "2024-01-30T15:30:00Z",
  "version": 3
}
```

**Status Codes:**

| Code | Description |
|------|-------------|
| 200 | Successfully retrieved balance |
| 400 | Missing or invalid balance ID (`MISSING_ID`, `INVALID_ID`) |
| 404 | Balance not found (`NOT_FOUND`) |
| 500 | Internal server error (`INTERNAL_ERROR`) |

---

#### GET /api/v1/portfolios/{portfolioId}/summary

Get a comprehensive summary of a portfolio including cash balance and all security positions.

**Path Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `portfolioId` | string | Yes | Portfolio ID (24 characters) |

**Response (200 OK):**

```json
{
  "portfolioId": "PORTFOLIO123456789012345",
  "cashBalance": "50000.00",
  "securityCount": 3,
  "lastUpdated": "2024-01-30T15:30:00Z",
  "securities": [
    {
      "securityId": "SECURITY_AAPL_67890123456",
      "quantityLong": "500.00",
      "quantityShort": "0.00",
      "netQuantity": "500.00",
      "lastUpdated": "2024-01-30T15:30:00Z"
    },
    {
      "securityId": "SECURITY_TSLA_11223344556",
      "quantityLong": "200.00",
      "quantityShort": "50.00",
      "netQuantity": "150.00",
      "lastUpdated": "2024-01-29T12:00:00Z"
    }
  ]
}
```

**Status Codes:**

| Code | Description |
|------|-------------|
| 200 | Successfully retrieved portfolio summary |
| 400 | Missing portfolio ID (`MISSING_PORTFOLIO_ID`) |
| 404 | Portfolio not found (`NOT_FOUND`) |
| 500 | Internal server error (`INTERNAL_ERROR`) |

---

### Health & Monitoring Endpoints

#### GET /health

Basic health check. Returns service status. Also available at `/api/v1/health`.

**Response (200 OK):**

```json
{
  "status": "healthy",
  "timestamp": "2024-01-30T15:30:00Z",
  "version": "1.0.0",
  "environment": "production",
  "checks": {}
}
```

**Status Codes:**

| Code | Description |
|------|-------------|
| 200 | Service is healthy |

---

#### GET /health/live

Kubernetes liveness probe. Always returns alive if the process is running. Also available at `/api/v1/health/live`.

**Response (200 OK):**

```json
{
  "status": "alive",
  "timestamp": "2024-01-30T15:30:00Z",
  "version": "1.0.0",
  "environment": "production"
}
```

**Status Codes:**

| Code | Description |
|------|-------------|
| 200 | Service process is alive |

---

#### GET /health/ready

Kubernetes readiness probe. Checks connectivity to external dependencies (portfolio service, security service). Also available at `/api/v1/health/ready`.

**Response (200 OK) — Ready:**

```json
{
  "status": "ready",
  "timestamp": "2024-01-30T15:30:00Z",
  "version": "1.0.0",
  "environment": "production",
  "checks": {
    "portfolio_service": { "status": "healthy" },
    "security_service": { "status": "healthy" }
  }
}
```

**Response (503 Service Unavailable) — Not Ready:**

```json
{
  "status": "not_ready",
  "timestamp": "2024-01-30T15:30:00Z",
  "version": "1.0.0",
  "environment": "production",
  "checks": {
    "portfolio_service": { "status": "unhealthy", "error": "connection refused" },
    "security_service": { "status": "healthy" }
  }
}
```

**Status Codes:**

| Code | Description |
|------|-------------|
| 200 | Service is ready to receive traffic |
| 503 | Service is not ready (one or more dependencies unavailable) |

---

#### GET /health/detailed

Detailed health check with dependency timing information. Also available at `/api/v1/health/detailed`.

**Response (200 OK) — Healthy:**

```json
{
  "status": "healthy",
  "timestamp": "2024-01-30T15:30:00Z",
  "version": "1.0.0",
  "environment": "production",
  "checks": {
    "portfolio_service": { "status": "healthy", "checked_at": "2024-01-30T15:30:00Z" },
    "security_service": { "status": "healthy", "checked_at": "2024-01-30T15:30:00Z" }
  }
}
```

**Response (503 Service Unavailable) — Degraded:**

```json
{
  "status": "degraded",
  "timestamp": "2024-01-30T15:30:00Z",
  "version": "1.0.0",
  "environment": "production",
  "checks": {
    "portfolio_service": { "status": "unhealthy", "error": "timeout", "checked_at": "2024-01-30T15:30:00Z" },
    "security_service": { "status": "healthy", "checked_at": "2024-01-30T15:30:00Z" }
  }
}
```

**Status Codes:**

| Code | Description |
|------|-------------|
| 200 | All dependencies healthy |
| 503 | One or more dependencies unhealthy (status: `degraded`) |

---

#### GET /metrics

Prometheus metrics endpoint. Only available when metrics are enabled in configuration.

**Response:** Prometheus text exposition format with metrics including:
- HTTP request duration and count by method, path, and status code
- Transaction processing metrics
- Balance update metrics
- Database connection pool metrics

**Status Codes:**

| Code | Description |
|------|-------------|
| 200 | Metrics returned in Prometheus format |

---

### Documentation Endpoints

#### GET /api

Returns API metadata including service name, version, and links to documentation.

**Response (200 OK):**

```json
{
  "name": "GlobeCo Portfolio Accounting Service API",
  "version": "1.0",
  "description": "Financial transaction processing and portfolio balance management microservice",
  "documentation": {
    "swagger_ui": "/swagger/index.html",
    "openapi_spec": "/swagger/doc.json",
    "redoc": "/redoc"
  },
  "contact": {
    "name": "GlobeCo Support",
    "email": "noah@kasbench.org",
    "url": "https://github.com/kasbench/globeco-portfolio-accounting-service"
  },
  "license": {
    "name": "MIT",
    "url": "https://opensource.org/licenses/MIT"
  }
}
```

---

#### GET /swagger/index.html

Serves the Swagger UI interactive documentation interface.

#### GET /swagger

Redirects (308 Permanent Redirect) to `/swagger/index.html`.

#### GET /openapi.json

Returns the OpenAPI 3.0 specification in JSON format.

#### GET /docs

Redirects (308 Permanent Redirect) to `/swagger/index.html`.

---

### Request/Response Headers

**Request Headers:**

| Header | Description |
|--------|-------------|
| `Content-Type` | `application/json` (required for POST requests) |
| `X-Request-ID` | Optional request tracking ID (auto-generated if not provided) |
| `X-Correlation-ID` | Optional correlation ID for distributed tracing (auto-generated if not provided) |

**Response Headers:**

| Header | Description |
|--------|-------------|
| `Content-Type` | `application/json` |
| `X-Request-ID` | Request tracking ID |
| `X-Correlation-ID` | Correlation ID for distributed tracing |

---

### Domain Reference

**Transaction Statuses:**

| Status | Description | Can Reprocess? |
|--------|-------------|----------------|
| `NEW` | Newly created, pending processing | Yes |
| `PROC` | Successfully processed (final state) | No |
| `ERROR` | Processing failed with recoverable error | Yes |
| `FATAL` | Processing failed with unrecoverable error (final state) | No |

**Pagination Defaults:**
- Default page size: 50
- Maximum page size: 1000
- Maximum batch size for POST: 1000 transactions

## ⚙️ Configuration

Configuration is managed through YAML files and environment variables:

```yaml
# config.yaml
server:
  host: "0.0.0.0"
  port: 8087
  read_timeout: "30s"
  write_timeout: "30s"

database:
  host: "localhost"
  port: 5432
  database: "portfolio_accounting"
  username: "postgres"
  password: "postgres"

cache:
  enabled: true
  cluster_name: "portfolio-cache"
  addresses: ["localhost:5701"]

kafka:
  enabled: true
  brokers: ["localhost:9092"]
  topics:
    transactions: "portfolio.transactions"
```

### Environment Variables
```bash
export DATABASE_HOST=localhost
export DATABASE_PASSWORD=secret
export KAFKA_BROKERS=localhost:9092
export LOG_LEVEL=info
```

## 🔧 Development

### Project Setup
```bash
# Install development tools
make install-tools

# Run tests
make test

# Run with hot reload
make dev

# Build binaries
make build
```

### Available Commands
```bash
make help                 # Show all available commands
make build               # Build server and CLI binaries
make test                # Run all tests
make test-integration    # Run integration tests
make lint                # Run linters
make fmt                 # Format code
make dev                 # Start with hot reload
make migrate-up          # Run database migrations
make migrate-down        # Rollback migrations
make docker-build        # Build Docker images
make generate-docs       # Generate API documentation
```

### Code Generation
```bash
# Generate Swagger documentation
make generate-docs

# Generate mocks
make generate-mocks
```

## 🐳 Docker & Deployment

### Docker Images
The service provides multiple Docker targets:

```bash
# Build production image
docker build --target production -t globeco-portfolio-accounting:latest .

# Build CLI image
docker build --target cli -t globeco-portfolio-accounting-cli:latest .

# Build development image
docker build --target development -t globeco-portfolio-accounting:dev .
```

### Docker Compose Profiles
```bash
# Full development environment
docker-compose --profile development up

# Infrastructure only
docker-compose --profile infrastructure up

# Production-like setup
docker-compose --profile full up
```

### Kubernetes Deployment
```bash
# Deploy to Kubernetes
kubectl apply -f deployments/

# Or use the deployment script
./scripts/k8s-deploy.sh deploy

# Check status
./scripts/k8s-deploy.sh status
```

## 📁 File Processing (CLI)

The CLI tool supports processing transaction files:

### Basic Usage
```bash
# Process transaction file
./cli process --file transactions.csv --portfolio-id PORTFOLIO123

# Validate file without processing
./cli validate --file transactions.csv --strict

# Check service status
./cli status --verbose
```

### CSV Format
```csv
portfolio_id,security_id,transaction_type,quantity,price,transaction_date,source_id
PORTFOLIO123456789012345,SECURITY123456789012345,BUY,100.00,50.25,20240130,EXTERNAL_SYSTEM
PORTFOLIO123456789012345,,DEP,1000.00,,20240130,CASH_DEPOSIT
```

### Processing Options
```bash
# Batch processing with custom settings
./cli process \
  --file large_file.csv \
  --batch-size 500 \
  --workers 4 \
  --sort-by portfolio,date \
  --dry-run
```

## 🧪 Testing

### Test Categories
- **Unit Tests**: Domain models, services, and utilities
- **Integration Tests**: Database operations with TestContainers
- **API Tests**: HTTP endpoint testing
- **Performance Tests**: Load and stress testing

### Running Tests
```bash
# All tests
make test

# Unit tests only
go test ./internal/domain/... ./pkg/...

# Integration tests with TestContainers
go test ./tests/integration/...

# Test coverage
make test-coverage
```

### Test Database
Integration tests use PostgreSQL TestContainers for isolated testing:

```go
// Automatic test database setup
container := postgres.RunContainer(ctx, testcontainers.WithImage("postgres:15-alpine"))
```

## 📊 Monitoring & Observability

### Metrics
- **Prometheus metrics** exposed at `/metrics`
- **Custom business metrics** for transactions and balances
- **HTTP request metrics** with duration and status codes
- **Database connection pool metrics**

### Logging
- **Structured logging** with Zap
- **Correlation IDs** for request tracing
- **Configurable log levels** (debug, info, warn, error)
- **JSON format** for production environments

### Health Checks
- **Basic health**: Service availability
- **Readiness**: Database and cache connectivity  
- **Liveness**: Process health for Kubernetes
- **Detailed health**: Comprehensive dependency status

### Distributed Tracing
- **OpenTelemetry integration** for request tracing
- **Jaeger support** for trace visualization
- **Context propagation** across service boundaries

## 🔒 Security

### Authentication
- **API Key authentication** via `X-API-Key` header
- **Configurable key validation**
- **Request rate limiting**

### Data Validation
- **Comprehensive input validation** using Go struct tags
- **Business rule enforcement** at domain layer
- **SQL injection prevention** with parameterized queries
- **XSS protection** in API responses

### Network Security
- **CORS configuration** for browser security
- **TLS support** for encrypted communication
- **Network policies** in Kubernetes deployment

## 🤝 Contributing

### Development Workflow
1. **Fork** the repository
2. **Create** a feature branch (`git checkout -b feature/amazing-feature`)
3. **Make** your changes following the coding standards
4. **Add** tests for new functionality
5. **Run** tests and linters (`make test lint`)
6. **Commit** your changes (`git commit -m 'Add amazing feature'`)
7. **Push** to your branch (`git push origin feature/amazing-feature`)
8. **Open** a Pull Request

### Coding Standards
- **Go standards**: Follow effective Go practices
- **Clean Architecture**: Maintain layer separation
- **Test coverage**: Aim for >80% coverage
- **Documentation**: Update docs for API changes
- **Commit messages**: Use conventional commits

### Code Review Process
- All changes require **peer review**
- **Automated tests** must pass
- **Security scanning** for vulnerabilities
- **Performance impact** assessment for changes

## 📋 Project Status

### Completed Features ✅
- Core transaction processing engine
- RESTful API with comprehensive endpoints
- Database integration with PostgreSQL
- Distributed caching with Hazelcast
- Batch file processing with CLI
- Docker containerization
- Kubernetes deployment manifests
- Comprehensive test suite
- API documentation with Swagger UI
- Monitoring and health checks

### Roadmap 🚧
- [ ] GraphQL API support
- [ ] Event sourcing implementation
- [ ] Advanced analytics endpoints
- [ ] Multi-tenant support
- [ ] Audit trail enhancements
- [ ] Performance optimizations

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 📞 Support

- **Issues**: [GitHub Issues](https://github.com/kasbench/globeco-portfolio-accounting-service/issues)
- **Documentation**: [API Docs](http://localhost:8087/swagger/index.html)
- **Email**: noah@kasbench.org

## 🙏 Acknowledgments

- Built with [Go](https://golang.org) and [Chi Router](https://go-chi.io)
- Database migrations with [golang-migrate](https://github.com/golang-migrate/migrate)
- API documentation with [Swaggo](https://github.com/swaggo/swag)
- Testing with [Testify](https://github.com/stretchr/testify) and [TestContainers](https://testcontainers.com)
- Caching with [Hazelcast](https://hazelcast.com)
- Monitoring with [Prometheus](https://prometheus.io)

---

**GlobeCo Portfolio Accounting Service** - Powering financial transaction processing for the GlobeCo benchmarking suite. 🚀

### Database Migrations

The service includes automatic database migration functionality:

#### Auto-Migration
- **Enabled by default**: Migrations run automatically on service startup
- **Configuration**: Control via `database.auto_migrate` in config.yaml or `GLOBECO_PA_DATABASE_AUTO_MIGRATE` environment variable
- **Safe**: Uses golang-migrate library with proper error handling
- **Containerized**: Works seamlessly in Docker and Kubernetes deployments

#### Manual Migration Commands (Development)
```bash
make migrate-up          # Run database migrations
make migrate-down        # Rollback migrations
make migrate-create NAME=migration_name  # Create new migration
```

#### Configuration Options
```yaml
database:
  migrations_path: "migrations"          # For local development
  # migrations_path: "/usr/local/share/migrations"  # For Docker containers
  auto_migrate: true                     # Enable/disable auto-migration
```

#### Environment Variables
```bash
# Local development
export GLOBECO_PA_DATABASE_MIGRATIONS_PATH="migrations"
export GLOBECO_PA_DATABASE_AUTO_MIGRATE="true"

# Docker containers
export GLOBECO_PA_DATABASE_MIGRATIONS_PATH="/usr/local/share/migrations"
export GLOBECO_PA_DATABASE_AUTO_MIGRATE="true"
```
