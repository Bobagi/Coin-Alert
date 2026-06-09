package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"coin-alert/internal/domain"
	"coin-alert/internal/repository"
)

// UserTradingService orchestrates per-user trades: it loads the user's decrypted credentials,
// places a market buy plus a take-profit limit sell, and records the operation and executions.
// Operations, executions and settings are scoped to the user's ACTIVE Binance environment.
type UserTradingService struct {
	credentialService   *UserCredentialService
	settingsRepository  repository.UserTradingSettingsRepository
	operationRepository repository.UserTradingOperationRepository
	executionRepository repository.UserTradingOperationExecutionRepository
}

func NewUserTradingService(credentialService *UserCredentialService, settingsRepository repository.UserTradingSettingsRepository, operationRepository repository.UserTradingOperationRepository, executionRepository repository.UserTradingOperationExecutionRepository) *UserTradingService {
	return &UserTradingService{
		credentialService:   credentialService,
		settingsRepository:  settingsRepository,
		operationRepository: operationRepository,
		executionRepository: executionRepository,
	}
}

// ExecuteBuy places a market buy for the given quote amount and an immediate take-profit limit sell.
// initiatedBy records whether a user or the bot triggered it. Real-money (PRODUCTION) orders are
// refused unless the user explicitly enabled live trading.
func (service *UserTradingService) ExecuteBuy(operationContext context.Context, userIdentifier int64, initiatedBy string, tradingPairSymbol string, quoteAmount float64, targetProfitPercent float64) (*domain.TradingOperation, error) {
	tradingPairSymbol = strings.ToUpper(strings.TrimSpace(tradingPairSymbol))
	if tradingPairSymbol == "" {
		return nil, errors.New("a trading pair is required")
	}
	if quoteAmount <= 0 {
		return nil, errors.New("the buy amount must be greater than zero")
	}

	environmentConfiguration, configurationError := service.credentialService.LoadActiveEnvironmentConfiguration(operationContext, userIdentifier)
	if configurationError != nil {
		return nil, configurationError
	}
	if environmentConfiguration == nil {
		return nil, errors.New("connect a Binance account before trading")
	}
	environmentName := environmentConfiguration.EnvironmentName

	settings, _ := service.settingsRepository.GetByUserAndEnvironment(operationContext, userIdentifier, environmentName)
	if targetProfitPercent <= 0 && settings != nil {
		targetProfitPercent = settings.TargetProfitPercent
	}
	if targetProfitPercent <= 0 {
		targetProfitPercent = 1.0
	}
	if environmentName == domain.BinanceEnvironmentProduction && (settings == nil || !settings.LiveTradingEnabled) {
		return nil, errors.New("enable live trading in your settings before placing real-money orders")
	}

	tradingService := NewBinanceTradingService(*environmentConfiguration)
	priceService := NewBinancePriceService(*environmentConfiguration)

	// Check the order value against the pair's minimum BEFORE buying, so the user gets a clear
	// message instead of a raw Binance -1013 NOTIONAL rejection.
	symbolFilters, _ := tradingService.FetchSymbolFilters(operationContext, tradingPairSymbol)
	if symbolFilters.MinNotional > 0 && quoteAmount < symbolFilters.MinNotional {
		return nil, fmt.Errorf("the minimum order value for %s is %s — you entered %s", tradingPairSymbol, formatDecimal(symbolFilters.MinNotional), formatDecimal(quoteAmount))
	}

	currentPricePerUnit, priceError := priceService.GetCurrentPrice(operationContext, tradingPairSymbol)
	if priceError != nil {
		return nil, fmt.Errorf("could not fetch the current price: %w", priceError)
	}
	if currentPricePerUnit <= 0 {
		return nil, errors.New("the current price is unavailable for this pair")
	}

	buyOrderResponse, buyError := tradingService.PlaceMarketBuyByQuote(operationContext, tradingPairSymbol, quoteAmount)
	if buyError != nil {
		service.logExecution(operationContext, userIdentifier, environmentName, initiatedBy, tradingPairSymbol, domain.TradingOperationTypeBuy, 0, 0, 0, false, buyError, nil)
		return nil, buyError
	}

	executedQuantity, _ := strconv.ParseFloat(buyOrderResponse.ExecutedQty, 64)
	if executedQuantity <= 0 {
		invalidQuantityError := errors.New("Binance returned an invalid executed quantity")
		service.logExecution(operationContext, userIdentifier, environmentName, initiatedBy, tradingPairSymbol, domain.TradingOperationTypeBuy, 0, 0, 0, false, invalidQuantityError, nil)
		return nil, invalidQuantityError
	}

	purchasePricePerUnit := currentPricePerUnit
	if cumulativeQuote, parseError := strconv.ParseFloat(buyOrderResponse.CumulativeQuote, 64); parseError == nil && cumulativeQuote > 0 {
		purchasePricePerUnit = cumulativeQuote / executedQuantity
	}

	buyOrderIdentifier := strconv.FormatInt(buyOrderResponse.OrderID, 10)
	service.logExecution(operationContext, userIdentifier, environmentName, initiatedBy, tradingPairSymbol, domain.TradingOperationTypeBuy, purchasePricePerUnit, executedQuantity, purchasePricePerUnit*executedQuantity, true, nil, &buyOrderIdentifier)

	targetSellPricePerUnit := purchasePricePerUnit * (1 + (targetProfitPercent / 100))
	if symbolFilters.TickSize > 0 {
		targetSellPricePerUnit = roundToIncrement(targetSellPricePerUnit, symbolFilters.TickSize)
	}

	var sellOrderIdentifier *string
	sellOrderResponse, sellError := tradingService.PlaceLimitSell(operationContext, tradingPairSymbol, executedQuantity, targetSellPricePerUnit, symbolFilters)
	if sellError != nil {
		service.logExecution(operationContext, userIdentifier, environmentName, initiatedBy, tradingPairSymbol, domain.TradingOperationTypeSell, targetSellPricePerUnit, executedQuantity, targetSellPricePerUnit*executedQuantity, false, sellError, nil)
	} else if sellOrderResponse != nil {
		identifier := strconv.FormatInt(sellOrderResponse.OrderID, 10)
		sellOrderIdentifier = &identifier
		service.logExecution(operationContext, userIdentifier, environmentName, initiatedBy, tradingPairSymbol, domain.TradingOperationTypeSell, targetSellPricePerUnit, executedQuantity, targetSellPricePerUnit*executedQuantity, true, nil, sellOrderIdentifier)
	}

	operation := domain.TradingOperation{
		TradingPairSymbol:      tradingPairSymbol,
		QuantityPurchased:      executedQuantity,
		PurchasePricePerUnit:   purchasePricePerUnit,
		TargetProfitPercent:    targetProfitPercent,
		Status:                 domain.TradingOperationStatusOpen,
		BinanceEnvironment:     environmentName,
		BuyOrderIdentifier:     &buyOrderIdentifier,
		SellOrderIdentifier:    sellOrderIdentifier,
		SellTargetPricePerUnit: &targetSellPricePerUnit,
	}
	operationIdentifier, recordError := service.operationRepository.CreatePurchaseOperationForUser(operationContext, userIdentifier, operation)
	if recordError != nil {
		return nil, recordError
	}
	operation.Identifier = operationIdentifier
	return &operation, nil
}

