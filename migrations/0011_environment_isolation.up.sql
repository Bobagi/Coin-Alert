BEGIN;

-- Per-environment isolation: operations, executions and bot settings each become scoped to the
-- Binance environment (Testnet vs Production) they belong to, so switching the active environment
-- shows only that environment's history and bot configuration.

ALTER TABLE trading_operations           ADD COLUMN IF NOT EXISTS binance_environment VARCHAR(20);
ALTER TABLE trading_operation_executions ADD COLUMN IF NOT EXISTS binance_environment VARCHAR(20);

-- Backfill existing rows from each user's currently-active credential environment (fallback TESTNET,
-- since early usage of this app was on Testnet).
UPDATE trading_operations o
   SET binance_environment = COALESCE(
       (SELECT c.environment FROM binance_credentials c
         WHERE c.user_id = o.user_id AND c.is_active = true
         ORDER BY c.created_at DESC LIMIT 1), 'TESTNET')
 WHERE binance_environment IS NULL;

UPDATE trading_operation_executions e
   SET binance_environment = COALESCE(
       (SELECT c.environment FROM binance_credentials c
         WHERE c.user_id = e.user_id AND c.is_active = true
         ORDER BY c.created_at DESC LIMIT 1), 'TESTNET')
 WHERE binance_environment IS NULL;

CREATE INDEX IF NOT EXISTS trading_operations_user_env_idx           ON trading_operations (user_id, binance_environment);
CREATE INDEX IF NOT EXISTS trading_operation_executions_user_env_idx ON trading_operation_executions (user_id, binance_environment);

-- Bot settings: one row per (user, environment).
ALTER TABLE user_trading_settings ADD COLUMN IF NOT EXISTS binance_environment VARCHAR(20) NOT NULL DEFAULT 'TESTNET';

UPDATE user_trading_settings s
   SET binance_environment = COALESCE(
       (SELECT c.environment FROM binance_credentials c
         WHERE c.user_id = s.user_id AND c.is_active = true
         ORDER BY c.created_at DESC LIMIT 1),
       NULLIF(s.active_binance_environment, ''),
       'TESTNET');

ALTER TABLE user_trading_settings DROP CONSTRAINT IF EXISTS user_trading_settings_pkey;
ALTER TABLE user_trading_settings ADD PRIMARY KEY (user_id, binance_environment);

COMMIT;
