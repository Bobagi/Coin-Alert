package repository

import (
	"context"
	"database/sql"
	"errors"

	"coin-alert/internal/domain"
)

// ErrRobotNotFound is returned when no robot matches the id for the given user.
var ErrRobotNotFound = errors.New("robot not found")

// ErrRobotSymbolExists is returned when a robot already exists for the same coin in the environment
// (the (user, environment, symbol) unique index was violated).
var ErrRobotSymbolExists = errors.New("a robot already exists for this coin in this environment")

const tradingRobotColumns = `id, user_id, binance_environment, trading_pair_symbol, COALESCE(name, ''),
	capital_threshold, target_profit_percent, stop_loss_percent, daily_purchase_hour_utc,
	daily_purchase_enabled, sell_order_validity_days, is_enabled, created_at, updated_at`

// TradingRobotRepository persists trading robots, always scoped to a single user (and usually a
// single Binance environment).
type TradingRobotRepository interface {
	ListRobotsForUser(loadContext context.Context, userIdentifier int64, environment string) ([]domain.TradingRobot, error)
	GetRobotForUser(loadContext context.Context, userIdentifier int64, robotIdentifier int64) (*domain.TradingRobot, error)
	CountRobotsForUser(loadContext context.Context, userIdentifier int64, environment string) (int, error)
	CreateRobotForUser(operationContext context.Context, userIdentifier int64, robot domain.TradingRobot) (int64, error)
	UpdateRobotForUser(operationContext context.Context, userIdentifier int64, robot domain.TradingRobot) error
	DeleteRobotForUser(operationContext context.Context, userIdentifier int64, robotIdentifier int64) error
}

type PostgresTradingRobotRepository struct {
	Database *sql.DB
}

func NewPostgresTradingRobotRepository(database *sql.DB) *PostgresTradingRobotRepository {
	return &PostgresTradingRobotRepository{Database: database}
}

func (repository *PostgresTradingRobotRepository) ListRobotsForUser(loadContext context.Context, userIdentifier int64, environment string) ([]domain.TradingRobot, error) {
	rows, queryError := repository.Database.QueryContext(
		loadContext,
		`SELECT `+tradingRobotColumns+` FROM trading_robots WHERE user_id = $1 AND binance_environment = $2 ORDER BY created_at ASC`,
		userIdentifier, environment,
	)
	if queryError != nil {
		return nil, queryError
	}
	defer rows.Close()
	return scanTradingRobotRows(rows)
}

func (repository *PostgresTradingRobotRepository) GetRobotForUser(loadContext context.Context, userIdentifier int64, robotIdentifier int64) (*domain.TradingRobot, error) {
	row := repository.Database.QueryRowContext(
		loadContext,
		`SELECT `+tradingRobotColumns+` FROM trading_robots WHERE id = $1 AND user_id = $2`,
		robotIdentifier, userIdentifier,
	)
	robot, scanError := scanTradingRobotRow(row)
	if errors.Is(scanError, sql.ErrNoRows) {
		return nil, ErrRobotNotFound
	}
	if scanError != nil {
		return nil, scanError
	}
	return robot, nil
}

func (repository *PostgresTradingRobotRepository) CountRobotsForUser(loadContext context.Context, userIdentifier int64, environment string) (int, error) {
	row := repository.Database.QueryRowContext(
		loadContext,
		`SELECT COUNT(*) FROM trading_robots WHERE user_id = $1 AND binance_environment = $2`,
		userIdentifier, environment,
	)
	var robotCount int
	if scanError := row.Scan(&robotCount); scanError != nil {
		return 0, scanError
	}
	return robotCount, nil
}

