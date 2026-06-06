package domain

import "time"

type EmailAlert struct {
        Identifier            int64
        RecipientAddress      string
        TradingPairOrCurrency string
        MinimumThreshold      float64
        MaximumThreshold      float64
        IsActive              bool
        TriggeredAt           *time.Time
        CreatedAt             time.Time
}
