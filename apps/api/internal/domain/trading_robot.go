package domain

import "time"

// TradingRobot is one automated trading bot scoped to a single coin/pair within a single Binance
// environment. A user can run several robots (one per coin). The fields mirror the per-coin bot
// configuration that used to live in UserTradingSettings.
type TradingRobot struct {
	Identifier            int64
	UserIdentifier        int64
	BinanceEnvironment    string
	TradingPairSymbol     string
	Name                  string
	CapitalThreshold      float64
	TargetProfitPercent   float64
	StopLossPercent       *float64 // nil means no stop-loss configured
	DailyPurchaseHourUTC  int
	DailyPurchaseEnabled  bool
	SellOrderValidityDays int // 0 = no expiry (GTC)
	IsEnabled             bool
	CreatedAt             time.Time
	UpdatedAt             time.Time
}
