BEGIN;

ALTER TABLE trading_operations DROP COLUMN IF EXISTS sell_order_expires_at;
ALTER TABLE user_trading_settings DROP COLUMN IF EXISTS sell_order_validity_days;

COMMIT;
