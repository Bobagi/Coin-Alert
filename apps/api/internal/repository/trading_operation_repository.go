package repository

import (
	"context"
	"database/sql"
	"time"

	"coin-alert/internal/domain"
)

type TradingOperationRepository interface {
	CreatePurchaseOperation(context.Context, domain.TradingOperation) (int64, error)
	ListRecentOperations(context.Context, int) ([]domain.TradingOperation, error)
	ListOperationsPage(context.Context, int, int) ([]domain.TradingOperation, error)
	ListOpenOperations(context.Context) ([]domain.TradingOperation, error)
	FindOldestOpenOperationForPair(context.Context, string) (*domain.TradingOperation, error)
	UpdateOperationAsSold(context.Context, int64, float64) error
	CalculateOpenAllocationTotal(context.Context) (float64, error)
}

type PostgresTradingOperationRepository struct {
	Database *sql.DB
}

func NewPostgresTradingOperationRepository(database *sql.DB) *PostgresTradingOperationRepository {
	return &PostgresTradingOperationRepository{Database: database}
}

func (repository *PostgresTradingOperationRepository) CreatePurchaseOperation(contextWithTimeout context.Context, operation domain.TradingOperation) (int64, error) {
	insertSQL := `INSERT INTO trading_operations(trading_pair_symbol, quantity_purchased, purchase_price_per_unit, target_profit_percent, status, buy_order_id, sell_order_id, sell_target_price_per_unit) VALUES($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id, purchased_at`
	statementContext, statementCancel := context.WithTimeout(contextWithTimeout, 5*time.Second)
	defer statementCancel()

	row := repository.Database.QueryRowContext(statementContext, insertSQL, operation.TradingPairSymbol, operation.QuantityPurchased, operation.PurchasePricePerUnit, operation.TargetProfitPercent, operation.Status, operation.BuyOrderIdentifier, operation.SellOrderIdentifier, operation.SellTargetPricePerUnit)

	var identifier int64
	var purchasedAt time.Time
	scanError := row.Scan(&identifier, &purchasedAt)
	if scanError != nil {
		return 0, scanError
	}

	return identifier, nil
}

