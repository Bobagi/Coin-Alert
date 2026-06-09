package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"coin-alert/internal/domain"
)

// ErrOperationNotFound is returned when no operation matches the id for the given user.
var ErrOperationNotFound = errors.New("operation not found")

const userTradingOperationColumns = `id, trading_pair_symbol, quantity_purchased, purchase_price_per_unit,
	target_profit_percent, status, sell_price_per_unit, purchased_at, sold_at,
	buy_order_id, sell_order_id, sell_target_price_per_unit, COALESCE(binance_environment, ''), sell_order_expires_at`

// UserTradingOperationRepository persists trading operations scoped to a single user AND environment.
type UserTradingOperationRepository interface {
	CreatePurchaseOperationForUser(operationContext context.Context, userIdentifier int64, operation domain.TradingOperation) (int64, error)
	ListRecentOperationsForUser(loadContext context.Context, userIdentifier int64, environment string, limit int) ([]domain.TradingOperation, error)
	ListOpenOperationsForUser(loadContext context.Context, userIdentifier int64, environment string) ([]domain.TradingOperation, error)
	FindOperationByIdForUser(loadContext context.Context, userIdentifier int64, operationIdentifier int64) (*domain.TradingOperation, error)
	UpdateOperationAsSoldForUser(operationContext context.Context, userIdentifier int64, operationIdentifier int64, sellPricePerUnit float64) error
	UpdateOperationSellOrderForUser(operationContext context.Context, userIdentifier int64, operationIdentifier int64, sellOrderIdentifier string, sellTargetPrice float64, sellOrderExpiresAt *time.Time) error
	MarkOperationCanceledForUser(operationContext context.Context, userIdentifier int64, operationIdentifier int64) error
	ClearSellOrderForUser(operationContext context.Context, userIdentifier int64, operationIdentifier int64) error
	CalculateOpenAllocationTotalForUser(loadContext context.Context, userIdentifier int64, environment string) (float64, error)
}

func (repository *PostgresTradingOperationRepository) CreatePurchaseOperationForUser(operationContext context.Context, userIdentifier int64, operation domain.TradingOperation) (int64, error) {
	row := repository.Database.QueryRowContext(
		operationContext,
		`INSERT INTO trading_operations
		    (user_id, trading_pair_symbol, quantity_purchased, purchase_price_per_unit, target_profit_percent, status, buy_order_id, sell_order_id, sell_target_price_per_unit, binance_environment, sell_order_expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id`,
		userIdentifier,
		operation.TradingPairSymbol,
		operation.QuantityPurchased,
		operation.PurchasePricePerUnit,
		operation.TargetProfitPercent,
		operation.Status,
		operation.BuyOrderIdentifier,
		operation.SellOrderIdentifier,
		operation.SellTargetPricePerUnit,
		operation.BinanceEnvironment,
		operation.SellOrderExpiresAt,
	)
	var operationIdentifier int64
	if scanError := row.Scan(&operationIdentifier); scanError != nil {
		return 0, scanError
	}
	return operationIdentifier, nil
}

func (repository *PostgresTradingOperationRepository) ListRecentOperationsForUser(loadContext context.Context, userIdentifier int64, environment string, limit int) ([]domain.TradingOperation, error) {
	rows, queryError := repository.Database.QueryContext(
		loadContext,
		`SELECT `+userTradingOperationColumns+` FROM trading_operations WHERE user_id = $1 AND binance_environment = $2 ORDER BY purchased_at DESC LIMIT $3`,
		userIdentifier, environment, limit,
	)
	if queryError != nil {
		return nil, queryError
	}
	defer rows.Close()
	return scanUserTradingOperationRows(rows)
}

func (repository *PostgresTradingOperationRepository) ListOpenOperationsForUser(loadContext context.Context, userIdentifier int64, environment string) ([]domain.TradingOperation, error) {
	rows, queryError := repository.Database.QueryContext(
		loadContext,
		`SELECT `+userTradingOperationColumns+` FROM trading_operations WHERE user_id = $1 AND binance_environment = $2 AND status = $3 ORDER BY purchased_at ASC`,
		userIdentifier, environment, domain.TradingOperationStatusOpen,
	)
	if queryError != nil {
		return nil, queryError
	}
	defer rows.Close()
	return scanUserTradingOperationRows(rows)
}

func (repository *PostgresTradingOperationRepository) FindOperationByIdForUser(loadContext context.Context, userIdentifier int64, operationIdentifier int64) (*domain.TradingOperation, error) {
	rows, queryError := repository.Database.QueryContext(
		loadContext,
		`SELECT `+userTradingOperationColumns+` FROM trading_operations WHERE id = $1 AND user_id = $2 LIMIT 1`,
		operationIdentifier, userIdentifier,
	)
	if queryError != nil {
		return nil, queryError
	}
	defer rows.Close()
	operations, scanError := scanUserTradingOperationRows(rows)
	if scanError != nil {
		return nil, scanError
	}
	if len(operations) == 0 {
		return nil, ErrOperationNotFound
	}
	return &operations[0], nil
}

