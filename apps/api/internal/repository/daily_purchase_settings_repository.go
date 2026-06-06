package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"coin-alert/internal/domain"
)

type DailyPurchaseSettingsRepository interface {
	UpsertActiveSettings(context.Context, domain.DailyPurchaseSettings) (int64, error)
	LoadActiveSettings(context.Context) (*domain.DailyPurchaseSettings, error)
}

type PostgresDailyPurchaseSettingsRepository struct {
	Database *sql.DB
}

func NewPostgresDailyPurchaseSettingsRepository(database *sql.DB) *PostgresDailyPurchaseSettingsRepository {
	return &PostgresDailyPurchaseSettingsRepository{Database: database}
}

func (repository *PostgresDailyPurchaseSettingsRepository) UpsertActiveSettings(contextWithTimeout context.Context, settings domain.DailyPurchaseSettings) (int64, error) {
	transaction, transactionError := repository.Database.BeginTx(contextWithTimeout, nil)
	if transactionError != nil {
		return 0, transactionError
	}

	_, deactivateError := transaction.ExecContext(contextWithTimeout, "UPDATE daily_purchase_settings SET is_active = false, updated_at = NOW() WHERE is_active = true")
	if deactivateError != nil {
		transaction.Rollback()
		return 0, deactivateError
	}

	insertSQL := `INSERT INTO daily_purchase_settings (trading_pair_symbol, purchase_amount, execution_hour_utc, is_active) VALUES ($1, $2, $3, $4) RETURNING id`
	row := transaction.QueryRowContext(contextWithTimeout, insertSQL, settings.TradingPairSymbol, settings.PurchaseAmount, settings.ExecutionHourUTC, true)
	var identifier int64
	scanError := row.Scan(&identifier)
	if scanError != nil {
		transaction.Rollback()
		return 0, scanError
	}

	commitError := transaction.Commit()
	if commitError != nil {
		return 0, commitError
	}

	return identifier, nil
}

func (repository *PostgresDailyPurchaseSettingsRepository) LoadActiveSettings(contextWithTimeout context.Context) (*domain.DailyPurchaseSettings, error) {
	querySQL := `SELECT id, trading_pair_symbol, purchase_amount, execution_hour_utc, is_active, created_at, updated_at FROM daily_purchase_settings WHERE is_active = true ORDER BY created_at DESC LIMIT 1`
	queryContext, queryCancel := context.WithTimeout(contextWithTimeout, 5*time.Second)
	defer queryCancel()

	row := repository.Database.QueryRowContext(queryContext, querySQL)
	var settings domain.DailyPurchaseSettings
	scanError := row.Scan(&settings.Identifier, &settings.TradingPairSymbol, &settings.PurchaseAmount, &settings.ExecutionHourUTC, &settings.IsActive, &settings.CreatedAt, &settings.UpdatedAt)
	if scanError != nil {
		if errors.Is(scanError, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, scanError
	}

	return &settings, nil
}
