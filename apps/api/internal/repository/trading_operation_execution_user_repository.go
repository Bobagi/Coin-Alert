package repository

import (
	"context"

	"coin-alert/internal/domain"
)

const userExecutionColumns = `id, scheduled_operation_id, trading_pair_symbol, operation_type, unit_price,
	quantity, total_value, executed_at, success, error_message, order_id, created_at, updated_at`

// UserTradingOperationExecutionRepository persists execution attempts scoped to a single user.
type UserTradingOperationExecutionRepository interface {
	LogExecutionForUser(operationContext context.Context, userIdentifier int64, execution domain.TradingOperationExecution) (int64, error)
	ListRecentExecutionsForUser(loadContext context.Context, userIdentifier int64, limit int) ([]domain.TradingOperationExecution, error)
}

func (repository *PostgresTradingOperationExecutionRepository) LogExecutionForUser(operationContext context.Context, userIdentifier int64, execution domain.TradingOperationExecution) (int64, error) {
	row := repository.Database.QueryRowContext(
		operationContext,
		`INSERT INTO trading_operation_executions
		    (user_id, scheduled_operation_id, trading_pair_symbol, operation_type, unit_price, quantity, total_value, executed_at, success, error_message, order_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id`,
		userIdentifier,
		execution.ScheduledOperationID,
		execution.TradingPairSymbol,
		execution.OperationType,
		execution.UnitPrice,
		execution.Quantity,
		execution.TotalValue,
		execution.ExecutedAt,
		execution.Success,
		execution.ErrorMessage,
		execution.OrderIdentifier,
	)
	var executionIdentifier int64
	if scanError := row.Scan(&executionIdentifier); scanError != nil {
		return 0, scanError
	}
	return executionIdentifier, nil
}

func (repository *PostgresTradingOperationExecutionRepository) ListRecentExecutionsForUser(loadContext context.Context, userIdentifier int64, limit int) ([]domain.TradingOperationExecution, error) {
	rows, queryError := repository.Database.QueryContext(
		loadContext,
		`SELECT `+userExecutionColumns+` FROM trading_operation_executions WHERE user_id = $1 ORDER BY executed_at DESC LIMIT $2`,
		userIdentifier, limit,
	)
	if queryError != nil {
		return nil, queryError
	}
	defer rows.Close()
	return scanTradingOperationExecutions(rows)
}
