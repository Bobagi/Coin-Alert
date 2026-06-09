package service

import (
	"context"
	"log"
	"strconv"
	"time"

	"coin-alert/internal/domain"
	"coin-alert/internal/repository"
)

type activeUserLister interface {
	ListActiveUserIdentifiers(loadContext context.Context) ([]int64, error)
}

type dailyPurchaseGuard interface {
	HasSuccessfulExecutionOfTypeSince(loadContext context.Context, userIdentifier int64, environment string, operationType string, since time.Time) (bool, error)
}

// AutomationWorker runs per-user background trading automation: it reconciles filled take-profit
// orders, enforces stop-loss, and runs the daily DCA purchase. It iterates every active user that
// has connected Binance credentials.
type AutomationWorker struct {
	userLister          activeUserLister
	credentialService   *UserCredentialService
	settingsRepository  repository.UserTradingSettingsRepository
	operationRepository repository.UserTradingOperationRepository
	executionRepository repository.UserTradingOperationExecutionRepository
	purchaseGuard       dailyPurchaseGuard
	tradingService      *UserTradingService
	monitorInterval     time.Duration
}

func NewAutomationWorker(
	userLister activeUserLister,
	credentialService *UserCredentialService,
	settingsRepository repository.UserTradingSettingsRepository,
	operationRepository repository.UserTradingOperationRepository,
	executionRepository repository.UserTradingOperationExecutionRepository,
	purchaseGuard dailyPurchaseGuard,
	tradingService *UserTradingService,
	monitorInterval time.Duration,
) *AutomationWorker {
	if monitorInterval <= 0 {
		monitorInterval = 30 * time.Second
	}
	return &AutomationWorker{
		userLister:          userLister,
		credentialService:   credentialService,
		settingsRepository:  settingsRepository,
		operationRepository: operationRepository,
		executionRepository: executionRepository,
		purchaseGuard:       purchaseGuard,
		tradingService:      tradingService,
		monitorInterval:     monitorInterval,
	}
}

func (worker *AutomationWorker) Start(applicationContext context.Context) {
	go worker.runMonitorLoop(applicationContext)
	go worker.runDailyPurchaseLoop(applicationContext)
	log.Printf("Automation worker started (monitor interval %s)", worker.monitorInterval)
}

func (worker *AutomationWorker) runMonitorLoop(applicationContext context.Context) {
	ticker := time.NewTicker(worker.monitorInterval)
	defer ticker.Stop()
	for {
		select {
		case <-applicationContext.Done():
			log.Println("Automation monitor loop stopped")
			return
		case <-ticker.C:
			worker.monitorAllUsers(applicationContext)
		}
	}
}

func (worker *AutomationWorker) monitorAllUsers(applicationContext context.Context) {
	userIdentifiers, listError := worker.userLister.ListActiveUserIdentifiers(applicationContext)
	if listError != nil {
		log.Printf("automation: could not list active users: %v", listError)
		return
	}
	for _, userIdentifier := range userIdentifiers {
		worker.monitorUser(applicationContext, userIdentifier)
	}
}

func (worker *AutomationWorker) monitorUser(applicationContext context.Context, userIdentifier int64) {
	environmentConfiguration, configurationError := worker.credentialService.LoadActiveEnvironmentConfiguration(applicationContext, userIdentifier)
	if configurationError != nil || environmentConfiguration == nil {
		return
	}

	openOperations, listError := worker.operationRepository.ListOpenOperationsForUser(applicationContext, userIdentifier, environmentConfiguration.EnvironmentName)
	if listError != nil {
		log.Printf("automation: open operations for user %d failed: %v", userIdentifier, listError)
		return
	}
	if len(openOperations) == 0 {
		return
	}

	settings, _ := worker.settingsRepository.GetByUserAndEnvironment(applicationContext, userIdentifier, environmentConfiguration.EnvironmentName)
	tradingService := NewBinanceTradingService(*environmentConfiguration)
	priceService := NewBinancePriceService(*environmentConfiguration)
	priceBySymbol := make(map[string]float64)

	resolvePrice := func(tradingPairSymbol string) (float64, bool) {
		if cachedPrice, present := priceBySymbol[tradingPairSymbol]; present {
			return cachedPrice, true
		}
		currentPrice, priceError := priceService.GetCurrentPrice(applicationContext, tradingPairSymbol)
		if priceError != nil {
			return 0, false
		}
		priceBySymbol[tradingPairSymbol] = currentPrice
		return currentPrice, true
	}

	for _, openOperation := range openOperations {
		worker.processOpenOperation(applicationContext, userIdentifier, openOperation, settings, tradingService, resolvePrice)
	}
}