// ExecuteDailyPurchase performs the daily DCA buy (always bot-initiated) and records a DAILY_BUY
// marker execution (used for the daily-buy history and to keep the daily purchase idempotent).
func (service *UserTradingService) ExecuteDailyPurchase(operationContext context.Context, userIdentifier int64, environment string, tradingPairSymbol string, quoteAmount float64, targetProfitPercent float64) (*domain.TradingOperation, error) {
	operation, buyError := service.ExecuteBuy(operationContext, userIdentifier, domain.ExecutionInitiatorBot, tradingPairSymbol, quoteAmount, targetProfitPercent)
	if buyError != nil {
		service.logExecution(operationContext, userIdentifier, environment, domain.ExecutionInitiatorBot, tradingPairSymbol, domain.TradingOperationTypeDailyBuy, 0, 0, 0, false, buyError, nil)
		return nil, buyError
	}
	service.logExecution(operationContext, userIdentifier, operation.BinanceEnvironment, domain.ExecutionInitiatorBot, operation.TradingPairSymbol, domain.TradingOperationTypeDailyBuy, operation.PurchasePricePerUnit, operation.QuantityPurchased, operation.PurchasePricePerUnit*operation.QuantityPurchased, true, nil, operation.BuyOrderIdentifier)
	return operation, nil
}

