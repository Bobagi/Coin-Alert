BEGIN;

DROP TABLE IF EXISTS email_alerts;
CREATE TABLE email_alerts (
    id SERIAL PRIMARY KEY,
    recipient_address VARCHAR(255) NOT NULL,
    trading_pair_or_currency VARCHAR(60) NOT NULL,
    threshold_value NUMERIC(20,8) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS scheduled_trading_operations (
    id SERIAL PRIMARY KEY,
    trading_pair_symbol VARCHAR(30) NOT NULL,
    capital_threshold NUMERIC(20,8) NOT NULL,
    target_profit_percent NUMERIC(10,4) NOT NULL,
    operation_type VARCHAR(10) NOT NULL,
    scheduled_execution_time TIMESTAMPTZ NOT NULL,
    status VARCHAR(15) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS trading_operation_executions (
    id SERIAL PRIMARY KEY,
    scheduled_operation_id INTEGER REFERENCES scheduled_trading_operations(id),
    trading_pair_symbol VARCHAR(30) NOT NULL,
    operation_type VARCHAR(10) NOT NULL,
    unit_price NUMERIC(20,8) NOT NULL,
    quantity NUMERIC(20,8) NOT NULL,
    total_value NUMERIC(20,8) NOT NULL,
    executed_at TIMESTAMPTZ NOT NULL,
    success BOOLEAN NOT NULL,
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

COMMIT;
