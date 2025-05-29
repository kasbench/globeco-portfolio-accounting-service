# GlobeCo Portfolio Accounting Service

## Background

This document provides requirements for the GlobeCo Portfolio Accounting Service  This service processes transactions and updates portfolio account balances.

This microservice will be deployed on Kubernetes 1.33.

This microservice is part of the GlobeCo suite of applications for benchmarking Kubernetes autoscaling.

- Name of service: Portfolio Accounting Service
- Host: globeco-portfolio-accounting-service
- Port: 8087 
- Author: Noah Krieger 
- Email: noah@kasbench.org
- Organization: KASBench
- Organization Domain: kasbench.org

## Technology

| Technology | Version | Notes |
|---------------------------|----------------|---------------------------------------|
| Go | 1.23.4 | |
| Kafka | 4.0.0 | |
| PostreSQL |17 |
| YugabyteDB |2.25.2 | |
| Hazelcast | 5.0 | Community edition.  Docker Hub hazelcast/hazelcast:latest|
---

This module will initially be developed for PostgreSQL and will be converted to YugabyteDB if necessary for scalability.


These are the Go modules used in other GlobeCo microservices.  To the extent possible, we want to maintain consistency.  **Please move the ones that will be used in this microservice into the table above**

	github.com/go-chi/chi/v5 v5.2.1
	github.com/golang-migrate/migrate/v4 v4.18.3
	github.com/jmoiron/sqlx v1.4.0
	github.com/lib/pq v1.10.9
	github.com/prometheus/client_golang v1.22.0
	github.com/segmentio/kafka-go v0.4.48
	github.com/spf13/viper v1.20.1
	github.com/stretchr/testify v1.10.0
	github.com/testcontainers/testcontainers-go/modules/kafka v0.37.0
	github.com/testcontainers/testcontainers-go/modules/postgres v0.37.0
	go.opentelemetry.io/otel v1.36.0
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.36.0
	go.opentelemetry.io/otel/sdk v1.36.0
	go.opentelemetry.io/otel/trace v1.36.0
	go.uber.org/zap v1.27.0




## Other services

| Name | Host | Port | Description | OpenAPI Schema |
| --- | --- | --- | --- | --- |
| GlobeCo Portfolio Service | globeco-portfolio-service-kafka | 8001 | Manage portfolios| [OpenAPI Spec](portfolio-service-openapi.json)| 
| GlobeCo Security Service | globeco-security-service | 8000 | Manage securities | [OpenAPI Spec](security-service-openapi.json) |


## Caching

- Software: Hazelcast 5
- Docker Hub: hazelcast/hazelcast:latest
- Host: globeco-portfolio-accounting-service-cache
- Port: 5701

## Database

- DBMS: Postgres
- Docker Hub: postgres:latest
- Host: globeco-portfolio-accounting-service-postgresql
- Port: 5432


## Data Model

### Tables

#### Table: `transactions`

| column | datatype | nullability | constraint | comments | 
| --- | --- | --- | --- | --- |
| id | serial | NOT NULL | PRIMARY KEY |
| portfolio_id | char(24) |NOT NULL| 
| security_id | char(24) | NULL | |May only be NULL if transaction_type is DEP or WD |
| source_id | varchar(50) | NOT NULL | | identifier provided by the source system |
| status | char(5)  | NOT NULL | DEFAULT 'NEW' | See validation section for valid values |
| transaction_type | char(5) | NOT NULL | |See validation section for valid values |
| quantity | decimal(18,8) | NOT NULL | | May be positive or negative.  If security_id is null, this amount represents cash |
| price | decimal(18,8) | NOT NULL | |For cash transactions, price should be 1.0 |
| transaction_date | date | NOT NULL | DEFAULT CURRENT DATE | |
| reprocessing_attempts | integer | NULL | DEFAULT 0 | 
| version | integer | NOT NULL | DEFAULT 1 | optimistic concurrency


---




#### Table `balances`

| column | datatype | nullability | constraint | comments | 
| --- | --- | --- | --- | --- |
| id | serial | NOT NULL | PRIMARY KEY | |
| portfolio_id | char(24) | NOT NULL | | |
| security_id | char(24) | NULL | | Each portfolio may have at most one balance with a null security_id.  That balance represents cash |
| quantity_long | decimal(18,8) | NOT NULL | DEFAULT 0 |  May be positive, negative, or zero.  Cash is always considered long.
| quantity_short | decimal(18,8) | NOT NULL | DEFAULT 0 |  May be positive, negative, or zero
| last_updated | timestamptz | NOT NULL | DEFAULT CURRENT TIMESTAMP | |
| version | integer | NOT NULL | DEFAULT 1 | optimistic concurrency |



