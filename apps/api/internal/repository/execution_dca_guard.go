package repository

import (
	"context"
	"time"
)

// HasSuccessfulExecutionOfTypeSince reports whether the user already has a successful execution of
// the given type since a timestamp. Used to make the daily purchase idempotent within a day.
func (repository *PostgresTradingOperationExecutionRepository) HasSuccessfulExecutionOfTypeSince(loadContext context.Context, userIdentifier int64, environment string, operationType string, since time.Time) (bool, error) {
	row := repository.Database.QueryRowContext(
		loadContext,
		`SELECT EXISTS(
		    SELECT 1 FROM trading_operation_executions
		    WHERE user_id = $1 AND binance_environment = $2 AND operation_type = $3 AND success = true AND executed_at >= $4
		 )`,
		userIdentifier, environment, operationType, since,
	)
	var executionExists bool
	if scanError := row.Scan(&executionExists); scanError != nil {
		return false, scanError
	}
	return executionExists, nil
}
