BEGIN;

-- A "robot" is one automated trading bot scoped to a single coin/pair, in a single Binance
-- environment, for one user. Replacing the previous single-config-per-environment model, a user can
-- now run several robots (one per coin). Standard users are limited to one robot per environment in
-- code; admins are unlimited (monetization hook).
CREATE TABLE IF NOT EXISTS trading_robots (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    binance_environment VARCHAR(20) NOT NULL,
    trading_pair_symbol VARCHAR(40) NOT NULL,
    name VARCHAR(80),
    capital_threshold DOUBLE PRECISION NOT NULL DEFAULT 0,
    target_profit_percent DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    stop_loss_percent DOUBLE PRECISION,            -- NULL = no stop-loss
    daily_purchase_hour_utc INT NOT NULL DEFAULT 4,
    daily_purchase_enabled BOOLEAN NOT NULL DEFAULT false,
    sell_order_validity_days INT NOT NULL DEFAULT 0, -- 0 = GTC
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS trading_robots_user_env_idx ON trading_robots (user_id, binance_environment);
-- One robot per coin within an environment, so the same pair is not automated twice.
CREATE UNIQUE INDEX IF NOT EXISTS trading_robots_user_env_symbol_unique
    ON trading_robots (user_id, binance_environment, trading_pair_symbol);

-- Preserve current behavior: seed one robot from each existing per-environment settings row.
INSERT INTO trading_robots (
    user_id, binance_environment, trading_pair_symbol, name,
    capital_threshold, target_profit_percent, stop_loss_percent,
    daily_purchase_hour_utc, daily_purchase_enabled, sell_order_validity_days, is_enabled)
SELECT
    user_id,
    binance_environment,
    COALESCE(NULLIF(trading_pair_symbol, ''), 'BTCUSDT'),
    COALESCE(NULLIF(trading_pair_symbol, ''), 'BTCUSDT'),
    capital_threshold,
    target_profit_percent,
    stop_loss_percent,
    daily_purchase_hour_utc,
    daily_purchase_enabled,
    sell_order_validity_days,
    true
FROM user_trading_settings
ON CONFLICT (user_id, binance_environment, trading_pair_symbol) DO NOTHING;

COMMIT;
