package domain

import "time"

type ScheduledTradingOperation struct {
        Identifier              int64
        TradingPairSymbol       string
        CapitalThreshold        float64
        TargetProfitPercent     float64
        OperationType           string
        ScheduledExecutionTime  time.Time
        Status                  string
        CreatedAt               time.Time
        UpdatedAt               time.Time
}

const (
        ScheduledOperationStatusScheduled = "SCHEDULED"
        ScheduledOperationStatusExecuting = "EXECUTING"
        ScheduledOperationStatusCancelled = "CANCELLED"
)

func (operation ScheduledTradingOperation) IsNextCandidate() bool {
        return operation.Status == ScheduledOperationStatusScheduled
}
