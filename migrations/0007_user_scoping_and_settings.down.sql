BEGIN;

ALTER TABLE binance_credentials          DROP COLUMN IF EXISTS user_id;
ALTER TABLE trading_operations           DROP COLUMN IF EXISTS user_id;
ALTER TABLE scheduled_trading_operations DROP COLUMN IF EXISTS user_id;
ALTER TABLE trading_operation_executions DROP COLUMN IF EXISTS user_id;
ALTER TABLE daily_purchase_settings      DROP COLUMN IF EXISTS user_id;
ALTER TABLE email_alerts                 DROP COLUMN IF EXISTS user_id;

DROP TABLE IF EXISTS user_trading_settings;

COMMIT;