func (repository *PostgresTradingOperationRepository) UpdateOperationAsSoldForUser(operationContext context.Context, userIdentifier int64, operationIdentifier int64, sellPricePerUnit float64) error {
	_, updateError := repository.Database.ExecContext(
		operationContext,
		`UPDATE trading_operations SET status = $1, sell_price_per_unit = $2, sold_at = NOW() WHERE id = $3 AND user_id = $4`,
		domain.TradingOperationStatusSold, sellPricePerUnit, operationIdentifier, userIdentifier,
	)
	return updateError
}

func (repository *PostgresTradingOperationRepository) UpdateOperationSellOrderForUser(operationContext context.Context, userIdentifier int64, operationIdentifier int64, sellOrderIdentifier string, sellTargetPrice float64, sellOrderExpiresAt *time.Time) error {
	_, updateError := repository.Database.ExecContext(
		operationContext,
		`UPDATE trading_operations SET sell_order_id = $1, sell_target_price_per_unit = $2, sell_order_expires_at = $3 WHERE id = $4 AND user_id = $5`,
		sellOrderIdentifier, sellTargetPrice, sellOrderExpiresAt, operationIdentifier, userIdentifier,
	)
	return updateError
}

// MarkOperationCanceledForUser closes an operation as CANCELED (its take-profit was cancelled outside
// the app), removing it from the active positions view.
func (repository *PostgresTradingOperationRepository) MarkOperationCanceledForUser(operationContext context.Context, userIdentifier int64, operationIdentifier int64) error {
	_, updateError := repository.Database.ExecContext(
		operationContext,
		`UPDATE trading_operations SET status = $1, sold_at = NOW() WHERE id = $2 AND user_id = $3`,
		domain.TradingOperationStatusCanceled, operationIdentifier, userIdentifier,
	)
	return updateError
}

// ClearSellOrderForUser detaches the resting sell order from an OPEN operation (e.g. after its
// validity expired), leaving the position open but unprotected so the user can re-place or sell.
func (repository *PostgresTradingOperationRepository) ClearSellOrderForUser(operationContext context.Context, userIdentifier int64, operationIdentifier int64) error {
	_, updateError := repository.Database.ExecContext(
		operationContext,
		`UPDATE trading_operations SET sell_order_id = NULL, sell_order_expires_at = NULL WHERE id = $1 AND user_id = $2`,
		operationIdentifier, userIdentifier,
	)
	return updateError
}

func (repository *PostgresTradingOperationRepository) CalculateOpenAllocationTotalForUser(loadContext context.Context, userIdentifier int64, environment string) (float64, error) {
	row := repository.Database.QueryRowContext(
		loadContext,
		`SELECT COALESCE(SUM(quantity_purchased * purchase_price_per_unit), 0) FROM trading_operations WHERE user_id = $1 AND binance_environment = $2 AND status = $3`,
		userIdentifier, environment, domain.TradingOperationStatusOpen,
	)
	var totalAllocated float64
	if scanError := row.Scan(&totalAllocated); scanError != nil {
		return 0, scanError
	}
	return totalAllocated, nil
}

func scanUserTradingOperationRows(rows *sql.Rows) ([]domain.TradingOperation, error) {
	operations := make([]domain.TradingOperation, 0)
	for rows.Next() {
		var operation domain.TradingOperation
		var sellPrice sql.NullFloat64
		var buyOrderIdentifier sql.NullString
		var sellOrderIdentifier sql.NullString
		var sellTargetPrice sql.NullFloat64
		var sellOrderExpiresAt sql.NullTime
		scanError := rows.Scan(
			&operation.Identifier,
			&operation.TradingPairSymbol,
			&operation.QuantityPurchased,
			&operation.PurchasePricePerUnit,
			&operation.TargetProfitPercent,
			&operation.Status,
			&sellPrice,
			&operation.PurchaseTimestamp,
			&operation.SellTimestamp,
			&buyOrderIdentifier,
			&sellOrderIdentifier,
			&sellTargetPrice,
			&operation.BinanceEnvironment,
			&sellOrderExpiresAt,
		)
		if scanError != nil {
			return nil, scanError
		}
		if sellPrice.Valid {
			value := sellPrice.Float64
			operation.SellPricePerUnit = &value
		}
		if buyOrderIdentifier.Valid {
			value := buyOrderIdentifier.String
			operation.BuyOrderIdentifier = &value
		}
		if sellOrderIdentifier.Valid {
			value := sellOrderIdentifier.String
			operation.SellOrderIdentifier = &value
		}
		if sellTargetPrice.Valid {
			value := sellTargetPrice.Float64
			operation.SellTargetPricePerUnit = &value
		}
		if sellOrderExpiresAt.Valid {
			value := sellOrderExpiresAt.Time
			operation.SellOrderExpiresAt = &value
		}
		operations = append(operations, operation)
	}
	return operations, rows.Err()
}