func (worker *AutomationWorker) processOpenOperation(applicationContext context.Context, userIdentifier int64, operation domain.TradingOperation, settings *domain.UserTradingSettings, tradingService *BinanceTradingService, resolvePrice func(string) (float64, bool)) {
	// 1) Reconcile the resting take-profit limit sell against Binance.
	if operation.SellOrderIdentifier != nil {
		orderStatus, statusError := tradingService.GetOrderStatus(applicationContext, operation.TradingPairSymbol, *operation.SellOrderIdentifier)
		if statusError == nil && orderStatus != nil {
			switch orderStatus.Status {
			case "FILLED":
				worker.markOperationSold(applicationContext, userIdentifier, operation, fillPriceFromStatus(*orderStatus, operation.PurchasePricePerUnit), "take-profit filled")
				return
			case "CANCELED", "EXPIRED", "REJECTED":
				// Removed outside the app (e.g. the user cancelled it in the Binance app).
				worker.markOperationCanceledExternally(applicationContext, userIdentifier, operation)
				return
			}
			// Still resting: enforce the app-side validity window (Binance spot LIMIT has no native expiry).
			if operation.SellOrderExpiresAt != nil && time.Now().After(*operation.SellOrderExpiresAt) {
				worker.expireSellOrder(applicationContext, userIdentifier, operation, tradingService)
				return
			}
		}
	}

	// 2) Stop-loss: if configured and price fell below the threshold, sell now.
	if settings == nil || settings.StopLossPercent == nil || *settings.StopLossPercent <= 0 {
		return
	}
	currentPrice, pricePresent := resolvePrice(operation.TradingPairSymbol)
	if !pricePresent {
		return
	}
	stopLossThreshold := operation.PurchasePricePerUnit * (1 - (*settings.StopLossPercent / 100))
	if currentPrice > stopLossThreshold {
		return
	}

	// Free the balance held by the resting limit sell before selling at market.
	if operation.SellOrderIdentifier != nil {
		if cancelError := tradingService.CancelOrder(applicationContext, operation.TradingPairSymbol, *operation.SellOrderIdentifier); cancelError != nil {
			// The cancel may have failed because the order just filled — reconcile that case.
			if orderStatus, statusError := tradingService.GetOrderStatus(applicationContext, operation.TradingPairSymbol, *operation.SellOrderIdentifier); statusError == nil && orderStatus != nil && orderStatus.Status == "FILLED" {
				worker.markOperationSold(applicationContext, userIdentifier, operation, fillPriceFromStatus(*orderStatus, operation.PurchasePricePerUnit), "take-profit filled")
			} else {
				log.Printf("automation: stop-loss cancel failed for operation %d (user %d): %v", operation.Identifier, userIdentifier, cancelError)
			}
			return
		}
	}

	sellResponse, sellError := tradingService.PlaceMarketSellByQuantity(applicationContext, operation.TradingPairSymbol, operation.QuantityPurchased)
	if sellError != nil {
		worker.logSellExecution(applicationContext, userIdentifier, operation.BinanceEnvironment, domain.ExecutionInitiatorBot, operation.TradingPairSymbol, currentPrice, operation.QuantityPurchased, false, sellError, nil)
		log.Printf("automation: stop-loss market sell failed for operation %d (user %d): %v", operation.Identifier, userIdentifier, sellError)
		return
	}
	worker.markOperationSold(applicationContext, userIdentifier, operation, fillPriceFromOrder(*sellResponse, currentPrice), "stop-loss")
}

