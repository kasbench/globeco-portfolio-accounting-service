-- Drop indexes for balances table
DROP INDEX IF EXISTS idx_balances_last_updated;
DROP INDEX IF EXISTS idx_balances_cash;
DROP INDEX IF EXISTS idx_balances_security_id;
DROP INDEX IF EXISTS idx_balances_portfolio_id;
DROP INDEX IF EXISTS balances_portfolio_security_ndx;

-- Drop indexes for transactions table
DROP INDEX IF EXISTS idx_transactions_status_new;
DROP INDEX IF EXISTS idx_transactions_portfolio_security_date;
DROP INDEX IF EXISTS idx_transactions_status;
DROP INDEX IF EXISTS idx_transactions_date;
DROP INDEX IF EXISTS idx_transactions_security_id;
DROP INDEX IF EXISTS idx_transactions_portfolio_id;
DROP INDEX IF EXISTS transaction_source_ndx; 