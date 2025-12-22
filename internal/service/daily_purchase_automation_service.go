package service

import (
	"context"
	"log"
	"strconv"
	"time"

	"coin-alert/internal/domain"
)

type DailyPurchaseAutomationService struct {
	DailyPurchaseSettingsService *DailyPurchaseSettingsService
	BinancePriceService          *BinancePriceService
	BinanceTradingService        *BinanceTradingService
	TradingOperationService      *TradingOperationService
	TradingScheduleService       *TradingScheduleService
}

func NewDailyPurchaseAutomationService(dailyPurchaseSettingsService *DailyPurchaseSettingsService, binancePriceService *BinancePriceService, binanceTradingService *BinanceTradingService, tradingOperationService *TradingOperationService, tradingScheduleService *TradingScheduleService) *DailyPurchaseAutomationService {
	return &DailyPurchaseAutomationService{
		DailyPurchaseSettingsService: dailyPurchaseSettingsService,
		BinancePriceService:          binancePriceService,
		BinanceTradingService:        binanceTradingService,
		TradingOperationService:      tradingOperationService,
		TradingScheduleService:       tradingScheduleService,
	}
}

func (service *DailyPurchaseAutomationService) StartBackgroundJobs(applicationContext context.Context) {
	go service.startDailyPurchaseLoop(applicationContext)
}

func (service *DailyPurchaseAutomationService) startDailyPurchaseLoop(applicationContext context.Context) {
	for {
		nextExecutionTime := service.calculateNextDailyPurchaseTime(applicationContext)
		waitDuration := time.Until(nextExecutionTime)
		if waitDuration < 0 {
			waitDuration = time.Minute
		}
		timer := time.NewTimer(waitDuration)
		select {
		case <-applicationContext.Done():
			timer.Stop()
			log.Println("Daily purchase loop stopped")
			return
		case <-timer.C:
			service.executeDailyPurchase(applicationContext)
		}
	}
}

func (service *DailyPurchaseAutomationService) calculateNextDailyPurchaseTime(applicationContext context.Context) time.Time {
	nowUTC := time.Now().UTC()
	settingsContext, settingsCancel := context.WithTimeout(applicationContext, 5*time.Second)
	defer settingsCancel()
	executionHourUTC := service.DailyPurchaseSettingsService.ResolveExecutionHourUTC(settingsContext)
	nextExecutionTime := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day(), executionHourUTC, 0, 0, 0, time.UTC)
	if !nextExecutionTime.After(nowUTC) {
		nextExecutionTime = nextExecutionTime.Add(24 * time.Hour)
	}
	return nextExecutionTime
}

