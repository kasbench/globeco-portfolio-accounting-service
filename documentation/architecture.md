# GlobeCo Portfolio Accounting Service - Technical Architecture

## Architecture Overview

The Portfolio Accounting Service follows **Clean Architecture** principles with **Domain-Driven Design (DDD)** patterns. The service is designed as a microservice with clear separation of concerns, ensuring maintainability, testability, and scalability.

### Architecture Principles

1. **Dependency Inversion** - High-level modules don't depend on low-level modules
2. **Single Responsibility** - Each layer has a single, well-defined purpose
3. **Interface Segregation** - Clean contracts between layers
4. **Domain-Centric Design** - Business logic isolated from infrastructure concerns
5. **Idempotency** - Safe to retry operations without side effects

## Project Structure

```
globeco-portfolio-accounting-service/
├── cmd/
│   ├── server/          # HTTP server entry point
│   └── cli/             # CLI for file processing
├── internal/
│   ├── api/             # HTTP handlers and routing
│   │   ├── handlers/    # HTTP request handlers
│   │   ├── middleware/  # HTTP middleware
│   │   └── routes/      # Route definitions
│   ├── domain/          # Business domain layer
│   │   ├── models/      # Domain entities
│   │   ├── services/    # Domain services
│   │   └── repositories/ # Repository interfaces
│   ├── infrastructure/  # External concerns
│   │   ├── database/    # Database implementation
│   │   ├── cache/       # Caching implementation
│   │   ├── kafka/       # Message broker
│   │   └── external/    # External service clients
│   ├── application/     # Application services
│   │   ├── dto/         # Data Transfer Objects
│   │   ├── services/    # Application services
│   │   └── mappers/     # DTO/Domain mapping
│   └── config/          # Configuration management
├── pkg/                 # Public packages
│   ├── logger/          # Logging utilities
│   ├── metrics/         # Metrics collection
│   ├── health/          # Health check utilities
│   └── validation/      # Validation utilities
├── migrations/          # Database migrations
├── deployments/         # Kubernetes manifests
├── scripts/            # Build and deployment scripts
└── tests/              # Integration tests
```

## Layer Architecture

### 1. Presentation Layer (`internal/api`)

**Responsibility:** Handle HTTP requests, routing, and response formatting

#### Components:
- **Handlers** - Process HTTP requests and responses
- **Middleware** - Cross-cutting concerns (logging, metrics, CORS)
- **Routes** - URL routing configuration
- **DTOs** - Data transfer objects for API contracts

#### Key Technologies:
- Chi router for HTTP routing
- Custom middleware for observability
- JSON serialization/deserialization

```go
// Example handler structure
type TransactionHandler struct {
    transactionService application.TransactionService
    logger            logger.Logger
}

func (h *TransactionHandler) CreateTransactions(w http.ResponseWriter, r *http.Request) {
    // Request handling logic
}
```

### 2. Application Layer (`internal/application`)

**Responsibility:** Orchestrate business operations and coordinate between layers

#### Components:
- **Services** - Application-specific business logic
- **DTOs** - Data transfer objects
- **Mappers** - Convert between DTOs and domain models
- **Validators** - Input validation logic

#### Key Patterns:
- Service pattern for business orchestration
- DTO pattern for data contracts
- Mapper pattern for object transformation

```go
type TransactionService interface {
    CreateTransactions(ctx context.Context, transactions []dto.TransactionPostDTO) ([]dto.TransactionResponseDTO, error)
    GetTransactions(ctx context.Context, filter dto.TransactionFilter) ([]dto.TransactionResponseDTO, error)
    ProcessTransactionFile(ctx context.Context, filename string) error
}
```

### 3. Domain Layer (`internal/domain`)

**Responsibility:** Core business logic and rules

#### Components:
- **Entities** - Core business objects with identity
- **Value Objects** - Immutable objects representing concepts
- **Domain Services** - Business logic that doesn't belong to entities
- **Repository Interfaces** - Data access contracts
- **Aggregates** - Consistency boundaries

#### Key Entities:
```go
type Transaction struct {
    ID                   int64
    PortfolioID         string
    SecurityID          *string
    SourceID            string
    Status              TransactionStatus
    TransactionType     TransactionType
    Quantity            decimal.Decimal
    Price               decimal.Decimal
    TransactionDate     time.Time
    ReprocessingAttempts int
    Version             int
    ErrorMessage        *string
}

type Balance struct {
    ID            int64
    PortfolioID   string
    SecurityID    *string
    QuantityLong  decimal.Decimal
    QuantityShort decimal.Decimal
    LastUpdated   time.Time
    Version       int
}
```

#### Business Rules Engine:
```go
type TransactionProcessor interface {
    ProcessTransaction(ctx context.Context, transaction *Transaction) error
    CalculateBalanceImpact(transactionType TransactionType) BalanceImpact
}
```

### 4. Infrastructure Layer (`internal/infrastructure`)

**Responsibility:** External system integration and technical concerns