func (worker *AutomationWorker) markOperationSold(applicationContext context.Context, userIdentifier int64, operation domain.TradingOperation, fillPrice float64, reason string) {
	if updateError := worker.operationRepository.UpdateOperationAsSoldForUser(applicationContext, userIdentifier, operation.Identifier, fillPrice); updateError != nil {
		log.Printf("automation: could not mark operation %d sold (user %d): %v", operation.Identifier, userIdentifier, updateError)
		return
	}
	worker.logSellExecution(applicationContext, userIdentifier, operation.BinanceEnvironment, domain.ExecutionInitiatorBot, operation.TradingPairSymbol, fillPrice, operation.QuantityPurchased, true, nil, operation.SellOrderIdentifier)
	log.Printf("automation: closed operation %d (user %d) via %s at %.8f", operation.Identifier, userIdentifier, reason, fillPrice)
}

// markOperationCanceledExternally handles a take-profit that was cancelled outside the app: it closes
// the operation as CANCELED (drops it from the active positions view) and records a history event.
func (worker *AutomationWorker) markOperationCanceledExternally(applicationContext context.Context, userIdentifier int64, operation domain.TradingOperation) {
	if updateError := worker.operationRepository.MarkOperationCanceledForUser(applicationContext, userIdentifier, operation.Identifier); updateError != nil {
		log.Printf("automation: could not mark operation %d canceled (user %d): %v", operation.Identifier, userIdentifier, updateError)
		return
	}
	worker.logTakeProfitEvent(applicationContext, userIdentifier, operation, domain.TradingOperationTypeSellCancel, domain.ExecutionInitiatorUser)
	log.Printf("automation: operation %d (user %d) take-profit cancelled externally; position released", operation.Identifier, userIdentifier)
}

// expireSellOrder cancels a take-profit that reached its validity window, leaving the position OPEN
// but unprotected (⚠) so the user can re-place it or sell. Records a history event.
func (worker *AutomationWorker) expireSellOrder(applicationContext context.Context, userIdentifier int64, operation domain.TradingOperation, tradingService *BinanceTradingService) {
	if operation.SellOrderIdentifier != nil {
		if cancelError := tradingService.CancelOrder(applicationContext, operation.TradingPairSymbol, *operation.SellOrderIdentifier); cancelError != nil {
			// If it actually filled meanwhile, reconcile to sold instead of expiring it.
			if orderStatus, statusError := tradingService.GetOrderStatus(applicationContext, operation.TradingPairSymbol, *operation.SellOrderIdentifier); statusError == nil && orderStatus != nil && orderStatus.Status == "FILLED" {
				worker.markOperationSold(applicationContext, userIdentifier, operation, fillPriceFromStatus(*orderStatus, operation.PurchasePricePerUnit), "take-profit filled")
				return
			}
			log.Printf("automation: could not cancel expired sell order for operation %d (user %d): %v", operation.Identifier, userIdentifier, cancelError)
			return
		}
	}
	if clearError := worker.operationRepository.ClearSellOrderForUser(applicationContext, userIdentifier, operation.Identifier); clearError != nil {
		log.Printf("automation: could not clear expired sell order for operation %d (user %d): %v", operation.Identifier, userIdentifier, clearError)
		return
	}
	worker.logTakeProfitEvent(applicationContext, userIdentifier, operation, domain.TradingOperationTypeSellExpire, domain.ExecutionInitiatorBot)
	log.Printf("automation: take-profit for operation %d (user %d) reached its validity and was cancelled", operation.Identifier, userIdentifier)
}

// logTakeProfitEvent records a non-trade history event (cancel/expire) for a take-profit order.
func (worker *AutomationWorker) logTakeProfitEvent(applicationContext context.Context, userIdentifier int64, operation domain.TradingOperation, operationType string, initiatedBy string) {
	_, _ = worker.executionRepository.LogExecutionForUser(applicationContext, userIdentifier, domain.TradingOperationExecution{
		TradingPairSymbol:  operation.TradingPairSymbol,
		OperationType:      operationType,
		BinanceEnvironment: operation.BinanceEnvironment,
		InitiatedBy:        initiatedBy,
		Quantity:           operation.QuantityPurchased,
		ExecutedAt:         time.Now(),
		Success:            true,
		OrderIdentifier:    operation.SellOrderIdentifier,
	})
}

