package repository

import (
	"context"
	"time"
)

// HasSuccessfulExecutionOfTypeSince reports whether the user already has a successful execution of
// the given type for a specific pair since a timestamp. Scoping by pair keeps the daily purchase
// idempotent per robot (one buy per coin per day), so several robots do not block one another.
func (repository *PostgresTradingOperationExecutionRepository) HasSuccessfulExecutionOfTypeSince(loadContext context.Context, userIdentifier int64, environment string, operationType string, tradingPairSymbol string, since time.Time) (bool, error) {
	row := repository.Database.QueryRowContext(
		loadContext,
		`SELECT EXISTS(
		    SELECT 1 FROM trading_operation_executions
		    WHERE user_id = $1 AND binance_environment = $2 AND operation_type = $3 AND trading_pair_symbol = $4 AND success = true AND executed_at >= $5
		 )`,
		userIdentifier, environment, operationType, tradingPairSymbol, since,
	)
	var executionExists bool
	if scanError := row.Scan(&executionExists); scanError != nil {
		return false, scanError
	}
	return executionExists, nil
}
