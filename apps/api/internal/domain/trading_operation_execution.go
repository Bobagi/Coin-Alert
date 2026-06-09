package domain

import "time"

type TradingOperationExecution struct {
	Identifier           int64
	ScheduledOperationID *int64
	TradingPairSymbol    string
	OperationType        string
	BinanceEnvironment   string // the environment (TESTNET/PRODUCTION) this execution happened in
	InitiatedBy          string // who triggered it: ExecutionInitiatorUser or ExecutionInitiatorBot
	UnitPrice            float64
	Quantity             float64
	TotalValue           float64
	ExecutedAt           time.Time
	Success              bool
	ErrorMessage         *string
	OrderIdentifier      *string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

const (
	TradingOperationTypeBuy      = "BUY"
	TradingOperationTypeSell     = "SELL"
	TradingOperationTypeDailyBuy = "DAILY_BUY"
)

// Who triggered an execution.
const (
	ExecutionInitiatorUser = "USER"
	ExecutionInitiatorBot  = "BOT"
)
