package domain

import "time"

type DailyPurchaseSettings struct {
	Identifier        int64
	TradingPairSymbol string
	PurchaseAmount    float64
	ExecutionHourUTC  int
	IsActive          bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
