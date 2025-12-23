package service

import (
	"context"
	"log"
	"time"

	"coin-alert/internal/domain"
)

type TradingAutomationService struct {
	TradingOperationService *TradingOperationService
	BinancePriceService     *BinancePriceService
	TradingPairSymbol       string
	AutomaticSellInterval   time.Duration
	TradingScheduleService  *TradingScheduleService
}

func NewTradingAutomationService(tradingOperationService *TradingOperationService, binancePriceService *BinancePriceService, tradingScheduleService *TradingScheduleService, tradingPairSymbol string, automaticSellIntervalMinutes int) *TradingAutomationService {
	return &TradingAutomationService{
		TradingOperationService: tradingOperationService,
		BinancePriceService:     binancePriceService,
		TradingScheduleService:  tradingScheduleService,
		TradingPairSymbol:       tradingPairSymbol,
		AutomaticSellInterval:   time.Duration(automaticSellIntervalMinutes) * time.Minute,
	}
}

func (service *TradingAutomationService) StartBackgroundJobs(applicationContext context.Context) {
	go service.startAutomaticSellLoop(applicationContext)
}

func (service *TradingAutomationService) ScheduleSellIfOpenPositionExists(applicationContext context.Context) {
	service.ensureNextOperationScheduled(applicationContext)
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
			service.EvaluateAndSellProfitableOperations(applicationContext)
		}
	}
}

func (service *TradingAutomationService) EvaluateAndSellProfitableOperations(applicationContext context.Context) {
	priceLookupContext, priceLookupCancel := context.WithTimeout(applicationContext, 10*time.Second)
	defer priceLookupCancel()

	currentPrice, priceError := service.BinancePriceService.GetCurrentPrice(priceLookupContext, service.TradingPairSymbol)
	if priceError != nil {
		log.Printf("Could not fetch current price for %s: %v", service.TradingPairSymbol, priceError)
		service.recordExecutionFailure(applicationContext, priceError)
		return
	}

	scheduleContext, scheduleCancel := context.WithTimeout(applicationContext, 5*time.Second)
	defer scheduleCancel()
	scheduledOperation, scheduleError := service.TradingScheduleService.StartExecutionForNextOperation(scheduleContext)
	if scheduleError != nil {
		log.Printf("Could not update scheduled operation status: %v", scheduleError)
	}

	openOperations, openFetchError := service.TradingOperationService.ListOpenOperations(priceLookupContext)
	if openFetchError != nil {
		log.Printf("Could not list open operations: %v", openFetchError)
	}

	totalQuantitySold := 0.0
	totalValueSold := 0.0
	for _, openOperation := range openOperations {
		if openOperation.HasReachedTarget(currentPrice) {
			totalQuantitySold += openOperation.QuantityPurchased
			totalValueSold += openOperation.QuantityPurchased * currentPrice
		}
	}

	closeContext, closeCancel := context.WithTimeout(applicationContext, 10*time.Second)
	defer closeCancel()

	closeError := service.TradingOperationService.CloseOperationsThatReachedTargetPrice(closeContext, currentPrice)
	if closeError != nil {
		log.Printf("Could not close profitable operations: %v", closeError)
		service.recordExecutionFailure(applicationContext, closeError)
	} else {
		service.recordExecutionSuccess(applicationContext, scheduledOperation, currentPrice, totalQuantitySold, totalValueSold)
	}

	service.ensureNextOperationScheduled(applicationContext)
}

func (service *TradingAutomationService) ensureNextOperationScheduled(applicationContext context.Context) {
	scheduleContext, scheduleCancel := context.WithTimeout(applicationContext, 5*time.Second)
	defer scheduleCancel()
	nextOperation, nextFetchError := service.TradingScheduleService.GetNextScheduledOperation(scheduleContext)
	if nextFetchError != nil {
		log.Printf("Could not fetch next scheduled operation: %v", nextFetchError)
		return
	}
	if nextOperation != nil {
		return
	}

	openOperation, openOperationError := service.TradingOperationService.FindOldestOpenOperationForPair(scheduleContext, service.TradingPairSymbol)
	if openOperationError != nil {
		log.Printf("Could not determine open position for scheduling: %v", openOperationError)
		return
	}
	if openOperation == nil {
		return
	}

	_, enqueueError := service.TradingScheduleService.EnqueueNextSellOperation(scheduleContext)
	if enqueueError != nil {
		log.Printf("Could not enqueue next scheduled operation: %v", enqueueError)
	}
}

func (service *TradingAutomationService) recordExecutionFailure(applicationContext context.Context, cause error) {
	executionContext, executionCancel := context.WithTimeout(applicationContext, 5*time.Second)
	defer executionCancel()
	errorMessage := cause.Error()
	executionRecord := domain.TradingOperationExecution{
		TradingPairSymbol: service.TradingPairSymbol,
		OperationType:     domain.TradingOperationTypeSell,
		UnitPrice:         0,
		Quantity:          0,
		TotalValue:        0,
		ExecutedAt:        time.Now(),
		Success:           false,
		ErrorMessage:      &errorMessage,
	}
	_, logError := service.TradingScheduleService.LogExecution(executionContext, executionRecord)
	if logError != nil {
		log.Printf("Could not log failed execution: %v", logError)
	}
}

func (service *TradingAutomationService) recordExecutionSuccess(applicationContext context.Context, scheduledOperation *domain.ScheduledTradingOperation, currentPrice float64, totalQuantity float64, totalValue float64) {
	executionContext, executionCancel := context.WithTimeout(applicationContext, 5*time.Second)
	defer executionCancel()
	executionRecord := domain.TradingOperationExecution{
		TradingPairSymbol: service.TradingPairSymbol,
		OperationType:     domain.TradingOperationTypeSell,
		UnitPrice:         currentPrice,
		Quantity:          totalQuantity,
		TotalValue:        totalValue,
		ExecutedAt:        time.Now(),
		Success:           true,
	}
	if scheduledOperation != nil {
		executionRecord.ScheduledOperationID = &scheduledOperation.Identifier
	}
	_, logError := service.TradingScheduleService.LogExecution(executionContext, executionRecord)
	if logError != nil {
		log.Printf("Could not log execution: %v", logError)
	}

	if scheduledOperation != nil {
		completionError := service.TradingScheduleService.CompleteScheduledOperation(executionContext, scheduledOperation.Identifier)
		if completionError != nil {
			log.Printf("Could not complete scheduled operation: %v", completionError)
		}
	}
}
