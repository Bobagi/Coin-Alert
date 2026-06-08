BEGIN;

ALTER TABLE user_trading_settings DROP COLUMN IF EXISTS daily_purchase_enabled;

COMMIT;
