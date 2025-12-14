package domain

import "time"

type EmailAlert struct {
        Identifier            int64
        RecipientAddress      string
        TradingPairOrCurrency string
        ThresholdValue        float64
        CreatedAt             time.Time
}
