# GlobeCo Portfolio Accounting Service - Architecture & Design

## Overview

The GlobeCo Portfolio Accounting Service is a microservice designed for processing financial transactions and maintaining real-time portfolio account balances. Built using **Clean Architecture** principles, it provides comprehensive transaction processing, balance management, and portfolio summaries for the GlobeCo benchmarking suite.

## Architectural Principles

• **Clean Architecture**: Clear separation of concerns across domain, application, infrastructure, and API layers
• **Domain-Driven Design**: Rich domain models with business logic encapsulation
• **Microservice Architecture**: Loosely coupled service with well-defined boundaries
• **Event-Driven Architecture**: Kafka integration for asynchronous event processing
• **Immutable Entities**: Domain objects designed for thread safety and consistency
• **Optimistic Locking**: Concurrent access control for balance updates

## System Architecture

### High-Level Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                        External Clients                         │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │   Web UI    │  │  CLI Tool   │  │   External Systems      │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                    API Gateway / Load Balancer                  │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│              Portfolio Accounting Service (Main)                │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                    API Layer                                ││
│  │  • REST Endpoints  • Middleware  • Request Validation      ││
│  └─────────────────────────────────────────────────────────────┘│
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                Application Layer                            ││
│  │  • Services  • DTOs  • Mappers  • Orchestration           ││
│  └─────────────────────────────────────────────────────────────┘│
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                  Domain Layer                               ││
│  │  • Entities  • Value Objects  • Business Rules            ││
│  └─────────────────────────────────────────────────────────────┘│
│  ┌─────────────────────────────────────────────────────────────┐│
│  │               Infrastructure Layer                          ││
│  │  • Database  • Cache  • External Services  • Messaging    ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
                    │                    │                    │
                    ▼                    ▼                    ▼
┌─────────────┐ ┌─────────────┐ ┌─────────────────────────────────┐
│ PostgreSQL  │ │   Redis     │ │        External Services        │
│  Database   │ │   Cache     │ │  ┌─────────────┐ ┌─────────────┐ │
└─────────────┘ └─────────────┘ │  │ Portfolio   │ │  Security   │ │
                                │  │  Service    │ │  Service    │ │
                                │  └─────────────┘ └─────────────┘ │
                                └─────────────────────────────────┘
                                │
                                ▼
                        ┌─────────────┐
                        │    Kafka    │
                        │ Event Bus   │
                        └─────────────┘
