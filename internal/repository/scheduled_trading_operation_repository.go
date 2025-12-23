package repository

import (
        "context"
        "database/sql"
        "time"

        "coin-alert/internal/domain"
)

type ScheduledTradingOperationRepository interface {
        CreateScheduledOperation(context.Context, domain.ScheduledTradingOperation) (int64, error)
        GetNextScheduledOperation(context.Context) (*domain.ScheduledTradingOperation, error)
        ListScheduledOperations(context.Context, int) ([]domain.ScheduledTradingOperation, error)
        UpdateStatus(context.Context, int64, string) error
}

type PostgresScheduledTradingOperationRepository struct {
        Database *sql.DB
}

func NewPostgresScheduledTradingOperationRepository(database *sql.DB) *PostgresScheduledTradingOperationRepository {
        return &PostgresScheduledTradingOperationRepository{Database: database}
}

func (repository *PostgresScheduledTradingOperationRepository) CreateScheduledOperation(contextWithTimeout context.Context, operation domain.ScheduledTradingOperation) (int64, error) {
        insertSQL := `INSERT INTO scheduled_trading_operations(trading_pair_symbol, capital_threshold, target_profit_percent, operation_type, scheduled_execution_time, status) VALUES($1, $2, $3, $4, $5, $6) RETURNING id, created_at, updated_at`
        statementContext, statementCancel := context.WithTimeout(contextWithTimeout, 5*time.Second)
        defer statementCancel()

        row := repository.Database.QueryRowContext(statementContext, insertSQL, operation.TradingPairSymbol, operation.CapitalThreshold, operation.TargetProfitPercent, operation.OperationType, operation.ScheduledExecutionTime, operation.Status)
        var identifier int64
        var createdAt time.Time
        var updatedAt time.Time
        scanError := row.Scan(&identifier, &createdAt, &updatedAt)
        if scanError != nil {
                return 0, scanError
        }

        return identifier, nil
}

func (repository *PostgresScheduledTradingOperationRepository) GetNextScheduledOperation(contextWithTimeout context.Context) (*domain.ScheduledTradingOperation, error) {
        querySQL := `SELECT id, trading_pair_symbol, capital_threshold, target_profit_percent, operation_type, scheduled_execution_time, status, created_at, updated_at FROM scheduled_trading_operations WHERE status = $1 ORDER BY scheduled_execution_time ASC LIMIT 1`
        queryContext, queryCancel := context.WithTimeout(contextWithTimeout, 5*time.Second)
        defer queryCancel()

        row := repository.Database.QueryRowContext(queryContext, querySQL, domain.ScheduledOperationStatusScheduled)
        var operation domain.ScheduledTradingOperation
        scanError := row.Scan(&operation.Identifier, &operation.TradingPairSymbol, &operation.CapitalThreshold, &operation.TargetProfitPercent, &operation.OperationType, &operation.ScheduledExecutionTime, &operation.Status, &operation.CreatedAt, &operation.UpdatedAt)
        if scanError != nil {
                if scanError == sql.ErrNoRows {
                        return nil, nil
                }
                return nil, scanError
        }

        return &operation, nil
}

func (repository *PostgresScheduledTradingOperationRepository) ListScheduledOperations(contextWithTimeout context.Context, limit int) ([]domain.ScheduledTradingOperation, error) {
        querySQL := `SELECT id, trading_pair_symbol, capital_threshold, target_profit_percent, operation_type, scheduled_execution_time, status, created_at, updated_at FROM scheduled_trading_operations ORDER BY scheduled_execution_time ASC LIMIT $1`
        queryContext, queryCancel := context.WithTimeout(contextWithTimeout, 5*time.Second)
        defer queryCancel()

        rows, queryError := repository.Database.QueryContext(queryContext, querySQL, limit)
        if queryError != nil {
                return nil, queryError
        }
        defer rows.Close()

        var operations []domain.ScheduledTradingOperation
        for rows.Next() {
                var operation domain.ScheduledTradingOperation
                scanError := rows.Scan(&operation.Identifier, &operation.TradingPairSymbol, &operation.CapitalThreshold, &operation.TargetProfitPercent, &operation.OperationType, &operation.ScheduledExecutionTime, &operation.Status, &operation.CreatedAt, &operation.UpdatedAt)
                if scanError != nil {
                        return nil, scanError
                }

                operations = append(operations, operation)
        }

        return operations, nil
}

func (repository *PostgresScheduledTradingOperationRepository) UpdateStatus(contextWithTimeout context.Context, operationIdentifier int64, newStatus string) error {
        updateSQL := `UPDATE scheduled_trading_operations SET status = $1, updated_at = NOW() WHERE id = $2`
        updateContext, updateCancel := context.WithTimeout(contextWithTimeout, 5*time.Second)
        defer updateCancel()

        _, updateError := repository.Database.ExecContext(updateContext, updateSQL, newStatus, operationIdentifier)
        return updateError
}