### Indexes

| table | columns | name | uniqueness |
| --- | --- | --- | --- |
| transactions | source_id | transaction_source_ndx | UNIQUE |
| balances | portfolio_id, security_id | trasaction_portfolio_security_ndx |UNIQUE.  NULLS NOT DISTINCT|




### Validation


### Transaction Type Validation
Valid values for `transaction_type` and their impact on units and cash

| transaction_type | Long Units | Short Units | Cash |
| --- | --- | --- | --- |
| BUY | + | N/A | - |
| SELL | - | N/A | + |
| SHORT | N/A | + | + |
| COVER | N/A | - | - |
| DEP | N/A | N/A | + |
| WD | N/A | N/A | - |
| IN | + | N/A | N/A |
| OUT | - | N/A | N/A |


### Status Validation 

Valid values for `status`

| value | definition |
| --- | --- |
| NEW | Initial status |
| PROC| Processed.  Balance Updated |
| FATAL | Fatal error.  Cannot be re-reprocessed |
| ERROR | Non-fatal error.  Attempt re-processing |


## Transaction File Layout

- Comma separated (CSV) file

Layout:
| column | datatype | Required | comments | 
| --- | --- | --- | --- | 
| portfolio_id | String |Yes| 24 character length
| security_id | String | No | Blank for cash.  24 character length.
| source_id | String | Yes |  identifier provided by the source system.  Max length 50 chars. 
| transaction_type | String | Yes | See validation section for valid values |
| quantity | Numeric | Yes |  May be positive or negative.  If security_id is null, this amount represents cash |
| price | Numeric | Yes | For cash transactions, price should be 1.0 |
| transaction_date | Date | Yes | YYYYMMDD format |
| error_message | String | No | Will be added by the transaction processor if any errors.





## General Requirements
- Service should be robustly idempotent.  If duplicate records are received, it should log the event and return an appropriate response, but it should not be treated as a fatal condition.
- Service should feature retries and circuit breakers for the calls to portfolio service and security service.
- Log API calls for easier debugging.  Include URL, request payload, HTTP code, and response
- APIs should support paging.


## Testing Requirements
- Test containers for Postgres and Hazelcast
- Unit and integration tests for all layers

## DTOs

__NOTE:__ Please modify these DTOs, as required, to support paging.


### TransactionResponseDTO

| API field | database column | datatype | required | comments | 
| --- | --- | --- | --- | --- |
| id | id | Integer | Yes | Transaction Id 
| portfolioId |portfolio_id | String |Yes| 24 character length
| securityId |security_id | String | No | Blank for cash.  24 character length.
| sourceId |source_id | String | Yes |  identifier provided by the source system.  Max length 50 chars. 
| status | status | String | Yes |
| transactionType | transaction_type | String | Yes | See validation section for valid values |
| quantity | quantity | Numeric | Yes |  May be positive, negative, or zero.  If security_id is null, this amount represents cash |
| price | price | Numeric | Yes | For cash transactions, price should be 1.0 |
| transactionDate | transaction_date | date | Yes | YYYYMMDD format |
| reprocessingAttempts | reprocessing_attempts | Integer | No | If null, return 0
| version | version | Integer | Yes |
| errorMessage |  |  String | No | Error message if the record hits an error and cannot be processed |

### TransactionPostDTO

| API field | database column | datatype | required | comments | 
| --- | --- | --- | --- | --- |
| portfolioId |portfolio_id | String |Yes| 24 character length
| securityId |security_id | String | No | Blank for cash.  24 character length.
| sourceId |source_id | String | Yes |  identifier provided by the source system.  Max length 50 chars. 
| transactionType | transaction_type | String | Yes | See validation section for valid values |
| quantity | quantity | Numeric | Yes |  May be positive, negative, or zero.  If security_id is null, this amount represents cash |
| price | price | Numeric | Yes | For cash transactions, price should be 1.0 |
| transactionDate | transaction_date | date | Yes | YYYYMMDD format |


### BalanceDTO

| API field | column | datatype | required | comments| 
| --- | --- | --- | --- | --- |
| id | id | Integer | Yes | 
| portfolioId | portfolio_id | String | Yes
| securityId | security_id | String | No |  Null is cash
| quantityLong | quantity_long | Numeric | Yes |
| quantityShort |quantity_short | Numeric | Yes |
| lastUpdated | last_updated | String | Yes |  Format like "2025-05-29T14:29:14.541Z"
| version | version | Integer |Yes | 


