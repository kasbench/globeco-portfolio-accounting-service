-- Create balances table
CREATE TABLE IF NOT EXISTS balances (
    id SERIAL PRIMARY KEY,
    portfolio_id CHAR(24) NOT NULL,
    security_id CHAR(24),
    quantity_long DECIMAL(18,8) NOT NULL DEFAULT 0,
    quantity_short DECIMAL(18,8) NOT NULL DEFAULT 0,
    last_updated TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Constraints
    CONSTRAINT chk_portfolio_id_length CHECK (char_length(portfolio_id) = 24),
    CONSTRAINT chk_security_id_length CHECK (security_id IS NULL OR char_length(security_id) = 24),
    CONSTRAINT chk_version_positive CHECK (version > 0),
    
    -- Business rule: each portfolio can have at most one balance with null security_id (cash)
    -- This will be enforced by the unique index in the next migration
    CONSTRAINT chk_cash_is_long_only CHECK (
        security_id IS NOT NULL OR quantity_short = 0
    )
);

-- Add comments for documentation
COMMENT ON TABLE balances IS 'Portfolio balances for securities and cash positions';
COMMENT ON COLUMN balances.id IS 'Unique balance identifier';
COMMENT ON COLUMN balances.portfolio_id IS 'Portfolio identifier (24 characters)';
COMMENT ON COLUMN balances.security_id IS 'Security identifier (24 characters), NULL for cash';
COMMENT ON COLUMN balances.quantity_long IS 'Long position quantity (always used for cash)';
COMMENT ON COLUMN balances.quantity_short IS 'Short position quantity (not used for cash)';
COMMENT ON COLUMN balances.last_updated IS 'Last update timestamp';
COMMENT ON COLUMN balances.version IS 'Version for optimistic locking';
COMMENT ON COLUMN balances.created_at IS 'Record creation timestamp'; 