// CloseOperationNow immediately closes an OPEN position at market on the user's request (user-initiated):
// it cancels the resting take-profit limit sell, places a market sell for the held quantity, and marks
// the operation sold. Real-money (PRODUCTION) sells require live trading to be enabled, like buys do.
func (service *UserTradingService) CloseOperationNow(operationContext context.Context, userIdentifier int64, operationIdentifier int64) (*domain.TradingOperation, error) {
	operation, lookupError := service.operationRepository.FindOperationByIdForUser(operationContext, userIdentifier, operationIdentifier)
	if lookupError != nil {
		return nil, lookupError
	}
	if operation.Status != domain.TradingOperationStatusOpen {
		return nil, errors.New("this operation is already closed")
	}

	environmentConfiguration, configurationError := service.credentialService.LoadActiveEnvironmentConfiguration(operationContext, userIdentifier)
	if configurationError != nil {
		return nil, configurationError
	}
	if environmentConfiguration == nil {
		return nil, errors.New("connect a Binance account first")
	}
	environmentName := environmentConfiguration.EnvironmentName
	settings, _ := service.settingsRepository.GetByUserAndEnvironment(operationContext, userIdentifier, environmentName)
	if environmentName == domain.BinanceEnvironmentProduction && (settings == nil || !settings.LiveTradingEnabled) {
		return nil, errors.New("enable live trading in your settings before selling real-money positions")
	}

	tradingService := NewBinanceTradingService(*environmentConfiguration)
	priceService := NewBinancePriceService(*environmentConfiguration)
	fallbackPrice, _ := priceService.GetCurrentPrice(operationContext, operation.TradingPairSymbol)

	// Free the balance held by the resting take-profit; if it already filled, reconcile to sold.
	if operation.SellOrderIdentifier != nil {
		if cancelError := tradingService.CancelOrder(operationContext, operation.TradingPairSymbol, *operation.SellOrderIdentifier); cancelError != nil {
			if orderStatus, statusError := tradingService.GetOrderStatus(operationContext, operation.TradingPairSymbol, *operation.SellOrderIdentifier); statusError == nil && orderStatus != nil && orderStatus.Status == "FILLED" {
				filledPrice := fillPriceFromStatus(*orderStatus, operation.PurchasePricePerUnit)
				return service.finalizeManualSell(operationContext, userIdentifier, environmentName, domain.ExecutionInitiatorUser, *operation, filledPrice, operation.SellOrderIdentifier)
			}
			return nil, fmt.Errorf("could not cancel the existing take-profit order: %w", cancelError)
		}
	}

	sellResponse, sellError := tradingService.PlaceMarketSellByQuantity(operationContext, operation.TradingPairSymbol, operation.QuantityPurchased)
	if sellError != nil {
		service.logExecution(operationContext, userIdentifier, environmentName, domain.ExecutionInitiatorUser, operation.TradingPairSymbol, domain.TradingOperationTypeSell, fallbackPrice, operation.QuantityPurchased, fallbackPrice*operation.QuantityPurchased, false, sellError, nil)
		return nil, sellError
	}
	sellOrderIdentifier := strconv.FormatInt(sellResponse.OrderID, 10)
	return service.finalizeManualSell(operationContext, userIdentifier, environmentName, domain.ExecutionInitiatorUser, *operation, fillPriceFromOrder(*sellResponse, fallbackPrice), &sellOrderIdentifier)
}

