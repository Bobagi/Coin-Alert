BEGIN;

ALTER TABLE trading_operation_executions DROP COLUMN IF EXISTS initiated_by;

COMMIT;
