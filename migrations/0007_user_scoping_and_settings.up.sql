BEGIN;

-- Per-user trading settings replace the previous process-global env/in-memory settings.
CREATE TABLE IF NOT EXISTS user_trading_settings (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    trading_pair_symbol VARCHAR(30) NOT NULL DEFAULT 'BTCUSDT',
    capital_threshold NUMERIC(20,8) NOT NULL DEFAULT 0,
    target_profit_percent NUMERIC(10,4) NOT NULL DEFAULT 1.0,
    stop_loss_percent NUMERIC(10,4),                       -- NULL = no stop-loss (Phase 2)
    auto_sell_interval_minutes INTEGER NOT NULL DEFAULT 60,
    daily_purchase_hour_utc INTEGER NOT NULL DEFAULT 4,
    live_trading_enabled BOOLEAN NOT NULL DEFAULT false,   -- explicit per-user opt-in to real money
    active_binance_environment VARCHAR(20) NOT NULL DEFAULT 'TESTNET',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Scope every existing domain table to a user (nullable FK for a non-destructive rollout;
-- tightened to NOT NULL during the Phase 5 fresh deploy/reset).
ALTER TABLE binance_credentials          ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE trading_operations           ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE scheduled_trading_operations ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE trading_operation_executions ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE daily_purchase_settings      ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE email_alerts                 ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS binance_credentials_user_id_idx          ON binance_credentials (user_id);
CREATE INDEX IF NOT EXISTS trading_operations_user_id_idx           ON trading_operations (user_id);
CREATE INDEX IF NOT EXISTS scheduled_trading_operations_user_id_idx ON scheduled_trading_operations (user_id);
CREATE INDEX IF NOT EXISTS trading_operation_executions_user_id_idx ON trading_operation_executions (user_id);
CREATE INDEX IF NOT EXISTS daily_purchase_settings_user_id_idx      ON daily_purchase_settings (user_id);
CREATE INDEX IF NOT EXISTS email_alerts_user_id_idx                 ON email_alerts (user_id);

COMMIT;