func (service *UserTradingService) finalizeManualSell(operationContext context.Context, userIdentifier int64, environment string, initiatedBy string, operation domain.TradingOperation, fillPrice float64, sellOrderIdentifier *string) (*domain.TradingOperation, error) {
	if updateError := service.operationRepository.UpdateOperationAsSoldForUser(operationContext, userIdentifier, operation.Identifier, fillPrice); updateError != nil {
		return nil, updateError
	}
	service.logExecution(operationContext, userIdentifier, environment, initiatedBy, operation.TradingPairSymbol, domain.TradingOperationTypeSell, fillPrice, operation.QuantityPurchased, fillPrice*operation.QuantityPurchased, true, nil, sellOrderIdentifier)

	soldAt := time.Now()
	operation.Status = domain.TradingOperationStatusSold
	operation.SellPricePerUnit = &fillPrice
	operation.SellTimestamp = &soldAt
	return &operation, nil
}

// PlaceTakeProfitForOperation (re)places the resting take-profit limit sell for an OPEN position
// whose sell order is missing (user-initiated). It is idempotent: a still-live sell order is left in
// place, and an already-filled one reconciles to sold.
func (service *UserTradingService) PlaceTakeProfitForOperation(operationContext context.Context, userIdentifier int64, operationIdentifier int64) (*domain.TradingOperation, error) {
	operation, lookupError := service.operationRepository.FindOperationByIdForUser(operationContext, userIdentifier, operationIdentifier)
	if lookupError != nil {
		return nil, lookupError
	}
	if operation.Status != domain.TradingOperationStatusOpen {
		return nil, errors.New("this operation is already closed")
	}

	environmentConfiguration, configurationError := service.credentialService.LoadActiveEnvironmentConfiguration(operationContext, userIdentifier)
	if configurationError != nil {
		return nil, configurationError
	}
	if environmentConfiguration == nil {
		return nil, errors.New("connect a Binance account first")
	}
	environmentName := environmentConfiguration.EnvironmentName
	if operation.BinanceEnvironment != "" && operation.BinanceEnvironment != environmentName {
		return nil, fmt.Errorf("switch to the %s environment to manage this position", operation.BinanceEnvironment)
	}
	settings, _ := service.settingsRepository.GetByUserAndEnvironment(operationContext, userIdentifier, environmentName)
	if environmentName == domain.BinanceEnvironmentProduction && (settings == nil || !settings.LiveTradingEnabled) {
		return nil, errors.New("enable live trading in your settings before placing real-money orders")
	}

	tradingService := NewBinanceTradingService(*environmentConfiguration)

	// Don't duplicate an existing sell order: leave a live one alone, reconcile a filled one to sold.
	if operation.SellOrderIdentifier != nil {
		if orderStatus, statusError := tradingService.GetOrderStatus(operationContext, operation.TradingPairSymbol, *operation.SellOrderIdentifier); statusError == nil && orderStatus != nil {
			switch orderStatus.Status {
			case "NEW", "PARTIALLY_FILLED":
				return operation, nil
			case "FILLED":
				filledPrice := fillPriceFromStatus(*orderStatus, operation.PurchasePricePerUnit)
				return service.finalizeManualSell(operationContext, userIdentifier, environmentName, domain.ExecutionInitiatorUser, *operation, filledPrice, operation.SellOrderIdentifier)
			}
		}
	}

	symbolFilters, _ := tradingService.FetchSymbolFilters(operationContext, operation.TradingPairSymbol)
	targetSellPricePerUnit := operation.PurchasePricePerUnit * (1 + (operation.TargetProfitPercent / 100))
	if symbolFilters.TickSize > 0 {
		targetSellPricePerUnit = roundToIncrement(targetSellPricePerUnit, symbolFilters.TickSize)
	}

	sellOrderResponse, sellError := tradingService.PlaceLimitSell(operationContext, operation.TradingPairSymbol, operation.QuantityPurchased, targetSellPricePerUnit, symbolFilters)
	if sellError != nil {
		service.logExecution(operationContext, userIdentifier, environmentName, domain.ExecutionInitiatorUser, operation.TradingPairSymbol, domain.TradingOperationTypeSell, targetSellPricePerUnit, operation.QuantityPurchased, targetSellPricePerUnit*operation.QuantityPurchased, false, sellError, nil)
		return nil, sellError
	}

	sellOrderIdentifier := strconv.FormatInt(sellOrderResponse.OrderID, 10)
	service.logExecution(operationContext, userIdentifier, environmentName, domain.ExecutionInitiatorUser, operation.TradingPairSymbol, domain.TradingOperationTypeSell, targetSellPricePerUnit, operation.QuantityPurchased, targetSellPricePerUnit*operation.QuantityPurchased, true, nil, &sellOrderIdentifier)
	if updateError := service.operationRepository.UpdateOperationSellOrderForUser(operationContext, userIdentifier, operation.Identifier, sellOrderIdentifier, targetSellPricePerUnit); updateError != nil {
		return nil, updateError
	}
	operation.SellOrderIdentifier = &sellOrderIdentifier
	operation.SellTargetPricePerUnit = &targetSellPricePerUnit
	return operation, nil
}

