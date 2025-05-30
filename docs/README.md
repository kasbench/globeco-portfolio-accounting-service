# GlobeCo Portfolio Accounting Service - API Documentation

This directory contains the auto-generated OpenAPI documentation for the GlobeCo Portfolio Accounting Service.

## Generated Files

- `swagger.json` - OpenAPI 3.0 specification in JSON format
- `swagger.yaml` - OpenAPI 3.0 specification in YAML format  
- `docs.go` - Go package containing embedded OpenAPI specification

## Accessing API Documentation

When the service is running, you can access the interactive API documentation at:

### Swagger UI
- **URL**: `http://localhost:8087/swagger/index.html`
- **Description**: Interactive API documentation with request/response examples
- **Features**: Try-it-out functionality, schema exploration, endpoint testing

### API Information
- **URL**: `http://localhost:8087/api`
- **Description**: Basic API information and documentation links

### OpenAPI Specification
- **URL**: `http://localhost:8087/swagger/doc.json`
- **Description**: Raw OpenAPI 3.0 specification in JSON format

### Alternative Access
- **URL**: `http://localhost:8087/docs` (redirects to Swagger UI)
- **URL**: `http://localhost:8087/swagger` (redirects to Swagger UI)

## API Endpoints Documented

### Transactions
- `GET /api/v1/transactions` - Get transactions with filtering and pagination
- `POST /api/v1/transactions` - Create batch of transactions
- `GET /api/v1/transaction/{id}` - Get specific transaction by ID

### Balances
- `GET /api/v1/balances` - Get balances with filtering and pagination
- `GET /api/v1/balance/{id}` - Get specific balance by ID
- `GET /api/v1/portfolios/{portfolioId}/summary` - Get portfolio summary

### Health Checks
- `GET /health` - Basic health check
- `GET /health/live` - Kubernetes liveness probe
- `GET /health/ready` - Kubernetes readiness probe
- `GET /health/detailed` - Detailed health check with dependencies

## Authentication

The API uses API key authentication via the `X-API-Key` header:

```bash
curl -H "X-API-Key: your-api-key" http://localhost:8087/api/v1/transactions
```

## Regenerating Documentation

To regenerate the OpenAPI documentation after making changes to the API:

```bash
# Install swag tool (if not already installed)
go install github.com/swaggo/swag/cmd/swag@latest

# Generate documentation
swag init -g cmd/server/main.go -o docs
```

## Example Usage

### Get All Transactions
```bash
curl -X GET "http://localhost:8087/api/v1/transactions?limit=10&offset=0" \
  -H "X-API-Key: your-api-key" \
  -H "Accept: application/json"
```

### Create Transactions
```bash
curl -X POST "http://localhost:8087/api/v1/transactions" \
  -H "X-API-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '[{
    "portfolio_id": "PORTFOLIO123456789012345",
    "security_id": "SECURITY123456789012345",
    "transaction_type": "BUY",
    "quantity": "100.00",
    "price": "50.25",
    "transaction_date": "20240130",
    "source_id": "EXTERNAL_SYSTEM"
  }]'
```

### Get Portfolio Summary
```bash
curl -X GET "http://localhost:8087/api/v1/portfolios/PORTFOLIO123456789012345/summary" \
  -H "X-API-Key: your-api-key" \
  -H "Accept: application/json"
```

## Schema Information

The API uses comprehensive data validation with the following key schemas:

- **TransactionPostDTO**: Input schema for creating transactions
- **TransactionResponseDTO**: Output schema for transaction data
- **BalanceDTO**: Schema for balance information
- **PortfolioSummaryDTO**: Schema for portfolio summaries
- **ErrorResponse**: Standardized error response format

For detailed schema information, visit the Swagger UI at `/swagger/index.html` when the service is running. 