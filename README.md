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

The service provides a comprehensive REST API documented with OpenAPI/Swagger:

### Core Endpoints

#### Transactions
- `GET /api/v1/transactions` - List transactions with filtering
- `POST /api/v1/transactions` - Create batch of transactions  
- `GET /api/v1/transaction/{id}` - Get specific transaction

#### Balances
- `GET /api/v1/balances` - List portfolio balances
- `GET /api/v1/balance/{id}` - Get specific balance
- `GET /api/v1/portfolios/{portfolioId}/summary` - Portfolio summary

#### Health & Monitoring
- `GET /health` - Basic health check
- `GET /health/ready` - Kubernetes readiness probe
- `GET /health/live` - Kubernetes liveness probe
- `GET /metrics` - Prometheus metrics

### Interactive Documentation
Visit **http://localhost:8087/swagger/index.html** for interactive API exploration with:
- Try-it-out functionality
- Request/response examples
- Schema documentation
- Authentication testing

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