func (service *UserTradingService) ListOperations(loadContext context.Context, userIdentifier int64, limit int) ([]domain.TradingOperation, error) {
	environment := service.credentialService.ActiveEnvironmentName(loadContext, userIdentifier)
	return service.operationRepository.ListRecentOperationsForUser(loadContext, userIdentifier, environment, limit)
}

func (service *UserTradingService) ListExecutions(loadContext context.Context, userIdentifier int64, limit int) ([]domain.TradingOperationExecution, error) {
	environment := service.credentialService.ActiveEnvironmentName(loadContext, userIdentifier)
	return service.executionRepository.ListRecentExecutionsForUser(loadContext, userIdentifier, environment, limit)
}

func (service *UserTradingService) ListOpenOrders(loadContext context.Context, userIdentifier int64, tradingPairSymbol string) ([]BinanceOpenOrder, error) {
	environmentConfiguration, configurationError := service.credentialService.LoadActiveEnvironmentConfiguration(loadContext, userIdentifier)
	if configurationError != nil {
		return nil, configurationError
	}
	if environmentConfiguration == nil {
		return nil, errors.New("connect a Binance account first")
	}

	tradingPairSymbol = strings.ToUpper(strings.TrimSpace(tradingPairSymbol))
	if tradingPairSymbol == "" {
		if settings, _ := service.settingsRepository.GetByUserAndEnvironment(loadContext, userIdentifier, environmentConfiguration.EnvironmentName); settings != nil {
			tradingPairSymbol = settings.TradingPairSymbol
		}
	}
	if tradingPairSymbol == "" {
		tradingPairSymbol = "BTCUSDT"
	}

	tradingService := NewBinanceTradingService(*environmentConfiguration)
	return tradingService.ListOpenOrders(loadContext, tradingPairSymbol)
}

func (service *UserTradingService) logExecution(operationContext context.Context, userIdentifier int64, environment string, initiatedBy string, tradingPairSymbol string, operationType string, unitPrice float64, quantity float64, totalValue float64, success bool, cause error, orderIdentifier *string) {
	var errorMessage *string
	if cause != nil {
		message := cause.Error()
		errorMessage = &message
	}
	_, _ = service.executionRepository.LogExecutionForUser(operationContext, userIdentifier, domain.TradingOperationExecution{
		TradingPairSymbol:  tradingPairSymbol,
		OperationType:      operationType,
		BinanceEnvironment: environment,
		InitiatedBy:        initiatedBy,
		UnitPrice:          unitPrice,
		Quantity:           quantity,
		TotalValue:         totalValue,
		ExecutedAt:         time.Now(),
		Success:            success,
		ErrorMessage:       errorMessage,
		OrderIdentifier:    orderIdentifier,
	})
}
