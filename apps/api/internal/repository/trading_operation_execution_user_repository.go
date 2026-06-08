package repository

import (
	"context"
	"database/sql"

	"coin-alert/internal/domain"
)

const userExecutionColumns = `id, scheduled_operation_id, trading_pair_symbol, operation_type, unit_price,
	quantity, total_value, executed_at, success, error_message, order_id, created_at, updated_at,
	COALESCE(binance_environment, '')`

// UserTradingOperationExecutionRepository persists execution attempts scoped to a single user AND environment.
type UserTradingOperationExecutionRepository interface {
	LogExecutionForUser(operationContext context.Context, userIdentifier int64, execution domain.TradingOperationExecution) (int64, error)
	ListRecentExecutionsForUser(loadContext context.Context, userIdentifier int64, environment string, limit int) ([]domain.TradingOperationExecution, error)
}

func (repository *PostgresTradingOperationExecutionRepository) LogExecutionForUser(operationContext context.Context, userIdentifier int64, execution domain.TradingOperationExecution) (int64, error) {
	row := repository.Database.QueryRowContext(
		operationContext,
		`INSERT INTO trading_operation_executions
		    (user_id, scheduled_operation_id, trading_pair_symbol, operation_type, unit_price, quantity, total_value, executed_at, success, error_message, order_id, binance_environment)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
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
		execution.BinanceEnvironment,
	)
	var executionIdentifier int64
	if scanError := row.Scan(&executionIdentifier); scanError != nil {
		return 0, scanError
	}
	return executionIdentifier, nil
}

func (repository *PostgresTradingOperationExecutionRepository) ListRecentExecutionsForUser(loadContext context.Context, userIdentifier int64, environment string, limit int) ([]domain.TradingOperationExecution, error) {
	rows, queryError := repository.Database.QueryContext(
		loadContext,
		`SELECT `+userExecutionColumns+` FROM trading_operation_executions WHERE user_id = $1 AND binance_environment = $2 ORDER BY executed_at DESC LIMIT $3`,
		userIdentifier, environment, limit,
	)
	if queryError != nil {
		return nil, queryError
	}
	defer rows.Close()
	return scanUserExecutionRows(rows)
}

// scanUserExecutionRows is separate from the legacy scanTradingOperationExecutions because the
// user-scoped query also selects binance_environment.
func scanUserExecutionRows(rows *sql.Rows) ([]domain.TradingOperationExecution, error) {
	executions := make([]domain.TradingOperationExecution, 0)
	for rows.Next() {
		var execution domain.TradingOperationExecution
		var scheduledOperationID sql.NullInt64
		var errorMessage sql.NullString
		var orderIdentifier sql.NullString
		scanError := rows.Scan(
			&execution.Identifier,
			&scheduledOperationID,
			&execution.TradingPairSymbol,
			&execution.OperationType,
			&execution.UnitPrice,
			&execution.Quantity,
			&execution.TotalValue,
			&execution.ExecutedAt,
			&execution.Success,
			&errorMessage,
			&orderIdentifier,
			&execution.CreatedAt,
			&execution.UpdatedAt,
			&execution.BinanceEnvironment,
		)
		if scanError != nil {
			return nil, scanError
		}
		if scheduledOperationID.Valid {
			value := scheduledOperationID.Int64
			execution.ScheduledOperationID = &value
		}
		if errorMessage.Valid {
			value := errorMessage.String
			execution.ErrorMessage = &value
		}
		if orderIdentifier.Valid {
			value := orderIdentifier.String
			execution.OrderIdentifier = &value
		}
		executions = append(executions, execution)
	}
	return executions, rows.Err()
}
