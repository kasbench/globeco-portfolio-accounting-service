-- Create indexes for transactions table

-- Unique index on source_id for idempotency
CREATE UNIQUE INDEX IF NOT EXISTS transaction_source_ndx 
ON transactions (source_id);

-- Index for querying transactions by portfolio
CREATE INDEX IF NOT EXISTS idx_transactions_portfolio_id 
ON transactions (portfolio_id);

-- Index for querying transactions by security
CREATE INDEX IF NOT EXISTS idx_transactions_security_id 
ON transactions (security_id) WHERE security_id IS NOT NULL;

-- Index for querying transactions by date
CREATE INDEX IF NOT EXISTS idx_transactions_date 
ON transactions (transaction_date);

-- Index for querying transactions by status
CREATE INDEX IF NOT EXISTS idx_transactions_status 
ON transactions (status);

-- Composite index for common query patterns
CREATE INDEX IF NOT EXISTS idx_transactions_portfolio_security_date 
ON transactions (portfolio_id, security_id, transaction_date);

-- Index for processing NEW transactions
CREATE INDEX IF NOT EXISTS idx_transactions_status_new 
ON transactions (status) WHERE status = 'NEW';

-- Create indexes for balances table

-- Unique index on portfolio_id, security_id for non-null security_id
-- This enforces that each portfolio can have at most one balance per security
CREATE UNIQUE INDEX IF NOT EXISTS balances_portfolio_security_ndx 
ON balances (portfolio_id, security_id) WHERE security_id IS NOT NULL;

-- Unique index on portfolio_id for cash balances (security_id IS NULL)
-- This enforces that each portfolio can have at most one cash balance
CREATE UNIQUE INDEX IF NOT EXISTS balances_portfolio_cash_ndx 
ON balances (portfolio_id) WHERE security_id IS NULL;

-- Index for querying balances by portfolio
CREATE INDEX IF NOT EXISTS idx_balances_portfolio_id 
ON balances (portfolio_id);

-- Index for querying balances by security
CREATE INDEX IF NOT EXISTS idx_balances_security_id 
ON balances (security_id) WHERE security_id IS NOT NULL;

-- Index for cash balances (security_id IS NULL)
CREATE INDEX IF NOT EXISTS idx_balances_cash 
ON balances (portfolio_id) WHERE security_id IS NULL;

-- Index for last updated timestamp for cache invalidation
CREATE INDEX IF NOT EXISTS idx_balances_last_updated 
ON balances (last_updated);

-- Add comments for index documentation
COMMENT ON INDEX transaction_source_ndx IS 'Unique index on source_id for transaction idempotency';
COMMENT ON INDEX balances_portfolio_security_ndx IS 'Unique index enforcing one balance per portfolio-security pair (non-null securities only)';
COMMENT ON INDEX balances_portfolio_cash_ndx IS 'Unique index enforcing one cash balance per portfolio (security_id IS NULL)';
COMMENT ON INDEX idx_transactions_portfolio_security_date IS 'Composite index for efficient transaction queries';
COMMENT ON INDEX idx_transactions_status_new IS 'Partial index for processing NEW transactions';
COMMENT ON INDEX idx_balances_cash IS 'Partial index for cash balances (security_id IS NULL)'; 