func (repository *PostgresTradingRobotRepository) CreateRobotForUser(operationContext context.Context, userIdentifier int64, robot domain.TradingRobot) (int64, error) {
	row := repository.Database.QueryRowContext(
		operationContext,
		`INSERT INTO trading_robots
		    (user_id, binance_environment, trading_pair_symbol, name, capital_threshold, target_profit_percent,
		     stop_loss_percent, daily_purchase_hour_utc, daily_purchase_enabled, sell_order_validity_days, is_enabled)
		 VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id`,
		userIdentifier,
		robot.BinanceEnvironment,
		robot.TradingPairSymbol,
		robot.Name,
		robot.CapitalThreshold,
		robot.TargetProfitPercent,
		nullableFloat(robot.StopLossPercent),
		robot.DailyPurchaseHourUTC,
		robot.DailyPurchaseEnabled,
		robot.SellOrderValidityDays,
		robot.IsEnabled,
	)
	var robotIdentifier int64
	if scanError := row.Scan(&robotIdentifier); scanError != nil {
		if isUniqueViolation(scanError) {
			return 0, ErrRobotSymbolExists
		}
		return 0, scanError
	}
	return robotIdentifier, nil
}

func (repository *PostgresTradingRobotRepository) UpdateRobotForUser(operationContext context.Context, userIdentifier int64, robot domain.TradingRobot) error {
	result, updateError := repository.Database.ExecContext(
		operationContext,
		`UPDATE trading_robots SET
		    name = NULLIF($1, ''),
		    capital_threshold = $2,
		    target_profit_percent = $3,
		    stop_loss_percent = $4,
		    daily_purchase_hour_utc = $5,
		    daily_purchase_enabled = $6,
		    sell_order_validity_days = $7,
		    is_enabled = $8,
		    updated_at = NOW()
		 WHERE id = $9 AND user_id = $10`,
		robot.Name,
		robot.CapitalThreshold,
		robot.TargetProfitPercent,
		nullableFloat(robot.StopLossPercent),
		robot.DailyPurchaseHourUTC,
		robot.DailyPurchaseEnabled,
		robot.SellOrderValidityDays,
		robot.IsEnabled,
		robot.Identifier,
		userIdentifier,
	)
	if updateError != nil {
		return updateError
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrRobotNotFound
	}
	return nil
}

func (repository *PostgresTradingRobotRepository) DeleteRobotForUser(operationContext context.Context, userIdentifier int64, robotIdentifier int64) error {
	result, deleteError := repository.Database.ExecContext(
		operationContext,
		`DELETE FROM trading_robots WHERE id = $1 AND user_id = $2`,
		robotIdentifier, userIdentifier,
	)
	if deleteError != nil {
		return deleteError
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrRobotNotFound
	}
	return nil
}

func nullableFloat(value *float64) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func scanTradingRobotRow(row *sql.Row) (*domain.TradingRobot, error) {
	robot := &domain.TradingRobot{}
	var stopLossPercent sql.NullFloat64
	scanError := row.Scan(
		&robot.Identifier,
		&robot.UserIdentifier,
		&robot.BinanceEnvironment,
		&robot.TradingPairSymbol,
		&robot.Name,
		&robot.CapitalThreshold,
		&robot.TargetProfitPercent,
		&stopLossPercent,
		&robot.DailyPurchaseHourUTC,
		&robot.DailyPurchaseEnabled,
		&robot.SellOrderValidityDays,
		&robot.IsEnabled,
		&robot.CreatedAt,
		&robot.UpdatedAt,
	)
	if scanError != nil {
		return nil, scanError
	}
	if stopLossPercent.Valid {
		value := stopLossPercent.Float64
		robot.StopLossPercent = &value
	}
	return robot, nil
}

func scanTradingRobotRows(rows *sql.Rows) ([]domain.TradingRobot, error) {
	robots := make([]domain.TradingRobot, 0)
	for rows.Next() {
		robot := domain.TradingRobot{}
		var stopLossPercent sql.NullFloat64
		scanError := rows.Scan(
			&robot.Identifier,
			&robot.UserIdentifier,
			&robot.BinanceEnvironment,
			&robot.TradingPairSymbol,
			&robot.Name,
			&robot.CapitalThreshold,
			&robot.TargetProfitPercent,
			&stopLossPercent,
			&robot.DailyPurchaseHourUTC,
			&robot.DailyPurchaseEnabled,
			&robot.SellOrderValidityDays,
			&robot.IsEnabled,
			&robot.CreatedAt,
			&robot.UpdatedAt,
		)
		if scanError != nil {
			return nil, scanError
		}
		if stopLossPercent.Valid {
			value := stopLossPercent.Float64
			robot.StopLossPercent = &value
		}
		robots = append(robots, robot)
	}
	return robots, rows.Err()
}
