BEGIN;

-- Explicit on/off switch for the daily DCA buy, so a user can fully pause the bot without having to
-- zero out their capital. Defaults to enabled to preserve existing behaviour.
ALTER TABLE user_trading_settings ADD COLUMN IF NOT EXISTS daily_purchase_enabled BOOLEAN NOT NULL DEFAULT true;

COMMIT;
