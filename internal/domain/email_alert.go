package domain

import "time"

type EmailAlert struct {
	Identifier            int64
	RecipientAddress      string
	TradingPairOrCurrency string
	ThresholdValue        float64
	Subject               string
	MessageBody           string
	CreatedAt             time.Time
}
