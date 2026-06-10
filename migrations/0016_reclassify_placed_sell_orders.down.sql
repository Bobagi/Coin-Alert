BEGIN;

-- Best-effort revert (placements created after the fix are indistinguishable, but use the same rule).
UPDATE trading_operation_executions executionRow
SET operation_type = 'SELL'
WHERE executionRow.operation_type = 'SELL_ORDER_PLACED'
  AND executionRow.order_id IS NOT NULL
  AND EXISTS (
    SELECT 1 FROM trading_operations operationRow
    WHERE operationRow.user_id = executionRow.user_id
      AND operationRow.sell_order_id = executionRow.order_id
      AND operationRow.status <> 'SOLD'
  );

COMMIT;