### TransactionPostDTO


## APIs


Prefix: api/v1

| Verb |URI | Query Parameters | Request DTO | Response DTO | Description | 
| --- | --- | --- | --- | --- | --- |
| GET | transactions | portfolio_id={portfolio_id}, security_id={security_id}, transaction_date={date}, transaction_type={transaction_type}, status={status}, offset={offset} sortby={list of fields} | | [TransactionResponseDTO] |Get first 50 transactions.  All query parameters are optional.  Parameters are combined using an "and." Offset is is used for paging.  Query parameter sortby is a comma separated list of fields and may include portfolio_id, security_id, transaction_date, transaction_type, and status. |
| GET | transaction/{id} || | TransactionResponseDTO| Gets specific transactions.id
| POST | transactions | | [TransactionPostDTO] | [TransactionResponseDTO] | Post a list of transactions
| GET | balances | portfolio_id={portfolio_id}, security_id={security_id}, offset={offset} sortby={list of fields} | | [BalanceDTO] |Get first 50 balances.  All query parameters are optional.  Parameters are combined using an "and." Offset is is used for paging.  Query parameter sortby is a comma separated list of fields and may include portfolio_id and/or security_id  |
| GET | balance/{id} || | BalanceDTO| Gets specific balances.id

__NOTE__: Transactions are immutable, so no PUT or DELETE.  To fix a transaction, input an offsetting transaction.



## Processing Requirements

### General Notes
- Transactions may be processed via file import initiated through a command line interface (CLI) or by API
- The API should allow multiple transactions to be processed at once.
- Processing transactions is a two step process.  First transactions are saved to the `transactions` table with a status of NEW.  Then they are used to update the `balances` table, whereupon status becomes PROC.  Updating the balance table and transaction status must be done within a database transaction.
- The ### Transaction Type Validation section has a critical table.  This table shows how different transaction types effect the balance.  There are unit balances (long and short) and cash.  If there is a plus sign in a column, it means that the quantity of the transaction increases that balance; a negative sign implies a decrease.  N/A means no impact to that balance.  Note that the balance record has quantity_long and quantity_short fields, corresponding to long and short balances, respetively.  Cash is always considered long.
- In the balances table, there may be at most one record for each portfolio_id with a NULL security_id.  That record represents the cash.
- Many transactions impact two balances: a security balance (security_id is not null) and cash (security_id is null).  Those updates must be done within a transaction.

### Processing Requirement

__NOTE__: Steps 2 through 7 must be performed in a single transaction.

For each transaction record:
1. Validate the fields
2. Lookup the accounting impact of the transaction type in the table in ### Transaction Type Validation.
3. Lookup the balance record for the portfolio_id and security_id.  If it doesn't exist, create it.
4. Update the balance record based on the accounting impact.
    - If Long Units is +, increase balances.quantity_long by the amount of the API quantity field
    - If long Units is -, decrease balances.quantity_long by the amount of the API quantity field
    - If Short Units is +, increase balances.quantity_short by the amount of the API quantity field
    - If Short Units is -, decrease balances.quantity_short by the amount of the API quantity field
5. Lookup the cash record for the portfolio_id and null security_id.  If it doesn't exist, create it.
6. Update the cash record based on the accounting impact.
    - If Cash is +, increase balances.quantity_long by the amount of the API quantity field
    - If Cash is -, decrease balances.quantity_long by the amount of the API quantity field
7. Update transactions.status to "PROC".
8. If udating is not successful, value an appropriate error message in the errorMessage API field.  If the transaction record has been saved, set the status to FATAL or ERROR for non-recoverable and recoverable errors, respectively.



### File Processing Requirements

1. Sort file by portfolio_id, transaction_date, and transaction_type to a temporary file.
2. Iterate through the file, converting records in the file to a list of TransactionPostDTO
3. Whenever you come to a new portfolio_id, send the list of TransactionDTOs to the POST api/v1/transactions API.  
4. Review TransactionResponseDTO.  If there are any errors, create a new file containing just the errors in the same format.  Value the error_message field of the CSV with the errorMessage from the API.  Give the new file the same name as the original file, but append "-errors" to the base of the name.  For example, the failed transactions for file "transactions.csv" would go in a new file named "transactions-errors.csv".
4, Clear the list of TransactionDTOs and continue iterating at step 2



