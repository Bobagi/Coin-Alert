BEGIN;

ALTER TABLE trading_operations DROP CONSTRAINT IF EXISTS trading_operations_environment_valid;
ALTER TABLE trading_operations ALTER COLUMN binance_environment DROP NOT NULL;

ALTER TABLE trading_operation_executions DROP CONSTRAINT IF EXISTS trading_operation_executions_environment_valid;
ALTER TABLE trading_operation_executions ALTER COLUMN binance_environment DROP NOT NULL;

COMMIT;
