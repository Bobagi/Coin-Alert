BEGIN;

ALTER TABLE binance_credentials
ADD COLUMN IF NOT EXISTS environment VARCHAR(20) NOT NULL DEFAULT 'TESTNET',
ADD COLUMN IF NOT EXISTS api_base_url TEXT NOT NULL DEFAULT 'https://testnet.binance.vision',
ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE trading_operations
ADD COLUMN IF NOT EXISTS buy_order_id TEXT,
ADD COLUMN IF NOT EXISTS sell_order_id TEXT,
ADD COLUMN IF NOT EXISTS sell_target_price_per_unit NUMERIC(20,8);

ALTER TABLE trading_operation_executions
ADD COLUMN IF NOT EXISTS order_id TEXT;

COMMIT;