func (worker *AutomationWorker) logSellExecution(applicationContext context.Context, userIdentifier int64, environment string, initiatedBy string, tradingPairSymbol string, unitPrice float64, quantity float64, success bool, cause error, orderIdentifier *string) {
	var errorMessage *string
	if cause != nil {
		message := cause.Error()
		errorMessage = &message
	}
	_, _ = worker.executionRepository.LogExecutionForUser(applicationContext, userIdentifier, domain.TradingOperationExecution{
		TradingPairSymbol:  tradingPairSymbol,
		OperationType:      domain.TradingOperationTypeSell,
		BinanceEnvironment: environment,
		InitiatedBy:        initiatedBy,
		UnitPrice:          unitPrice,
		Quantity:           quantity,
		TotalValue:         unitPrice * quantity,
		ExecutedAt:         time.Now(),
		Success:            success,
		ErrorMessage:       errorMessage,
		OrderIdentifier:    orderIdentifier,
	})
}

func (worker *AutomationWorker) runDailyPurchaseLoop(applicationContext context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-applicationContext.Done():
			log.Println("Automation daily purchase loop stopped")
			return
		case <-ticker.C:
			worker.processDailyPurchases(applicationContext)
		}
	}
}

func (worker *AutomationWorker) processDailyPurchases(applicationContext context.Context) {
	userIdentifiers, listError := worker.userLister.ListActiveUserIdentifiers(applicationContext)
	if listError != nil {
		return
	}

	nowUTC := time.Now().UTC()
	startOfDayUTC := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day(), 0, 0, 0, 0, time.UTC)

	for _, userIdentifier := range userIdentifiers {
		environmentConfiguration, _ := worker.credentialService.LoadActiveEnvironmentConfiguration(applicationContext, userIdentifier)
		if environmentConfiguration == nil {
			continue
		}
		environmentName := environmentConfiguration.EnvironmentName

		settings, _ := worker.settingsRepository.GetByUserAndEnvironment(applicationContext, userIdentifier, environmentName)
		if settings == nil || !settings.DailyPurchaseEnabled || settings.CapitalThreshold <= 0 {
			continue
		}
		if nowUTC.Hour() != settings.DailyPurchaseHourUTC {
			continue
		}
		alreadyPurchased, _ := worker.purchaseGuard.HasSuccessfulExecutionOfTypeSince(applicationContext, userIdentifier, environmentName, domain.TradingOperationTypeDailyBuy, startOfDayUTC)
		if alreadyPurchased {
			continue
		}

		log.Printf("automation: running daily purchase for user %d", userIdentifier)
		if _, purchaseError := worker.tradingService.ExecuteDailyPurchase(applicationContext, userIdentifier, environmentName, settings.TradingPairSymbol, settings.CapitalThreshold, settings.TargetProfitPercent); purchaseError != nil {
			log.Printf("automation: daily purchase failed for user %d: %v", userIdentifier, purchaseError)
		}
	}
}

func fillPriceFromStatus(orderStatus BinanceOrderStatus, fallbackPrice float64) float64 {
	executedQuantity, quantityError := strconv.ParseFloat(orderStatus.ExecutedQty, 64)
	cumulativeQuote, quoteError := strconv.ParseFloat(orderStatus.CumulativeQuote, 64)
	if quantityError == nil && quoteError == nil && executedQuantity > 0 && cumulativeQuote > 0 {
		return cumulativeQuote / executedQuantity
	}
	if parsedPrice, priceError := strconv.ParseFloat(orderStatus.Price, 64); priceError == nil && parsedPrice > 0 {
		return parsedPrice
	}
	return fallbackPrice
}

func fillPriceFromOrder(orderResponse binanceOrderResponse, fallbackPrice float64) float64 {
	executedQuantity, quantityError := strconv.ParseFloat(orderResponse.ExecutedQty, 64)
	cumulativeQuote, quoteError := strconv.ParseFloat(orderResponse.CumulativeQuote, 64)
	if quantityError == nil && quoteError == nil && executedQuantity > 0 && cumulativeQuote > 0 {
		return cumulativeQuote / executedQuantity
	}
	return fallbackPrice
}
