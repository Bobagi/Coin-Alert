BEGIN;

-- Best-effort reversal. Collapsing per-environment settings back to one row per user can fail if a
-- user has rows for more than one environment; remove the extra rows first if that happens.
ALTER TABLE user_trading_settings DROP CONSTRAINT IF EXISTS user_trading_settings_pkey;
ALTER TABLE user_trading_settings ADD PRIMARY KEY (user_id);
ALTER TABLE user_trading_settings DROP COLUMN IF EXISTS binance_environment;

DROP INDEX IF EXISTS trading_operations_user_env_idx;
DROP INDEX IF EXISTS trading_operation_executions_user_env_idx;
ALTER TABLE trading_operations           DROP COLUMN IF EXISTS binance_environment;
ALTER TABLE trading_operation_executions DROP COLUMN IF EXISTS binance_environment;

COMMIT;
