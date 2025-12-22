BEGIN;

CREATE TABLE IF NOT EXISTS daily_purchase_settings (
    id SERIAL PRIMARY KEY,
    trading_pair_symbol VARCHAR(30) NOT NULL,
    purchase_amount NUMERIC(20,8) NOT NULL,
    execution_hour_utc INTEGER NOT NULL DEFAULT 4,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

COMMIT;
