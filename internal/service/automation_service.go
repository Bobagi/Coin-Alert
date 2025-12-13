package service

import (
    "context"
    "log"
    "time"

    "coin-alert/internal/domain"
)

type AutomationService struct {
    TransactionService *TransactionService
    AutomaticSellIntervalMinutes int
    DailyPurchaseIntervalMinutes int
}

func NewAutomationService(transactionService *TransactionService, automaticSellIntervalMinutes int, dailyPurchaseIntervalMinutes int) *AutomationService {
    return &AutomationService{
        TransactionService: transactionService,
        AutomaticSellIntervalMinutes: automaticSellIntervalMinutes,
        DailyPurchaseIntervalMinutes: dailyPurchaseIntervalMinutes,
    }
}

func (service *AutomationService) StartBackgroundJobs(applicationContext context.Context) {
    go service.startAutomaticSellLoop(applicationContext)
    go service.startDailyPurchaseLoop(applicationContext)
}

func (service *AutomationService) startAutomaticSellLoop(applicationContext context.Context) {
    ticker := time.NewTicker(time.Duration(service.AutomaticSellIntervalMinutes) * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-applicationContext.Done():
            log.Println("Automatic sell loop stopped")
            return
        case <-ticker.C:
            service.executeAutomaticSell(applicationContext)
        }
    }
}

func (service *AutomationService) startDailyPurchaseLoop(applicationContext context.Context) {
    ticker := time.NewTicker(time.Duration(service.DailyPurchaseIntervalMinutes) * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-applicationContext.Done():
            log.Println("Daily purchase loop stopped")
            return
        case <-ticker.C:
            service.executeDailyPurchase(applicationContext)
        }
    }
}

func (service *AutomationService) executeAutomaticSell(applicationContext context.Context) {
    automaticSellTransaction := domain.Transaction{
        OperationType: "SELL",
        AssetSymbol:   "BTC",
        Quantity:      0.0001,
        PricePerUnit:  1,
        Notes:         "Scheduled automatic sell placeholder",
    }

    _, transactionError := service.TransactionService.RecordTransaction(applicationContext, automaticSellTransaction)
    if transactionError != nil {
        log.Printf("Automatic sell failed: %v", transactionError)
    } else {
        log.Println("Automatic sell recorded")
    }
}

func (service *AutomationService) executeDailyPurchase(applicationContext context.Context) {
    dailyPurchaseTransaction := domain.Transaction{
        OperationType: "BUY",
        AssetSymbol:   "BTC",
        Quantity:      0.0001,
        PricePerUnit:  1,
        Notes:         "Scheduled daily purchase placeholder",
    }

    _, transactionError := service.TransactionService.RecordTransaction(applicationContext, dailyPurchaseTransaction)
    if transactionError != nil {
        log.Printf("Daily purchase failed: %v", transactionError)
    } else {
        log.Println("Daily purchase recorded")
    }
}
