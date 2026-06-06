package domain

import "time"

// User is an authenticated account that owns its own credentials, settings, and trades.
type User struct {
	Identifier   int64
	Email        string
	PasswordHash string
	DisplayName  string
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// UserTradingSettings holds the per-user configuration that used to be process-global.
type UserTradingSettings struct {
	UserIdentifier               int64
	TradingPairSymbol            string
	CapitalThreshold             float64
	TargetProfitPercent          float64
	StopLossPercent              *float64 // nil means no stop-loss configured
	AutomaticSellIntervalMinutes int
	DailyPurchaseHourUTC         int
	LiveTradingEnabled           bool
	ActiveBinanceEnvironment     string
}
