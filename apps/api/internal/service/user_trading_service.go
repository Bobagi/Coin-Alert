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

	settings, _ := service.settingsRepository.GetByUserIdentifier(operationContext, userIdentifier)
	if targetProfitPercent <= 0 && settings != nil {
		targetProfitPercent = settings.TargetProfitPercent
	}
	if targetProfitPercent <= 0 {
		targetProfitPercent = 1.0
	}

	environmentConfiguration, configurationError := service.credentialService.LoadActiveEnvironmentConfiguration(operationContext, userIdentifier)
	if configurationError != nil {
		return nil, configurationError
	}
	if environmentConfiguration == nil {
		return nil, errors.New("connect a Binance account before trading")
	}
	if environmentConfiguration.EnvironmentName == domain.BinanceEnvironmentProduction && (settings == nil || !settings.LiveTradingEnabled) {
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
		service.logExecution(operationContext, userIdentifier, tradingPairSymbol, domain.TradingOperationTypeBuy, 0, 0, 0, false, buyError, nil)
		return nil, buyError
	}

	executedQuantity, _ := strconv.ParseFloat(buyOrderResponse.ExecutedQty, 64)
	if executedQuantity <= 0 {
		invalidQuantityError := errors.New("Binance returned an invalid executed quantity")
		service.logExecution(operationContext, userIdentifier, tradingPairSymbol, domain.TradingOperationTypeBuy, 0, 0, 0, false, invalidQuantityError, nil)
		return nil, invalidQuantityError
	}

	purchasePricePerUnit := currentPricePerUnit
	if cumulativeQuote, parseError := strconv.ParseFloat(buyOrderResponse.CumulativeQuote, 64); parseError == nil && cumulativeQuote > 0 {
		purchasePricePerUnit = cumulativeQuote / executedQuantity
	}

	buyOrderIdentifier := strconv.FormatInt(buyOrderResponse.OrderID, 10)
	service.logExecution(operationContext, userIdentifier, tradingPairSymbol, domain.TradingOperationTypeBuy, purchasePricePerUnit, executedQuantity, purchasePricePerUnit*executedQuantity, true, nil, &buyOrderIdentifier)

	targetSellPricePerUnit := purchasePricePerUnit * (1 + (targetProfitPercent / 100))

	var sellOrderIdentifier *string
	sellOrderResponse, sellError := tradingService.PlaceLimitSell(operationContext, tradingPairSymbol, executedQuantity, targetSellPricePerUnit)
	if sellError != nil {
		service.logExecution(operationContext, userIdentifier, tradingPairSymbol, domain.TradingOperationTypeSell, targetSellPricePerUnit, executedQuantity, targetSellPricePerUnit*executedQuantity, false, sellError, nil)
	} else if sellOrderResponse != nil {
		identifier := strconv.FormatInt(sellOrderResponse.OrderID, 10)
		sellOrderIdentifier = &identifier
		service.logExecution(operationContext, userIdentifier, tradingPairSymbol, domain.TradingOperationTypeSell, targetSellPricePerUnit, executedQuantity, targetSellPricePerUnit*executedQuantity, true, nil, sellOrderIdentifier)
	}

	operation := domain.TradingOperation{
		TradingPairSymbol:      tradingPairSymbol,
		QuantityPurchased:      executedQuantity,
		PurchasePricePerUnit:   purchasePricePerUnit,
		TargetProfitPercent:    targetProfitPercent,
		Status:                 domain.TradingOperationStatusOpen,
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
func (service *UserTradingService) ExecuteDailyPurchase(operationContext context.Context, userIdentifier int64, tradingPairSymbol string, quoteAmount float64, targetProfitPercent float64) (*domain.TradingOperation, error) {
	operation, buyError := service.ExecuteBuy(operationContext, userIdentifier, tradingPairSymbol, quoteAmount, targetProfitPercent)
	if buyError != nil {
		service.logExecution(operationContext, userIdentifier, tradingPairSymbol, domain.TradingOperationTypeDailyBuy, 0, 0, 0, false, buyError, nil)
		return nil, buyError
	}
	service.logExecution(operationContext, userIdentifier, operation.TradingPairSymbol, domain.TradingOperationTypeDailyBuy, operation.PurchasePricePerUnit, operation.QuantityPurchased, operation.PurchasePricePerUnit*operation.QuantityPurchased, true, nil, operation.BuyOrderIdentifier)
	return operation, nil
}

func (service *UserTradingService) ListOperations(loadContext context.Context, userIdentifier int64, limit int) ([]domain.TradingOperation, error) {
	return service.operationRepository.ListRecentOperationsForUser(loadContext, userIdentifier, limit)
}

func (service *UserTradingService) ListExecutions(loadContext context.Context, userIdentifier int64, limit int) ([]domain.TradingOperationExecution, error) {
	return service.executionRepository.ListRecentExecutionsForUser(loadContext, userIdentifier, limit)
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
		if settings, _ := service.settingsRepository.GetByUserIdentifier(loadContext, userIdentifier); settings != nil {
			tradingPairSymbol = settings.TradingPairSymbol
		}
	}
	if tradingPairSymbol == "" {
		tradingPairSymbol = "BTCUSDT"
	}

	tradingService := NewBinanceTradingService(*environmentConfiguration)
	return tradingService.ListOpenOrders(loadContext, tradingPairSymbol)
}

func (service *UserTradingService) logExecution(operationContext context.Context, userIdentifier int64, tradingPairSymbol string, operationType string, unitPrice float64, quantity float64, totalValue float64, success bool, cause error, orderIdentifier *string) {
	var errorMessage *string
	if cause != nil {
		message := cause.Error()
		errorMessage = &message
	}
	_, _ = service.executionRepository.LogExecutionForUser(operationContext, userIdentifier, domain.TradingOperationExecution{
		TradingPairSymbol: tradingPairSymbol,
		OperationType:     operationType,
		UnitPrice:         unitPrice,
		Quantity:          quantity,
		TotalValue:        totalValue,
		ExecutedAt:        time.Now(),
		Success:           success,
		ErrorMessage:      errorMessage,
		OrderIdentifier:   orderIdentifier,
	})
}
