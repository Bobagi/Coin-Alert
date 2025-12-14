package service

import (
        "context"
        "log"
        "time"
)

type TradingAutomationService struct {
        TradingOperationService *TradingOperationService
        BinancePriceService     *BinancePriceService
        TradingPairSymbol       string
        AutomaticSellInterval   time.Duration
}

func NewTradingAutomationService(tradingOperationService *TradingOperationService, binancePriceService *BinancePriceService, tradingPairSymbol string, automaticSellIntervalMinutes int) *TradingAutomationService {
        return &TradingAutomationService{
                TradingOperationService: tradingOperationService,
                BinancePriceService:     binancePriceService,
                TradingPairSymbol:       tradingPairSymbol,
                AutomaticSellInterval:   time.Duration(automaticSellIntervalMinutes) * time.Minute,
        }
}

func (service *TradingAutomationService) StartBackgroundJobs(applicationContext context.Context) {
        go service.startAutomaticSellLoop(applicationContext)
}

func (service *TradingAutomationService) startAutomaticSellLoop(applicationContext context.Context) {
        ticker := time.NewTicker(service.AutomaticSellInterval)
        defer ticker.Stop()

        for {
            select {
            case <-applicationContext.Done():
                log.Println("Automatic sell loop stopped")
                return
            case <-ticker.C:
                service.evaluateAndSellProfitableOperations(applicationContext)
            }
        }
}

func (service *TradingAutomationService) evaluateAndSellProfitableOperations(applicationContext context.Context) {
        priceLookupContext, cancel := context.WithTimeout(applicationContext, 10*time.Second)
        defer cancel()

        currentPrice, priceError := service.BinancePriceService.GetCurrentPrice(priceLookupContext, service.TradingPairSymbol)
        if priceError != nil {
            log.Printf("Could not fetch current price for %s: %v", service.TradingPairSymbol, priceError)
            return
        }

        closeContext, closeCancel := context.WithTimeout(applicationContext, 10*time.Second)
        defer closeCancel()

        closeError := service.TradingOperationService.CloseOperationsThatReachedTargetPrice(closeContext, currentPrice)
        if closeError != nil {
            log.Printf("Could not close profitable operations: %v", closeError)
        }
}