```

## Core Components

### 1. API Layer (`internal/api/`)

• **REST API**: Comprehensive RESTful endpoints with OpenAPI documentation
• **Middleware Stack**: Request ID, CORS, logging, authentication, tracing
• **Route Management**: Versioned API endpoints with health checks
• **Error Handling**: Structured error responses with proper HTTP status codes

**Key Endpoints:**
- `GET/POST /api/v1/transactions` - Transaction management
- `GET /api/v1/balances` - Balance queries
- `GET /api/v1/portfolios/{id}/summary` - Portfolio summaries
- `GET /health/*` - Health check endpoints

### 2. Application Layer (`internal/application/`)

• **Service Orchestration**: Coordinates business operations across domain services
• **Data Transfer Objects**: Clean data contracts between layers
• **Mappers**: Transform between domain models and API representations
• **Validation**: Input validation and business rule enforcement

### 3. Domain Layer (`internal/domain/`)

**Core Entities:**
• **Transaction**: Immutable financial transaction with business rules
• **Balance**: Portfolio position tracking with optimistic locking
• **Value Objects**: PortfolioID, SecurityID, Quantity, Price, Amount

**Business Rules:**
• Cash transactions (DEP/WD) must have empty security ID
• Security transactions require valid security ID
• Quantity cannot be zero, prices must be positive
• Balance impact calculations based on transaction type

### 4. Infrastructure Layer (`internal/infrastructure/`)

• **Database**: PostgreSQL with automatic migrations and connection pooling
• **Caching**: Redis for performance optimization
• **External Services**: HTTP clients with circuit breakers and retry logic
• **Messaging**: Kafka integration for event streaming
• **Observability**: OpenTelemetry tracing and Prometheus metrics

## External Dependencies

### Microservices

• **Portfolio Service** (`globeco-portfolio-service`)
  - Endpoint: `http://globeco-portfolio-service:8001`
  - Purpose: Portfolio metadata and validation
  - Health Check: `/health`

• **Security Service** (`globeco-security-service`)
  - Endpoint: `http://globeco-security-service:8000`
  - Purpose: Security master data and validation
  - Health Check: `/health/liveness`

### Infrastructure Dependencies

• **PostgreSQL 17+**: Primary data store with ACID compliance
• **Redis 7+**: Distributed caching and session storage
• **Apache Kafka**: Event streaming and message processing
• **Hazelcast**: Alternative distributed cache (Docker Compose)

### Go Libraries

**Core Framework:**
- `github.com/go-chi/chi/v5` - HTTP router and middleware
- `github.com/jmoiron/sqlx` - Database operations
- `github.com/lib/pq` - PostgreSQL driver

**Business Logic:**
- `github.com/shopspring/decimal` - Precise decimal arithmetic
- `github.com/google/uuid` - UUID generation

**Infrastructure:**
- `github.com/redis/go-redis/v9` - Redis client
- `github.com/golang-migrate/migrate/v4` - Database migrations
- `go.opentelemetry.io/otel` - Observability and tracing

**Configuration & CLI:**
- `github.com/spf13/viper` - Configuration management
- `github.com/spf13/cobra` - CLI framework

## Data Architecture

### Database Schema

**Core Tables:**
• `transactions` - Financial transaction records
• `balances` - Portfolio position balances
• `schema_migrations` - Database version control

**Key Features:**
• **Optimistic Locking**: Version fields prevent concurrent update conflicts
• **Audit Trail**: Created/updated timestamps on all entities
• **Data Integrity**: Foreign key constraints and check constraints
• **Indexing**: Optimized queries for portfolio and security lookups

### State Management

**Transaction States:**
- `NEW` - Initial state, ready for processing
- `PROC` - Successfully processed
- `ERROR` - Processing failed, can be retried
- `FATAL` - Permanent failure, manual intervention required

**Balance Consistency:**
• Real-time balance updates with transaction processing
• Separate long/short position tracking
• Cash balance management for security transactions
• Atomic updates using database transactions

## Resiliency Features

### Fault Tolerance

• **Circuit Breakers**: Prevent cascade failures to external services
• **Retry Logic**: Exponential backoff for transient failures
• **Timeout Management**: Configurable timeouts for all operations
• **Graceful Degradation**: Service continues with reduced functionality

### Data Consistency

• **Optimistic Locking**: Prevents lost updates in concurrent scenarios
• **Database Transactions**: ACID compliance for critical operations
• **Idempotency**: Safe retry of operations without side effects
• **Event Sourcing**: Kafka events for audit and recovery

### Monitoring & Observability

• **Health Checks**: Kubernetes-ready liveness and readiness probes
• **Metrics**: Prometheus metrics for performance monitoring
• **Distributed Tracing**: OpenTelemetry for request flow tracking
• **Structured Logging**: JSON logs with correlation IDs

### Deployment Resilience

• **Auto-Migration**: Database schema updates on service startup
• **Configuration Management**: Environment-based configuration
• **Container Health**: Docker health checks and restart policies
• **Kubernetes Integration**: StatefulSets for data persistence

## Security Architecture

### Authentication & Authorization

• **API Key Authentication**: X-API-Key header validation
• **Request Rate Limiting**: Protection against abuse
• **CORS Configuration**: Cross-origin request security

### Data Protection

• **Input Validation**: Comprehensive validation at API boundaries
• **SQL Injection Prevention**: Parameterized queries only
• **XSS Protection**: Proper response encoding
• **TLS Support**: Encrypted communication in production

## Performance Characteristics

### Caching Strategy

• **Redis Caching**: External service responses cached for performance
• **Cache-Aside Pattern**: Fallback to direct service calls
• **TTL Management**: Configurable cache expiration

### Scalability

• **Horizontal Scaling**: Stateless service design
• **Connection Pooling**: Efficient database resource usage
• **Batch Processing**: Bulk transaction processing capabilities
• **Async Processing**: Kafka for non-blocking operations

### Resource Management

• **Memory Optimization**: Efficient data structures and GC tuning
• **CPU Efficiency**: Optimized algorithms and concurrent processing
• **I/O Management**: Connection pooling and timeout configuration

## Deployment Architecture

### Container Strategy

• **Multi-stage Builds**: Optimized Docker images
• **Development/Production Targets**: Environment-specific builds
• **CLI Tools**: Separate container for batch processing

### Kubernetes Deployment

• **StatefulSet**: PostgreSQL with persistent storage
• **Deployment**: Stateless application pods
• **Services**: Internal service discovery and load balancing
• **ConfigMaps**: Environment-specific configuration

### Environment Management

• **Development**: Docker Compose with hot reload
• **Testing**: TestContainers for integration tests
• **Production**: Kubernetes with monitoring and alerting

This architecture provides a robust, scalable, and maintainable foundation for financial transaction processing with comprehensive error handling, monitoring, and resiliency features.