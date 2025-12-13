package domain

import "time"

type Transaction struct {
    Identifier     int64
    OperationType  string
    AssetSymbol    string
    Quantity       float64
    PricePerUnit   float64
    Notes          string
    CreatedAt      time.Time
}

func (transaction Transaction) TotalValue() float64 {
    return transaction.Quantity * transaction.PricePerUnit
}
