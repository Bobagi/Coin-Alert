package domain

import "time"

const (
        TradingOperationStatusOpen = "OPEN"
        TradingOperationStatusSold = "SOLD"
)

type TradingOperation struct {
        Identifier           int64
        TradingPairSymbol    string
        QuantityPurchased    float64
        PurchasePricePerUnit float64
        TargetProfitPercent  float64
        Status               string
        SellPricePerUnit     float64
        PurchaseTimestamp    time.Time
        SellTimestamp        *time.Time
}

func (operation TradingOperation) PurchaseValueTotal() float64 {
        return operation.QuantityPurchased * operation.PurchasePricePerUnit
}

func (operation TradingOperation) TargetSellPricePerUnit() float64 {
        return operation.PurchasePricePerUnit * (1 + (operation.TargetProfitPercent / 100))
}

func (operation TradingOperation) HasReachedTarget(currentPricePerUnit float64) bool {
        return currentPricePerUnit >= operation.TargetSellPricePerUnit()
}
