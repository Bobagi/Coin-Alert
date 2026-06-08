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
// Real-money (PRODUCTION) orders are refused unless the user explicitly enabled live trading.
func (service *UserTradingService) ExecuteBuy(operationContext context.Context, userIdentifier int64, tradingPairSymbol string, quoteAmount float64, targetProfitPercent float64) (*domain.TradingOperation, error) {
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

	currentPricePerUnit, priceError := priceService.GetCurrentPrice(operationContext, tradingPairSymbol)
	if priceError != nil {
		return nil, fmt.Errorf("could not fetch the current price: %w", priceError)
	}
	if currentPricePerUnit <= 0 {
		return nil, errors.New("the current price is unavailable for this pair")
	}

	buyOrderResponse, buyError := tradingService.PlaceMarketBuyByQuote(operationContext, tradingPairSymbol, quoteAmount)
	if buyError != nil {
		service.logExecution(operationContext, userIdentifier, environmentName, tradingPairSymbol, domain.TradingOperationTypeBuy, 0, 0, 0, false, buyError, nil)
		return nil, buyError
	}

	executedQuantity, _ := strconv.ParseFloat(buyOrderResponse.ExecutedQty, 64)
	if executedQuantity <= 0 {
		invalidQuantityError := errors.New("Binance returned an invalid executed quantity")
		service.logExecution(operationContext, userIdentifier, environmentName, tradingPairSymbol, domain.TradingOperationTypeBuy, 0, 0, 0, false, invalidQuantityError, nil)
		return nil, invalidQuantityError
	}

	purchasePricePerUnit := currentPricePerUnit
	if cumulativeQuote, parseError := strconv.ParseFloat(buyOrderResponse.CumulativeQuote, 64); parseError == nil && cumulativeQuote > 0 {
		purchasePricePerUnit = cumulativeQuote / executedQuantity
	}

	buyOrderIdentifier := strconv.FormatInt(buyOrderResponse.OrderID, 10)
	service.logExecution(operationContext, userIdentifier, environmentName, tradingPairSymbol, domain.TradingOperationTypeBuy, purchasePricePerUnit, executedQuantity, purchasePricePerUnit*executedQuantity, true, nil, &buyOrderIdentifier)

	targetSellPricePerUnit := purchasePricePerUnit * (1 + (targetProfitPercent / 100))

	var sellOrderIdentifier *string
	sellOrderResponse, sellError := tradingService.PlaceLimitSell(operationContext, tradingPairSymbol, executedQuantity, targetSellPricePerUnit)
	if sellError != nil {
		service.logExecution(operationContext, userIdentifier, environmentName, tradingPairSymbol, domain.TradingOperationTypeSell, targetSellPricePerUnit, executedQuantity, targetSellPricePerUnit*executedQuantity, false, sellError, nil)
	} else if sellOrderResponse != nil {
		identifier := strconv.FormatInt(sellOrderResponse.OrderID, 10)
		sellOrderIdentifier = &identifier
		service.logExecution(operationContext, userIdentifier, environmentName, tradingPairSymbol, domain.TradingOperationTypeSell, targetSellPricePerUnit, executedQuantity, targetSellPricePerUnit*executedQuantity, true, nil, sellOrderIdentifier)
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

// ExecuteDailyPurchase performs the daily DCA buy and records a DAILY_BUY marker execution
// (used for the daily-buy history and to keep the daily purchase idempotent within a day).
func (service *UserTradingService) ExecuteDailyPurchase(operationContext context.Context, userIdentifier int64, environment string, tradingPairSymbol string, quoteAmount float64, targetProfitPercent float64) (*domain.TradingOperation, error) {
	operation, buyError := service.ExecuteBuy(operationContext, userIdentifier, tradingPairSymbol, quoteAmount, targetProfitPercent)
	if buyError != nil {
		service.logExecution(operationContext, userIdentifier, environment, tradingPairSymbol, domain.TradingOperationTypeDailyBuy, 0, 0, 0, false, buyError, nil)
		return nil, buyError
	}
	service.logExecution(operationContext, userIdentifier, operation.BinanceEnvironment, operation.TradingPairSymbol, domain.TradingOperationTypeDailyBuy, operation.PurchasePricePerUnit, operation.QuantityPurchased, operation.PurchasePricePerUnit*operation.QuantityPurchased, true, nil, operation.BuyOrderIdentifier)
	return operation, nil
}

// CloseOperationNow immediately closes an OPEN position at market on the user's request: it cancels
// the resting take-profit limit sell, places a market sell for the held quantity, and marks the
// operation sold. Real-money (PRODUCTION) sells require live trading to be enabled, like buys do.
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
				return service.finalizeManualSell(operationContext, userIdentifier, environmentName, *operation, filledPrice, operation.SellOrderIdentifier)
			}
			return nil, fmt.Errorf("could not cancel the existing take-profit order: %w", cancelError)
		}
	}

	sellResponse, sellError := tradingService.PlaceMarketSellByQuantity(operationContext, operation.TradingPairSymbol, operation.QuantityPurchased)
	if sellError != nil {
		service.logExecution(operationContext, userIdentifier, environmentName, operation.TradingPairSymbol, domain.TradingOperationTypeSell, fallbackPrice, operation.QuantityPurchased, fallbackPrice*operation.QuantityPurchased, false, sellError, nil)
		return nil, sellError
	}
	sellOrderIdentifier := strconv.FormatInt(sellResponse.OrderID, 10)
	return service.finalizeManualSell(operationContext, userIdentifier, environmentName, *operation, fillPriceFromOrder(*sellResponse, fallbackPrice), &sellOrderIdentifier)
}

func (service *UserTradingService) finalizeManualSell(operationContext context.Context, userIdentifier int64, environment string, operation domain.TradingOperation, fillPrice float64, sellOrderIdentifier *string) (*domain.TradingOperation, error) {
	if updateError := service.operationRepository.UpdateOperationAsSoldForUser(operationContext, userIdentifier, operation.Identifier, fillPrice); updateError != nil {
		return nil, updateError
	}
	service.logExecution(operationContext, userIdentifier, environment, operation.TradingPairSymbol, domain.TradingOperationTypeSell, fillPrice, operation.QuantityPurchased, fillPrice*operation.QuantityPurchased, true, nil, sellOrderIdentifier)

	soldAt := time.Now()
	operation.Status = domain.TradingOperationStatusSold
	operation.SellPricePerUnit = &fillPrice
	operation.SellTimestamp = &soldAt
	return &operation, nil
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

func (service *UserTradingService) logExecution(operationContext context.Context, userIdentifier int64, environment string, tradingPairSymbol string, operationType string, unitPrice float64, quantity float64, totalValue float64, success bool, cause error, orderIdentifier *string) {
	var errorMessage *string
	if cause != nil {
		message := cause.Error()
		errorMessage = &message
	}
	_, _ = service.executionRepository.LogExecutionForUser(operationContext, userIdentifier, domain.TradingOperationExecution{
		TradingPairSymbol:  tradingPairSymbol,
		OperationType:      operationType,
		BinanceEnvironment: environment,
		UnitPrice:          unitPrice,
		Quantity:           quantity,
		TotalValue:         totalValue,
		ExecutedAt:         time.Now(),
		Success:            success,
		ErrorMessage:       errorMessage,
		OrderIdentifier:    orderIdentifier,
	})
}