#### Components:
- **Database** - PostgreSQL implementation
- **Cache** - Hazelcast implementation
- **Message Broker** - Kafka integration
- **External Services** - HTTP clients for portfolio/security services
- **Migrations** - Database schema management

#### Database Layer:
```go
type PostgreSQLTransactionRepository struct {
    db     *sqlx.DB
    cache  cache.Cache
    logger logger.Logger
}

func (r *PostgreSQLTransactionRepository) Create(ctx context.Context, transaction *domain.Transaction) error {
    // Database implementation
}
```

## Data Flow Architecture

### 1. Transaction Processing Flow

```
HTTP Request → Handler → Application Service → Domain Service → Repository → Database
                ↓
         Response ← DTO Mapper ← Domain Entity ← Database Result
```

### 2. File Processing Flow

```
CLI → File Reader → CSV Parser → Batch Processor → Transaction Service → Database
                                       ↓
                              Error Handler → Error File Writer
```

### 3. Caching Strategy

```
Request → Check Cache → Cache Hit? → Return Cached Data
              ↓
         Cache Miss → Database → Update Cache → Return Data
```

## Component Integration

### Database Integration

- **Connection Pool** - Managed connection pooling with sqlx
- **Transaction Management** - Database transactions for consistency
- **Migration Management** - Automated schema versioning
- **Optimistic Locking** - Version-based concurrency control

### Caching Strategy

- **Cache-Aside Pattern** - Application manages cache
- **TTL-Based Expiration** - Time-based cache invalidation
- **Distributed Caching** - Hazelcast cluster support
- **Cache Keys** - Hierarchical key structure

### Message Processing

- **Kafka Integration** - Event publishing for transaction events
- **Async Processing** - Non-blocking message handling
- **Error Handling** - Dead letter queue for failed messages
- **Idempotency** - Duplicate message handling

### External Service Integration

- **Circuit Breaker Pattern** - Fault tolerance for external calls
- **Retry Logic** - Exponential backoff strategies
- **Timeout Management** - Configurable request timeouts
- **Service Discovery** - Kubernetes-based service resolution

## Security Architecture

### Authentication & Authorization
- Service-to-service authentication via API keys
- Request validation and sanitization
- CORS configuration for web clients

### Data Protection
- SQL injection prevention via parameterized queries
- Input validation at multiple layers
- Sensitive data logging restrictions

### Network Security
- TLS encryption for external communications
- Internal service mesh security (future enhancement)

## Observability Architecture

### Logging
- **Structured Logging** - JSON format with Zap
- **Correlation IDs** - Request tracing across services
- **Log Levels** - Configurable verbosity
- **Log Aggregation** - Centralized log collection

### Metrics
- **Prometheus Integration** - Custom business metrics
- **Performance Metrics** - Response times, throughput
- **Error Metrics** - Error rates and types
- **Business Metrics** - Transaction counts, balance changes

### Tracing
- **OpenTelemetry** - Distributed tracing
- **Span Instrumentation** - Key operation tracking
- **Trace Sampling** - Configurable sampling rates

### Health Checks
- **Liveness Probe** - Service availability
- **Readiness Probe** - Service readiness
- **Dependency Checks** - Database/cache connectivity

## Scalability Architecture

### Horizontal Scaling
- **Stateless Design** - No server-side session state
- **Load Balancing** - Kubernetes service load balancing
- **Auto-scaling** - HPA based on CPU/memory metrics

### Database Scaling
- **Read Replicas** - Read traffic distribution
- **Connection Pooling** - Efficient connection management
- **YugabyteDB Migration** - Horizontal scaling option

### Caching Strategy
- **Distributed Cache** - Hazelcast cluster
- **Cache Partitioning** - Data distribution across nodes
- **Cache Replication** - High availability

## Error Handling Architecture

### Error Categories
1. **Validation Errors** - Input validation failures
2. **Business Logic Errors** - Domain rule violations
3. **Infrastructure Errors** - Database/network failures
4. **External Service Errors** - Dependency failures

### Error Handling Strategy
- **Error Wrapping** - Contextual error information
- **Error Classification** - Recoverable vs non-recoverable
- **Retry Logic** - Exponential backoff for transient errors
- **Circuit Breakers** - Fail-fast for degraded services

### Error Response Format
```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid transaction type",
    "details": {
      "field": "transactionType",
      "value": "INVALID"
    },
    "timestamp": "2024-12-19T10:30:00Z",
    "traceId": "abc123"
  }
}
```

## Performance Considerations

### Database Optimization
- **Indexing Strategy** - Optimized for query patterns
- **Connection Pooling** - Configurable pool sizes
- **Query Optimization** - Efficient SQL queries
- **Batch Processing** - Bulk operations for file processing

### Caching Strategy
- **Cache Hit Ratio** - Target 80%+ hit ratio
- **Cache Warming** - Proactive cache population
- **Cache Eviction** - LRU and TTL-based policies

### Concurrency
- **Goroutine Pools** - Bounded concurrency
- **Context Cancellation** - Request timeout handling
- **Rate Limiting** - Request throttling (future enhancement)
