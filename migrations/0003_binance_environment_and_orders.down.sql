BEGIN;

ALTER TABLE trading_operation_executions
DROP COLUMN IF EXISTS order_id;

ALTER TABLE trading_operations
DROP COLUMN IF EXISTS sell_target_price_per_unit,
DROP COLUMN IF EXISTS sell_order_id,
DROP COLUMN IF EXISTS buy_order_id;

ALTER TABLE binance_credentials
DROP COLUMN IF EXISTS is_active,
DROP COLUMN IF EXISTS api_base_url,
DROP COLUMN IF EXISTS environment;

COMMIT;
