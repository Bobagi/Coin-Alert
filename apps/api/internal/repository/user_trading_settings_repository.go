package repository

import (
	"context"
	"database/sql"
	"errors"

	"coin-alert/internal/domain"
)

// UserTradingSettingsRepository persists bot/trading settings scoped to a single user AND a single
// Binance environment (one row per user per environment), so Testnet and Production are independent.
type UserTradingSettingsRepository interface {
	GetByUserAndEnvironment(lookupContext context.Context, userIdentifier int64, environment string) (*domain.UserTradingSettings, error)
	EnsureDefaults(operationContext context.Context, userIdentifier int64, environment string) (*domain.UserTradingSettings, error)
	Upsert(operationContext context.Context, settings domain.UserTradingSettings) error
}

type PostgresUserTradingSettingsRepository struct {
	Database *sql.DB
}

func NewPostgresUserTradingSettingsRepository(database *sql.DB) *PostgresUserTradingSettingsRepository {
	return &PostgresUserTradingSettingsRepository{Database: database}
}

// GetByUserAndEnvironment returns the settings row for a user+environment, or (nil, nil) when none exists.
func (repository *PostgresUserTradingSettingsRepository) GetByUserAndEnvironment(lookupContext context.Context, userIdentifier int64, environment string) (*domain.UserTradingSettings, error) {
	row := repository.Database.QueryRowContext(
		lookupContext,
		`SELECT user_id, trading_pair_symbol, capital_threshold, target_profit_percent,
		        stop_loss_percent, auto_sell_interval_minutes, daily_purchase_hour_utc,
		        daily_purchase_enabled, live_trading_enabled, active_binance_environment, binance_environment
		 FROM user_trading_settings WHERE user_id = $1 AND binance_environment = $2`,
		userIdentifier, environment,
	)

	settings := &domain.UserTradingSettings{}
	var stopLossPercent sql.NullFloat64
	scanError := row.Scan(
		&settings.UserIdentifier,
		&settings.TradingPairSymbol,
		&settings.CapitalThreshold,
		&settings.TargetProfitPercent,
		&stopLossPercent,
		&settings.AutomaticSellIntervalMinutes,
		&settings.DailyPurchaseHourUTC,
		&settings.DailyPurchaseEnabled,
		&settings.LiveTradingEnabled,
		&settings.ActiveBinanceEnvironment,
		&settings.BinanceEnvironment,
	)
	if errors.Is(scanError, sql.ErrNoRows) {
		return nil, nil
	}
	if scanError != nil {
		return nil, scanError
	}
	if stopLossPercent.Valid {
		stopLossValue := stopLossPercent.Float64
		settings.StopLossPercent = &stopLossValue
	}
	return settings, nil
}

// EnsureDefaults creates a default settings row for the user+environment if absent, then returns it.
func (repository *PostgresUserTradingSettingsRepository) EnsureDefaults(operationContext context.Context, userIdentifier int64, environment string) (*domain.UserTradingSettings, error) {
	_, insertError := repository.Database.ExecContext(
		operationContext,
		`INSERT INTO user_trading_settings (user_id, binance_environment) VALUES ($1, $2)
		 ON CONFLICT (user_id, binance_environment) DO NOTHING`,
		userIdentifier, environment,
	)
	if insertError != nil {
		return nil, insertError
	}
	return repository.GetByUserAndEnvironment(operationContext, userIdentifier, environment)
}

func (repository *PostgresUserTradingSettingsRepository) Upsert(operationContext context.Context, settings domain.UserTradingSettings) error {
	var stopLossArgument interface{}
	if settings.StopLossPercent != nil {
		stopLossArgument = *settings.StopLossPercent
	}

	_, executionError := repository.Database.ExecContext(
		operationContext,
		`INSERT INTO user_trading_settings (
		    user_id, trading_pair_symbol, capital_threshold, target_profit_percent,
		    stop_loss_percent, auto_sell_interval_minutes, daily_purchase_hour_utc,
		    daily_purchase_enabled, live_trading_enabled, active_binance_environment, binance_environment, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW())
		 ON CONFLICT (user_id, binance_environment) DO UPDATE SET
		    trading_pair_symbol = EXCLUDED.trading_pair_symbol,
		    capital_threshold = EXCLUDED.capital_threshold,
		    target_profit_percent = EXCLUDED.target_profit_percent,
		    stop_loss_percent = EXCLUDED.stop_loss_percent,
		    auto_sell_interval_minutes = EXCLUDED.auto_sell_interval_minutes,
		    daily_purchase_hour_utc = EXCLUDED.daily_purchase_hour_utc,
		    daily_purchase_enabled = EXCLUDED.daily_purchase_enabled,
		    live_trading_enabled = EXCLUDED.live_trading_enabled,
		    active_binance_environment = EXCLUDED.active_binance_environment,
		    updated_at = NOW()`,
		settings.UserIdentifier,
		settings.TradingPairSymbol,
		settings.CapitalThreshold,
		settings.TargetProfitPercent,
		stopLossArgument,
		settings.AutomaticSellIntervalMinutes,
		settings.DailyPurchaseHourUTC,
		settings.DailyPurchaseEnabled,
		settings.LiveTradingEnabled,
		settings.ActiveBinanceEnvironment,
		settings.BinanceEnvironment,
	)
	return executionError
}
