BEGIN;

-- Record whether each execution was triggered by the user (manual action) or the bot (automation).
ALTER TABLE trading_operation_executions ADD COLUMN IF NOT EXISTS initiated_by VARCHAR(10);

-- Backfill existing rows: daily buys are always the bot; the rest default to the user (best-effort).
UPDATE trading_operation_executions
   SET initiated_by = CASE WHEN operation_type = 'DAILY_BUY' THEN 'BOT' ELSE 'USER' END
 WHERE initiated_by IS NULL;

COMMIT;
