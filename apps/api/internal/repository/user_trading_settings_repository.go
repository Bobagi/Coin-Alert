package repository

import (
	"context"
	"database/sql"
	"errors"

	"coin-alert/internal/domain"
)

type UserTradingSettingsRepository interface {
	GetByUserIdentifier(lookupContext context.Context, userIdentifier int64) (*domain.UserTradingSettings, error)
	EnsureDefaults(operationContext context.Context, userIdentifier int64) (*domain.UserTradingSettings, error)
	Upsert(operationContext context.Context, settings domain.UserTradingSettings) error
}

type PostgresUserTradingSettingsRepository struct {
	Database *sql.DB
}

func NewPostgresUserTradingSettingsRepository(database *sql.DB) *PostgresUserTradingSettingsRepository {
	return &PostgresUserTradingSettingsRepository{Database: database}
}

// GetByUserIdentifier returns the settings row for a user, or (nil, nil) when none exists yet.
func (repository *PostgresUserTradingSettingsRepository) GetByUserIdentifier(lookupContext context.Context, userIdentifier int64) (*domain.UserTradingSettings, error) {
	row := repository.Database.QueryRowContext(
		lookupContext,
		`SELECT user_id, trading_pair_symbol, capital_threshold, target_profit_percent,
		        stop_loss_percent, auto_sell_interval_minutes, daily_purchase_hour_utc,
		        live_trading_enabled, active_binance_environment
		 FROM user_trading_settings WHERE user_id = $1`,
		userIdentifier,
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
		&settings.LiveTradingEnabled,
		&settings.ActiveBinanceEnvironment,
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

// EnsureDefaults creates a default settings row for the user if absent, then returns it.
func (repository *PostgresUserTradingSettingsRepository) EnsureDefaults(operationContext context.Context, userIdentifier int64) (*domain.UserTradingSettings, error) {
	_, insertError := repository.Database.ExecContext(
		operationContext,
		`INSERT INTO user_trading_settings (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`,
		userIdentifier,
	)
	if insertError != nil {
		return nil, insertError
	}
	return repository.GetByUserIdentifier(operationContext, userIdentifier)
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
		    live_trading_enabled, active_binance_environment, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		 ON CONFLICT (user_id) DO UPDATE SET
		    trading_pair_symbol = EXCLUDED.trading_pair_symbol,
		    capital_threshold = EXCLUDED.capital_threshold,
		    target_profit_percent = EXCLUDED.target_profit_percent,
		    stop_loss_percent = EXCLUDED.stop_loss_percent,
		    auto_sell_interval_minutes = EXCLUDED.auto_sell_interval_minutes,
		    daily_purchase_hour_utc = EXCLUDED.daily_purchase_hour_utc,
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
		settings.LiveTradingEnabled,
		settings.ActiveBinanceEnvironment,
	)
	return executionError
}
