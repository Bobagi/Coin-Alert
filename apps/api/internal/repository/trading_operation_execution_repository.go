package repository

import (
	"context"
	"database/sql"
	"time"

	"coin-alert/internal/domain"
)

type TradingOperationExecutionRepository interface {
	LogExecution(context.Context, domain.TradingOperationExecution) (int64, error)
	ListRecentExecutions(context.Context, int) ([]domain.TradingOperationExecution, error)
	ListExecutionsPage(context.Context, int, int) ([]domain.TradingOperationExecution, error)
	ListRecentExecutionsByOperationType(context.Context, int, string) ([]domain.TradingOperationExecution, error)
	ListExecutionsPageByOperationType(context.Context, int, int, string) ([]domain.TradingOperationExecution, error)
}

type PostgresTradingOperationExecutionRepository struct {
	Database *sql.DB
}

func NewPostgresTradingOperationExecutionRepository(database *sql.DB) *PostgresTradingOperationExecutionRepository {
	return &PostgresTradingOperationExecutionRepository{Database: database}
}

func (repository *PostgresTradingOperationExecutionRepository) LogExecution(contextWithTimeout context.Context, execution domain.TradingOperationExecution) (int64, error) {
	insertSQL := `INSERT INTO trading_operation_executions(scheduled_operation_id, trading_pair_symbol, operation_type, unit_price, quantity, total_value, executed_at, success, error_message, order_id) VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id, created_at, updated_at`
	statementContext, statementCancel := context.WithTimeout(contextWithTimeout, 5*time.Second)
	defer statementCancel()

	row := repository.Database.QueryRowContext(statementContext, insertSQL, execution.ScheduledOperationID, execution.TradingPairSymbol, execution.OperationType, execution.UnitPrice, execution.Quantity, execution.TotalValue, execution.ExecutedAt, execution.Success, execution.ErrorMessage, execution.OrderIdentifier)
	var identifier int64
	var createdAt time.Time
	var updatedAt time.Time
	scanError := row.Scan(&identifier, &createdAt, &updatedAt)
	if scanError != nil {
		return 0, scanError
	}

	return identifier, nil
}

func (repository *PostgresTradingOperationExecutionRepository) ListRecentExecutions(contextWithTimeout context.Context, limit int) ([]domain.TradingOperationExecution, error) {
	querySQL := `SELECT id, scheduled_operation_id, trading_pair_symbol, operation_type, unit_price, quantity, total_value, executed_at, success, error_message, order_id, created_at, updated_at FROM trading_operation_executions ORDER BY executed_at DESC LIMIT $1`
	queryContext, queryCancel := context.WithTimeout(contextWithTimeout, 5*time.Second)
	defer queryCancel()

	rows, queryError := repository.Database.QueryContext(queryContext, querySQL, limit)
	if queryError != nil {
		return nil, queryError
	}
	defer rows.Close()
	return scanTradingOperationExecutions(rows)
}

func (repository *PostgresTradingOperationExecutionRepository) ListExecutionsPage(contextWithTimeout context.Context, limit int, offset int) ([]domain.TradingOperationExecution, error) {
	querySQL := `SELECT id, scheduled_operation_id, trading_pair_symbol, operation_type, unit_price, quantity, total_value, executed_at, success, error_message, order_id, created_at, updated_at FROM trading_operation_executions ORDER BY executed_at DESC LIMIT $1 OFFSET $2`
	queryContext, queryCancel := context.WithTimeout(contextWithTimeout, 5*time.Second)
	defer queryCancel()

	rows, queryError := repository.Database.QueryContext(queryContext, querySQL, limit, offset)
	if queryError != nil {
		return nil, queryError
	}
	defer rows.Close()
	return scanTradingOperationExecutions(rows)
}

func (repository *PostgresTradingOperationExecutionRepository) ListRecentExecutionsByOperationType(contextWithTimeout context.Context, limit int, operationType string) ([]domain.TradingOperationExecution, error) {
	querySQL := `SELECT id, scheduled_operation_id, trading_pair_symbol, operation_type, unit_price, quantity, total_value, executed_at, success, error_message, order_id, created_at, updated_at FROM trading_operation_executions WHERE operation_type = $1 ORDER BY executed_at DESC LIMIT $2`
	queryContext, queryCancel := context.WithTimeout(contextWithTimeout, 5*time.Second)
	defer queryCancel()

	rows, queryError := repository.Database.QueryContext(queryContext, querySQL, operationType, limit)
	if queryError != nil {
		return nil, queryError
	}
	defer rows.Close()

	return scanTradingOperationExecutions(rows)
}

func (repository *PostgresTradingOperationExecutionRepository) ListExecutionsPageByOperationType(contextWithTimeout context.Context, limit int, offset int, operationType string) ([]domain.TradingOperationExecution, error) {
	querySQL := `SELECT id, scheduled_operation_id, trading_pair_symbol, operation_type, unit_price, quantity, total_value, executed_at, success, error_message, order_id, created_at, updated_at FROM trading_operation_executions WHERE operation_type = $1 ORDER BY executed_at DESC LIMIT $2 OFFSET $3`
	queryContext, queryCancel := context.WithTimeout(contextWithTimeout, 5*time.Second)
	defer queryCancel()

	rows, queryError := repository.Database.QueryContext(queryContext, querySQL, operationType, limit, offset)
	if queryError != nil {
		return nil, queryError
	}
	defer rows.Close()

	return scanTradingOperationExecutions(rows)
}

func scanTradingOperationExecutions(rows *sql.Rows) ([]domain.TradingOperationExecution, error) {
	var executions []domain.TradingOperationExecution
	for rows.Next() {
		var execution domain.TradingOperationExecution
		var scheduledOperationID sql.NullInt64
		var errorMessage sql.NullString
		var orderIdentifier sql.NullString
		scanError := rows.Scan(&execution.Identifier, &scheduledOperationID, &execution.TradingPairSymbol, &execution.OperationType, &execution.UnitPrice, &execution.Quantity, &execution.TotalValue, &execution.ExecutedAt, &execution.Success, &errorMessage, &orderIdentifier, &execution.CreatedAt, &execution.UpdatedAt)
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

	return executions, nil
}
