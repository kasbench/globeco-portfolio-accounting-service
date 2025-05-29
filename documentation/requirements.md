# GlobeCo Portfolio Accounting Service - Requirements

## Service Overview

**Service Name:** Portfolio Accounting Service  
**Host:** globeco-portfolio-accounting-service  
**Port:** 8087  
**Author:** Noah Krieger (noah@kasbench.org)  
**Organization:** KASBench (kasbench.org)  

### Purpose
This microservice processes financial transactions and maintains portfolio account balances. It serves as part of the GlobeCo suite of applications for benchmarking Kubernetes autoscaling and will be deployed on Kubernetes 1.33.

## Technology Stack

### Core Technologies
| Technology | Version | Purpose |
|------------|---------|---------|
| Go | 1.23.4 | Primary language |
| PostgreSQL | 17 | Primary database |
| YugabyteDB | 2.25.2 | Scalability option |
| Kafka | 4.0.0 | Message streaming |
| Hazelcast | 5.0 Community | Caching |

### Go Dependencies
| Module | Version | Purpose |
|--------|---------|---------|
| github.com/go-chi/chi/v5 | v5.2.1 | HTTP routing |
| github.com/jmoiron/sqlx | v1.4.0 | Database operations |
| github.com/lib/pq | v1.10.9 | PostgreSQL driver |
| github.com/spf13/viper | v1.20.1 | Configuration |
| github.com/stretchr/testify | v1.10.0 | Testing |
| go.uber.org/zap | v1.27.0 | Structured logging |
| github.com/golang-migrate/migrate/v4 | v4.18.3 | Database migrations |
| github.com/prometheus/client_golang | v1.22.0 | Metrics |
| github.com/segmentio/kafka-go | v0.4.48 | Kafka client |
| go.opentelemetry.io/otel | v1.36.0 | Observability |

### Infrastructure Components
| Component | Host | Port | Docker Image |
|-----------|------|------|--------------|
| Cache | globeco-portfolio-accounting-service-cache | 5701 | hazelcast/hazelcast:latest |
| Database | globeco-portfolio-accounting-service-postgresql | 5432 | postgres:latest |

## Data Model

### Transactions Table
| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | serial | PRIMARY KEY | Unique identifier |
| portfolio_id | char(24) | NOT NULL | Portfolio identifier |
| security_id | char(24) | NULL | Security identifier (NULL for cash) |
| source_id | varchar(50) | NOT NULL, UNIQUE | Source system identifier |
| status | char(5) | NOT NULL, DEFAULT 'NEW' | Processing status |
| transaction_type | char(5) | NOT NULL | Transaction type code |
| quantity | decimal(18,8) | NOT NULL | Transaction quantity |
| price | decimal(18,8) | NOT NULL | Transaction price |
| transaction_date | date | NOT NULL, DEFAULT CURRENT_DATE | Transaction date |
| reprocessing_attempts | integer | DEFAULT 0 | Retry count |
| version | integer | NOT NULL, DEFAULT 1 | Optimistic locking |

### Balances Table
| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | serial | PRIMARY KEY | Unique identifier |
| portfolio_id | char(24) | NOT NULL | Portfolio identifier |
| security_id | char(24) | NULL | Security identifier (NULL for cash) |
| quantity_long | decimal(18,8) | NOT NULL, DEFAULT 0 | Long position quantity |
| quantity_short | decimal(18,8) | NOT NULL, DEFAULT 0 | Short position quantity |
| last_updated | timestamptz | NOT NULL, DEFAULT CURRENT_TIMESTAMP | Last update time |
| version | integer | NOT NULL, DEFAULT 1 | Optimistic locking |

### Indexes
- `transactions.source_id` - UNIQUE
- `balances.(portfolio_id, security_id)` - UNIQUE, NULLS NOT DISTINCT

### Business Rules

#### Transaction Types and Balance Impact
| Type | Long Units | Short Units | Cash | Description |
|------|------------|-------------|------|-------------|
| BUY | + | N/A | - | Buy securities |
| SELL | - | N/A | + | Sell securities |
| SHORT | N/A | + | + | Short sell |
| COVER | N/A | - | - | Cover short position |
| DEP | N/A | N/A | + | Cash deposit |
| WD | N/A | N/A | - | Cash withdrawal |
| IN | + | N/A | N/A | Securities transfer in |
| OUT | - | N/A | N/A | Securities transfer out |