func (repository *PostgresTradingOperationRepository) ListRecentOperations(contextWithTimeout context.Context, limit int) ([]domain.TradingOperation, error) {
	querySQL := `SELECT id, trading_pair_symbol, quantity_purchased, purchase_price_per_unit, target_profit_percent, status, sell_price_per_unit, purchased_at, sold_at, buy_order_id, sell_order_id, sell_target_price_per_unit FROM trading_operations ORDER BY purchased_at DESC LIMIT $1`
	queryContext, queryCancel := context.WithTimeout(contextWithTimeout, 5*time.Second)
	defer queryCancel()

	rows, queryError := repository.Database.QueryContext(queryContext, querySQL, limit)
	if queryError != nil {
		return nil, queryError
	}
	defer rows.Close()

	var operations []domain.TradingOperation
	for rows.Next() {
		var operation domain.TradingOperation
		var sellPrice sql.NullFloat64
		var buyOrderIdentifier sql.NullString
		var sellOrderIdentifier sql.NullString
		var sellTargetPrice sql.NullFloat64
		scanError := rows.Scan(&operation.Identifier, &operation.TradingPairSymbol, &operation.QuantityPurchased, &operation.PurchasePricePerUnit, &operation.TargetProfitPercent, &operation.Status, &sellPrice, &operation.PurchaseTimestamp, &operation.SellTimestamp, &buyOrderIdentifier, &sellOrderIdentifier, &sellTargetPrice)
		if scanError != nil {
			return nil, scanError
		}

		if sellPrice.Valid {
			sellPricePerUnit := sellPrice.Float64
			operation.SellPricePerUnit = &sellPricePerUnit
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

	return operations, nil
}

func (repository *PostgresTradingOperationRepository) ListOperationsPage(contextWithTimeout context.Context, limit int, offset int) ([]domain.TradingOperation, error) {
	querySQL := `SELECT id, trading_pair_symbol, quantity_purchased, purchase_price_per_unit, target_profit_percent, status, sell_price_per_unit, purchased_at, sold_at, buy_order_id, sell_order_id, sell_target_price_per_unit FROM trading_operations ORDER BY purchased_at DESC LIMIT $1 OFFSET $2`
	queryContext, queryCancel := context.WithTimeout(contextWithTimeout, 5*time.Second)
	defer queryCancel()

	rows, queryError := repository.Database.QueryContext(queryContext, querySQL, limit, offset)
	if queryError != nil {
		return nil, queryError
	}
	defer rows.Close()

	var operations []domain.TradingOperation
	for rows.Next() {
		var operation domain.TradingOperation
		var sellPrice sql.NullFloat64
		var buyOrderIdentifier sql.NullString
		var sellOrderIdentifier sql.NullString
		var sellTargetPrice sql.NullFloat64
		scanError := rows.Scan(&operation.Identifier, &operation.TradingPairSymbol, &operation.QuantityPurchased, &operation.PurchasePricePerUnit, &operation.TargetProfitPercent, &operation.Status, &sellPrice, &operation.PurchaseTimestamp, &operation.SellTimestamp, &buyOrderIdentifier, &sellOrderIdentifier, &sellTargetPrice)
		if scanError != nil {
			return nil, scanError
		}

		if sellPrice.Valid {
			sellPricePerUnit := sellPrice.Float64
			operation.SellPricePerUnit = &sellPricePerUnit
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

	return operations, nil
}

func (repository *PostgresTradingOperationRepository) ListOpenOperations(contextWithTimeout context.Context) ([]domain.TradingOperation, error) {
	querySQL := `SELECT id, trading_pair_symbol, quantity_purchased, purchase_price_per_unit, target_profit_percent, status, sell_price_per_unit, purchased_at, sold_at, buy_order_id, sell_order_id, sell_target_price_per_unit FROM trading_operations WHERE status = $1 ORDER BY purchased_at ASC`
	queryContext, queryCancel := context.WithTimeout(contextWithTimeout, 5*time.Second)
	defer queryCancel()

	rows, queryError := repository.Database.QueryContext(queryContext, querySQL, domain.TradingOperationStatusOpen)
	if queryError != nil {
		return nil, queryError
	}
	defer rows.Close()

	var operations []domain.TradingOperation
	for rows.Next() {
		var operation domain.TradingOperation
		var sellPrice sql.NullFloat64
		var buyOrderIdentifier sql.NullString
		var sellOrderIdentifier sql.NullString
		var sellTargetPrice sql.NullFloat64
		scanError := rows.Scan(&operation.Identifier, &operation.TradingPairSymbol, &operation.QuantityPurchased, &operation.PurchasePricePerUnit, &operation.TargetProfitPercent, &operation.Status, &sellPrice, &operation.PurchaseTimestamp, &operation.SellTimestamp, &buyOrderIdentifier, &sellOrderIdentifier, &sellTargetPrice)
		if scanError != nil {
			return nil, scanError
		}

		if sellPrice.Valid {
			sellPricePerUnit := sellPrice.Float64
			operation.SellPricePerUnit = &sellPricePerUnit
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

	return operations, nil
}

func (repository *PostgresTradingOperationRepository) UpdateOperationAsSold(contextWithTimeout context.Context, operationIdentifier int64, sellPricePerUnit float64) error {
	updateSQL := `UPDATE trading_operations SET status = $1, sell_price_per_unit = $2, sold_at = NOW() WHERE id = $3`
	updateContext, updateCancel := context.WithTimeout(contextWithTimeout, 5*time.Second)
	defer updateCancel()

	_, updateError := repository.Database.ExecContext(updateContext, updateSQL, domain.TradingOperationStatusSold, sellPricePerUnit, operationIdentifier)
	return updateError
}

func (repository *PostgresTradingOperationRepository) CalculateOpenAllocationTotal(contextWithTimeout context.Context) (float64, error) {
	sumSQL := `SELECT COALESCE(SUM(quantity_purchased * purchase_price_per_unit), 0) FROM trading_operations WHERE status = $1`
	sumContext, sumCancel := context.WithTimeout(contextWithTimeout, 5*time.Second)
	defer sumCancel()

	row := repository.Database.QueryRowContext(sumContext, sumSQL, domain.TradingOperationStatusOpen)
	var totalAllocated float64
	scanError := row.Scan(&totalAllocated)
	if scanError != nil {
		return 0, scanError
	}

	return totalAllocated, nil
}

func (repository *PostgresTradingOperationRepository) FindOldestOpenOperationForPair(contextWithTimeout context.Context, tradingPairSymbol string) (*domain.TradingOperation, error) {
	querySQL := `SELECT id, trading_pair_symbol, quantity_purchased, purchase_price_per_unit, target_profit_percent, status, sell_price_per_unit, purchased_at, sold_at, buy_order_id, sell_order_id, sell_target_price_per_unit FROM trading_operations WHERE status = $1 AND trading_pair_symbol = $2 ORDER BY purchased_at ASC LIMIT 1`
	queryContext, queryCancel := context.WithTimeout(contextWithTimeout, 5*time.Second)
	defer queryCancel()

	row := repository.Database.QueryRowContext(queryContext, querySQL, domain.TradingOperationStatusOpen, tradingPairSymbol)

	var operation domain.TradingOperation
	var sellPrice sql.NullFloat64
	var buyOrderIdentifier sql.NullString
	var sellOrderIdentifier sql.NullString
	var sellTargetPrice sql.NullFloat64
	scanError := row.Scan(&operation.Identifier, &operation.TradingPairSymbol, &operation.QuantityPurchased, &operation.PurchasePricePerUnit, &operation.TargetProfitPercent, &operation.Status, &sellPrice, &operation.PurchaseTimestamp, &operation.SellTimestamp, &buyOrderIdentifier, &sellOrderIdentifier, &sellTargetPrice)
	if scanError != nil {
		if scanError == sql.ErrNoRows {
			return nil, nil
		}
		return nil, scanError
	}

	if sellPrice.Valid {
		sellPricePerUnit := sellPrice.Float64
		operation.SellPricePerUnit = &sellPricePerUnit
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

	return &operation, nil
}