func (service *DailyPurchaseAutomationService) executeDailyPurchase(applicationContext context.Context) {
	settingsContext, settingsCancel := context.WithTimeout(applicationContext, 5*time.Second)
	defer settingsCancel()
	settings, settingsError := service.DailyPurchaseSettingsService.GetActiveSettings(settingsContext)
	if settingsError != nil {
		log.Printf("Daily purchase settings lookup failed: %v", settingsError)
		service.logDailyPurchaseFailure(applicationContext, "Daily purchase settings could not be loaded.")
		return
	}
	if settings == nil {
		log.Println("Daily purchase skipped because no settings are configured")
		return
	}

	priceLookupContext, priceLookupCancel := context.WithTimeout(applicationContext, 10*time.Second)
	defer priceLookupCancel()
	currentPricePerUnit, priceLookupError := service.BinancePriceService.GetCurrentPrice(priceLookupContext, settings.TradingPairSymbol)
	if priceLookupError != nil {
		log.Printf("Daily purchase price lookup failed for %s: %v", settings.TradingPairSymbol, priceLookupError)
		service.logDailyPurchaseFailure(applicationContext, "Daily purchase failed: could not fetch current price.")
		return
	}
	if currentPricePerUnit <= 0 {
		log.Printf("Daily purchase price is not positive for %s: %f", settings.TradingPairSymbol, currentPricePerUnit)
		service.logDailyPurchaseFailure(applicationContext, "Daily purchase failed: current price is unavailable.")
		return
	}

	buyExecutionContext, buyExecutionCancel := context.WithTimeout(applicationContext, 15*time.Second)
	defer buyExecutionCancel()
	buyOrderResponse, buyError := service.BinanceTradingService.PlaceMarketBuyByQuote(buyExecutionContext, settings.TradingPairSymbol, settings.PurchaseAmount)
	if buyError != nil {
		log.Printf("Daily purchase buy failed for %s: %v", settings.TradingPairSymbol, buyError)
		service.logDailyPurchaseFailure(applicationContext, "Daily purchase failed: "+buyError.Error())
		return
	}

	executedQuantity, quantityParseError := strconv.ParseFloat(buyOrderResponse.ExecutedQty, 64)
	if quantityParseError != nil || executedQuantity <= 0 {
		log.Printf("Daily purchase returned invalid executed quantity for %s: %v", settings.TradingPairSymbol, quantityParseError)
		service.logDailyPurchaseFailure(applicationContext, "Daily purchase failed: Binance returned an invalid executed quantity.")
		return
	}

	purchaseUnitPrice := currentPricePerUnit
	cumulativeQuoteValue, cumulativeQuoteError := strconv.ParseFloat(buyOrderResponse.CumulativeQuote, 64)
	if cumulativeQuoteError == nil && executedQuantity > 0 {
		calculatedPrice := cumulativeQuoteValue / executedQuantity
		if calculatedPrice > 0 {
			purchaseUnitPrice = calculatedPrice
		}
	}

	targetProfitPercent := service.TradingScheduleService.TargetProfitPercent
	targetSellPricePerUnit := purchaseUnitPrice * (1 + (targetProfitPercent / 100))

	sellExecutionContext, sellExecutionCancel := context.WithTimeout(applicationContext, 15*time.Second)
	defer sellExecutionCancel()
	sellOrderResponse, sellError := service.BinanceTradingService.PlaceLimitSell(sellExecutionContext, settings.TradingPairSymbol, executedQuantity, targetSellPricePerUnit)

	buyOrderIdentifier := strconv.FormatInt(buyOrderResponse.OrderID, 10)
	var sellOrderIdentifier *string
	if sellOrderResponse != nil {
		sellIdentifier := strconv.FormatInt(sellOrderResponse.OrderID, 10)
		sellOrderIdentifier = &sellIdentifier
	}

	recordContext, recordCancel := context.WithTimeout(applicationContext, 10*time.Second)
	defer recordCancel()
	operation := domain.TradingOperation{
		TradingPairSymbol:    settings.TradingPairSymbol,
		QuantityPurchased:    executedQuantity,
		PurchasePricePerUnit: purchaseUnitPrice,
		TargetProfitPercent:  targetProfitPercent,
		BuyOrderIdentifier:   &buyOrderIdentifier,
		SellOrderIdentifier:  sellOrderIdentifier,
	}
	_, recordError := service.TradingOperationService.RecordPurchaseOperation(recordContext, operation)
	if recordError != nil {
		log.Printf("Daily purchase could not record operation for %s: %v", settings.TradingPairSymbol, recordError)
		service.logDailyPurchaseFailure(applicationContext, "Daily purchase failed: could not record the operation.")
		return
	}

	if sellError != nil {
		log.Printf("Daily purchase sell order failed for %s: %v", settings.TradingPairSymbol, sellError)
		service.logDailyPurchaseFailure(applicationContext, "Daily purchase completed but sell order failed: "+sellError.Error())
		return
	}

	service.logDailyPurchaseSuccess(applicationContext, purchaseUnitPrice, executedQuantity, purchaseUnitPrice*executedQuantity, &buyOrderIdentifier)
}

func (service *DailyPurchaseAutomationService) logDailyPurchaseFailure(applicationContext context.Context, message string) {
	service.logDailyPurchaseExecution(applicationContext, false, message, nil, 0, 0, 0)
}

func (service *DailyPurchaseAutomationService) logDailyPurchaseSuccess(applicationContext context.Context, unitPrice float64, quantity float64, totalValue float64, orderIdentifier *string) {
	service.logDailyPurchaseExecution(applicationContext, true, "", orderIdentifier, unitPrice, quantity, totalValue)
}

func (service *DailyPurchaseAutomationService) logDailyPurchaseExecution(applicationContext context.Context, success bool, errorMessageText string, orderIdentifier *string, unitPrice float64, quantity float64, totalValue float64) {
	executionContext, executionCancel := context.WithTimeout(applicationContext, 5*time.Second)
	defer executionCancel()

	var errorMessage *string
	if !success {
		if errorMessageText == "" {
			errorMessageText = "Daily purchase failed for an unknown reason."
		}
		errorMessage = &errorMessageText
	}

	settingsContext, settingsCancel := context.WithTimeout(applicationContext, 5*time.Second)
	defer settingsCancel()
	settings, settingsError := service.DailyPurchaseSettingsService.GetActiveSettings(settingsContext)
	if settingsError != nil || settings == nil {
		log.Printf("Daily purchase execution logging skipped because settings are unavailable: %v", settingsError)
		return
	}

	executionRecord := domain.TradingOperationExecution{
		TradingPairSymbol: settings.TradingPairSymbol,
		OperationType:     domain.TradingOperationTypeDailyBuy,
		UnitPrice:         unitPrice,
		Quantity:          quantity,
		TotalValue:        totalValue,
		ExecutedAt:        time.Now(),
		Success:           success,
		ErrorMessage:      errorMessage,
		OrderIdentifier:   orderIdentifier,
	}

	_, logError := service.TradingScheduleService.LogExecution(executionContext, executionRecord)
	if logError != nil {
		log.Printf("Could not log daily purchase execution: %v", logError)
	}
}
