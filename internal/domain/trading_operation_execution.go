package domain

import "time"

type TradingOperationExecution struct {
        Identifier           int64
        ScheduledOperationID *int64
        TradingPairSymbol    string
        OperationType        string
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
        TradingOperationTypeBuy  = "BUY"
        TradingOperationTypeSell = "SELL"
)
