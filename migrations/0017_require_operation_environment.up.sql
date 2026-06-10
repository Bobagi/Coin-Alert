BEGIN;

-- Every operation/execution must belong to a Binance environment. The app already refuses to trade
-- without an active environment; this enforces it at the database level too. (No NULL/empty values
-- exist today, so no backfill is needed.)
ALTER TABLE trading_operations ALTER COLUMN binance_environment SET NOT NULL;
ALTER TABLE trading_operations
    ADD CONSTRAINT trading_operations_environment_valid
    CHECK (binance_environment IN ('TESTNET', 'PRODUCTION'));

ALTER TABLE trading_operation_executions ALTER COLUMN binance_environment SET NOT NULL;
ALTER TABLE trading_operation_executions
    ADD CONSTRAINT trading_operation_executions_environment_valid
    CHECK (binance_environment IN ('TESTNET', 'PRODUCTION'));

COMMIT;
