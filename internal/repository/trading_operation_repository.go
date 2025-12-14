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
	ListOpenOperations(context.Context) ([]domain.TradingOperation, error)
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
	insertSQL := `INSERT INTO trading_operations(trading_pair_symbol, quantity_purchased, purchase_price_per_unit, target_profit_percent, status) VALUES($1, $2, $3, $4, $5) RETURNING id, purchased_at`
	statementContext, statementCancel := context.WithTimeout(contextWithTimeout, 5*time.Second)
	defer statementCancel()

	row := repository.Database.QueryRowContext(statementContext, insertSQL, operation.TradingPairSymbol, operation.QuantityPurchased, operation.PurchasePricePerUnit, operation.TargetProfitPercent, operation.Status)

	var identifier int64
	var purchasedAt time.Time
	scanError := row.Scan(&identifier, &purchasedAt)
	if scanError != nil {
		return 0, scanError
	}

	return identifier, nil
}

func (repository *PostgresTradingOperationRepository) ListRecentOperations(contextWithTimeout context.Context, limit int) ([]domain.TradingOperation, error) {
	querySQL := `SELECT id, trading_pair_symbol, quantity_purchased, purchase_price_per_unit, target_profit_percent, status, sell_price_per_unit, purchased_at, sold_at FROM trading_operations ORDER BY purchased_at DESC LIMIT $1`
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
		scanError := rows.Scan(&operation.Identifier, &operation.TradingPairSymbol, &operation.QuantityPurchased, &operation.PurchasePricePerUnit, &operation.TargetProfitPercent, &operation.Status, &sellPrice, &operation.PurchaseTimestamp, &operation.SellTimestamp)
		if scanError != nil {
			return nil, scanError
		}

		if sellPrice.Valid {
			sellPricePerUnit := sellPrice.Float64
			operation.SellPricePerUnit = &sellPricePerUnit
		}

		operations = append(operations, operation)
	}

	return operations, nil
}

func (repository *PostgresTradingOperationRepository) ListOpenOperations(contextWithTimeout context.Context) ([]domain.TradingOperation, error) {
	querySQL := `SELECT id, trading_pair_symbol, quantity_purchased, purchase_price_per_unit, target_profit_percent, status, sell_price_per_unit, purchased_at, sold_at FROM trading_operations WHERE status = $1 ORDER BY purchased_at ASC`
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
		scanError := rows.Scan(&operation.Identifier, &operation.TradingPairSymbol, &operation.QuantityPurchased, &operation.PurchasePricePerUnit, &operation.TargetProfitPercent, &operation.Status, &sellPrice, &operation.PurchaseTimestamp, &operation.SellTimestamp)
		if scanError != nil {
			return nil, scanError
		}

		if sellPrice.Valid {
			sellPricePerUnit := sellPrice.Float64
			operation.SellPricePerUnit = &sellPricePerUnit
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
