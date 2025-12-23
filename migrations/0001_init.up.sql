CREATE TABLE IF NOT EXISTS trading_operations (
    id SERIAL PRIMARY KEY,
    trading_pair_symbol VARCHAR(30) NOT NULL,
    quantity_purchased NUMERIC(20,8) NOT NULL,
    purchase_price_per_unit NUMERIC(20,8) NOT NULL,
    target_profit_percent NUMERIC(10,4) NOT NULL,
    status VARCHAR(10) NOT NULL,
    sell_price_per_unit NUMERIC(20,8),
    purchased_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    sold_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE IF NOT EXISTS email_alerts (
    id SERIAL PRIMARY KEY,
    recipient_address VARCHAR(255) NOT NULL,
    subject TEXT NOT NULL,
    message_body TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS binance_credentials (
    id SERIAL PRIMARY KEY,
    api_key TEXT NOT NULL,
    api_secret TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
