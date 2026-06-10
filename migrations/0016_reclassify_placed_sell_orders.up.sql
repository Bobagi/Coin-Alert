BEGIN;

-- operation_type was VARCHAR(10), too small for newer types (SELL_ORDER_PLACED=17, SELL_CANCELED=13,
-- SELL_EXPIRED=12) — their inserts were silently failing. Widen it first.
ALTER TABLE trading_operation_executions ALTER COLUMN operation_type TYPE VARCHAR(32);

-- Historical hygiene. Take-profit ORDER placements used to be logged as SELL, which now renders as
-- "Sold" and misleads (it looks like the position was closed and the money recovered). Reclassify
-- those placements to SELL_ORDER_PLACED. A SELL execution whose order id belongs to an operation
-- that never completed (status <> SOLD) can only be a placement — never a finished sale — so this is
-- safe. Genuine completed sales (on SOLD operations) keep their SELL type.
UPDATE trading_operation_executions executionRow
SET operation_type = 'SELL_ORDER_PLACED'
WHERE executionRow.operation_type = 'SELL'
  AND executionRow.order_id IS NOT NULL
  AND EXISTS (
    SELECT 1 FROM trading_operations operationRow
    WHERE operationRow.user_id = executionRow.user_id
      AND operationRow.sell_order_id = executionRow.order_id
      AND operationRow.status <> 'SOLD'
  );

COMMIT;