#### Status Values
| Status | Description |
|--------|-------------|
| NEW | Initial status |
| PROC | Processed successfully |
| ERROR | Recoverable error |
| FATAL | Non-recoverable error |

## API Specification

### Base Path
`/api/v1`

### Endpoints

#### Transactions
| Method | Path | Parameters | Request | Response | Description |
|--------|------|------------|---------|----------|-------------|
| GET | /transactions | portfolio_id, security_id, transaction_date, transaction_type, status, offset, sortby | - | TransactionResponseDTO[] | List transactions (paginated) |
| GET | /transaction/{id} | - | - | TransactionResponseDTO | Get specific transaction |
| POST | /transactions | - | TransactionPostDTO[] | TransactionResponseDTO[] | Create transactions |

#### Balances
| Method | Path | Parameters | Request | Response | Description |
|--------|------|------------|---------|----------|-------------|
| GET | /balances | portfolio_id, security_id, offset, sortby | - | BalanceDTO[] | List balances (paginated) |
| GET | /balance/{id} | - | - | BalanceDTO | Get specific balance |

### Data Transfer Objects

#### TransactionPostDTO
```json
{
  "portfolioId": "string(24)",
  "securityId": "string(24)?",
  "sourceId": "string(50)",
  "transactionType": "string(5)",
  "quantity": "decimal(18,8)",
  "price": "decimal(18,8)",
  "transactionDate": "date(YYYYMMDD)"
}
```

#### TransactionResponseDTO
```json
{
  "id": "integer",
  "portfolioId": "string(24)",
  "securityId": "string(24)?",
  "sourceId": "string(50)",
  "status": "string(5)",
  "transactionType": "string(5)",
  "quantity": "decimal(18,8)",
  "price": "decimal(18,8)",
  "transactionDate": "date(YYYYMMDD)",
  "reprocessingAttempts": "integer",
  "version": "integer",
  "errorMessage": "string?"
}
```

#### BalanceDTO
```json
{
  "id": "integer",
  "portfolioId": "string(24)",
  "securityId": "string(24)?",
  "quantityLong": "decimal(18,8)",
  "quantityShort": "decimal(18,8)",
  "lastUpdated": "string(ISO8601)",
  "version": "integer"
}
```

## Processing Requirements

### Transaction Processing Flow
1. **Validation** - Validate all required fields and business rules
2. **Balance Lookup** - Find or create balance records for security and cash
3. **Balance Update** - Apply transaction impact based on transaction type
4. **Status Update** - Mark transaction as PROC upon successful completion
5. **Error Handling** - Set appropriate status (ERROR/FATAL) on failure

### File Processing Requirements
1. **Sort** - Sort by portfolio_id, transaction_date, transaction_type
2. **Batch** - Group by portfolio_id for API calls
3. **Process** - Send batches to POST /transactions API
4. **Error Handling** - Generate error files for failed transactions

### Transactional Requirements
- Steps 2-7 of transaction processing must be atomic (single database transaction)
- Balance updates for both security and cash must be consistent
- Optimistic locking for concurrent access protection

## Quality Requirements

### Performance
- Support pagination for all list endpoints (50 records default)
- Implement caching with Hazelcast for frequently accessed data

### Reliability
- Robust idempotency - handle duplicate records gracefully
- Circuit breakers and retries for external service calls
- Graceful error handling and logging

### Observability
- Structured logging for all operations
- Prometheus metrics integration
- Distributed tracing with OpenTelemetry
- Health check endpoints

### Testing
- Comprehensive unit tests with high coverage
- Integration tests using TestContainers (Postgres, Hazelcast)
- Test coverage for all layers

### Security
- Input validation on all endpoints
- CORS configuration
- Environment-based configuration management

## External Dependencies

### Services
| Service | Host | Port | Purpose |
|---------|------|------|---------|
| GlobeCo Portfolio Service | globeco-portfolio-service-kafka | 8001 | Portfolio management |
| GlobeCo Security Service | globeco-security-service | 8000 | Security management |

### File Format
CSV files with the following structure:
```
portfolio_id,security_id,source_id,transaction_type,quantity,price,transaction_date,error_message
```

## Deployment Requirements
- Containerized with Docker
- Multi-stage builds for minimal image size
- Kubernetes deployment ready
- Environment-specific configuration
- Graceful shutdown implementation
