BEGIN;

-- Optional time limit for the take-profit sell order. Binance spot LIMIT orders are GTC-only, so the
-- app enforces this: 0 = no expiry (GTC, default); N = cancel the resting sell after N days.
ALTER TABLE user_trading_settings ADD COLUMN IF NOT EXISTS sell_order_validity_days INTEGER NOT NULL DEFAULT 0;

-- When the resting take-profit should be auto-cancelled (NULL = never / GTC).
ALTER TABLE trading_operations ADD COLUMN IF NOT EXISTS sell_order_expires_at TIMESTAMPTZ;

COMMIT;
