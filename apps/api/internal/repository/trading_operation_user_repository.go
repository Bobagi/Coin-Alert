package repository

import (
	"context"
	"database/sql"

	"coin-alert/internal/domain"
)

const userTradingOperationColumns = `id, trading_pair_symbol, quantity_purchased, purchase_price_per_unit,
	target_profit_percent, status, sell_price_per_unit, purchased_at, sold_at,
	buy_order_id, sell_order_id, sell_target_price_per_unit`

// UserTradingOperationRepository persists trading operations scoped to a single user.
type UserTradingOperationRepository interface {
	CreatePurchaseOperationForUser(operationContext context.Context, userIdentifier int64, operation domain.TradingOperation) (int64, error)
	ListRecentOperationsForUser(loadContext context.Context, userIdentifier int64, limit int) ([]domain.TradingOperation, error)
	ListOpenOperationsForUser(loadContext context.Context, userIdentifier int64) ([]domain.TradingOperation, error)
	UpdateOperationAsSoldForUser(operationContext context.Context, userIdentifier int64, operationIdentifier int64, sellPricePerUnit float64) error
	CalculateOpenAllocationTotalForUser(loadContext context.Context, userIdentifier int64) (float64, error)
}

func (repository *PostgresTradingOperationRepository) CreatePurchaseOperationForUser(operationContext context.Context, userIdentifier int64, operation domain.TradingOperation) (int64, error) {
	row := repository.Database.QueryRowContext(
		operationContext,
		`INSERT INTO trading_operations
		    (user_id, trading_pair_symbol, quantity_purchased, purchase_price_per_unit, target_profit_percent, status, buy_order_id, sell_order_id, sell_target_price_per_unit)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
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
	)
	var operationIdentifier int64
	if scanError := row.Scan(&operationIdentifier); scanError != nil {
		return 0, scanError
	}
	return operationIdentifier, nil
}

func (repository *PostgresTradingOperationRepository) ListRecentOperationsForUser(loadContext context.Context, userIdentifier int64, limit int) ([]domain.TradingOperation, error) {
	rows, queryError := repository.Database.QueryContext(
		loadContext,
		`SELECT `+userTradingOperationColumns+` FROM trading_operations WHERE user_id = $1 ORDER BY purchased_at DESC LIMIT $2`,
		userIdentifier, limit,
	)
	if queryError != nil {
		return nil, queryError
	}
	defer rows.Close()
	return scanUserTradingOperationRows(rows)
}

func (repository *PostgresTradingOperationRepository) ListOpenOperationsForUser(loadContext context.Context, userIdentifier int64) ([]domain.TradingOperation, error) {
	rows, queryError := repository.Database.QueryContext(
		loadContext,
		`SELECT `+userTradingOperationColumns+` FROM trading_operations WHERE user_id = $1 AND status = $2 ORDER BY purchased_at ASC`,
		userIdentifier, domain.TradingOperationStatusOpen,
	)
	if queryError != nil {
		return nil, queryError
	}
	defer rows.Close()
	return scanUserTradingOperationRows(rows)
}

func (repository *PostgresTradingOperationRepository) UpdateOperationAsSoldForUser(operationContext context.Context, userIdentifier int64, operationIdentifier int64, sellPricePerUnit float64) error {
	_, updateError := repository.Database.ExecContext(
		operationContext,
		`UPDATE trading_operations SET status = $1, sell_price_per_unit = $2, sold_at = NOW() WHERE id = $3 AND user_id = $4`,
		domain.TradingOperationStatusSold, sellPricePerUnit, operationIdentifier, userIdentifier,
	)
	return updateError
}

func (repository *PostgresTradingOperationRepository) CalculateOpenAllocationTotalForUser(loadContext context.Context, userIdentifier int64) (float64, error) {
	row := repository.Database.QueryRowContext(
		loadContext,
		`SELECT COALESCE(SUM(quantity_purchased * purchase_price_per_unit), 0) FROM trading_operations WHERE user_id = $1 AND status = $2`,
		userIdentifier, domain.TradingOperationStatusOpen,
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
		operations = append(operations, operation)
	}
	return operations, rows.Err()
}
