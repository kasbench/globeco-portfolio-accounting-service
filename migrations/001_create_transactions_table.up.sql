-- Create transactions table
CREATE TABLE IF NOT EXISTS transactions (
    id SERIAL PRIMARY KEY,
    portfolio_id CHAR(24) NOT NULL,
    security_id CHAR(24),
    source_id VARCHAR(50) NOT NULL,
    status CHAR(5) NOT NULL DEFAULT 'NEW',
    transaction_type CHAR(5) NOT NULL,
    quantity DECIMAL(18,8) NOT NULL,
    price DECIMAL(18,8) NOT NULL,
    transaction_date DATE NOT NULL DEFAULT CURRENT_DATE,
    reprocessing_attempts INTEGER DEFAULT 0,
    error_message VARCHAR(255),
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Constraints
    CONSTRAINT chk_status CHECK (status IN ('NEW', 'PROC', 'ERROR', 'FATAL')),
    CONSTRAINT chk_transaction_type CHECK (transaction_type IN ('BUY', 'SELL', 'SHORT', 'COVER', 'DEP', 'WD', 'IN', 'OUT')),
    CONSTRAINT chk_portfolio_id_length CHECK (char_length(portfolio_id) = 24),
    CONSTRAINT chk_security_id_length CHECK (security_id IS NULL OR char_length(security_id) = 24),
    CONSTRAINT chk_source_id_not_empty CHECK (char_length(trim(source_id)) > 0),
    CONSTRAINT chk_version_positive CHECK (version > 0),
    CONSTRAINT chk_reprocessing_attempts_non_negative CHECK (reprocessing_attempts >= 0),
    
    -- Business rule constraints
    CONSTRAINT chk_cash_transactions CHECK (
        (transaction_type IN ('DEP', 'WD') AND security_id IS NULL) OR
        (transaction_type NOT IN ('DEP', 'WD') AND security_id IS NOT NULL)
    )
);

-- Add comments for documentation
COMMENT ON TABLE transactions IS 'Portfolio transactions with balance impact tracking';
COMMENT ON COLUMN transactions.id IS 'Unique transaction identifier';
COMMENT ON COLUMN transactions.portfolio_id IS 'Portfolio identifier (24 characters)';
COMMENT ON COLUMN transactions.security_id IS 'Security identifier (24 characters), NULL for cash transactions';
COMMENT ON COLUMN transactions.source_id IS 'Source system identifier (max 50 characters)';
COMMENT ON COLUMN transactions.status IS 'Processing status: NEW, PROC, ERROR, FATAL';
COMMENT ON COLUMN transactions.transaction_type IS 'Transaction type: BUY, SELL, SHORT, COVER, DEP, WD, IN, OUT';
COMMENT ON COLUMN transactions.quantity IS 'Transaction quantity (positive or negative)';
COMMENT ON COLUMN transactions.price IS 'Transaction price (1.0 for cash transactions)';
COMMENT ON COLUMN transactions.transaction_date IS 'Transaction date';
COMMENT ON COLUMN transactions.reprocessing_attempts IS 'Number of reprocessing attempts';
COMMENT ON COLUMN transactions.version IS 'Version for optimistic locking';
COMMENT ON COLUMN transactions.created_at IS 'Record creation timestamp';
COMMENT ON COLUMN transactions.updated_at IS 'Record last update timestamp'; 