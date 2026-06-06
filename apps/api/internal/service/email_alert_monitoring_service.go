package service

import (
	"context"
	"log"
	"time"

	"coin-alert/internal/domain"
	"coin-alert/internal/repository"
)

type EmailAlertMonitoringService struct {
	EmailAlertRepository repository.EmailAlertRepository
	EmailAlertService    *EmailAlertService
	BinancePriceService  *BinancePriceService
	PollInterval         time.Duration
}

func NewEmailAlertMonitoringService(emailAlertRepository repository.EmailAlertRepository, emailAlertService *EmailAlertService, binancePriceService *BinancePriceService, pollIntervalSeconds int) *EmailAlertMonitoringService {
	pollInterval := time.Duration(pollIntervalSeconds) * time.Second
	if pollInterval <= 0 {
		pollInterval = time.Minute
	}
	return &EmailAlertMonitoringService{
		EmailAlertRepository: emailAlertRepository,
		EmailAlertService:    emailAlertService,
		BinancePriceService:  binancePriceService,
		PollInterval:         pollInterval,
	}
}

func (service *EmailAlertMonitoringService) StartBackgroundJobs(applicationContext context.Context) {
	go service.startMonitoringLoop(applicationContext)
}

func (service *EmailAlertMonitoringService) startMonitoringLoop(applicationContext context.Context) {
	ticker := time.NewTicker(service.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-applicationContext.Done():
			log.Println("Email alert monitoring stopped")
			return
		case <-ticker.C:
			service.evaluateActiveAlerts(applicationContext)
		}
	}
}

func (service *EmailAlertMonitoringService) evaluateActiveAlerts(applicationContext context.Context) {
	alertsContext, alertsCancel := context.WithTimeout(applicationContext, 8*time.Second)
	defer alertsCancel()

	activeAlerts, alertsError := service.EmailAlertRepository.ListActiveAlerts(alertsContext, 200)
	if alertsError != nil {
		log.Printf("Email alert lookup failed: %v", alertsError)
		return
	}
	if len(activeAlerts) == 0 {
		return
	}

	currentPrices := service.fetchCurrentPricesForAlerts(applicationContext, activeAlerts)
	for _, alert := range activeAlerts {
		currentPrice, priceAvailable := currentPrices[alert.TradingPairOrCurrency]
		if !priceAvailable {
			continue
		}

		triggerBoundary := resolveTriggerBoundary(alert, currentPrice)
		if triggerBoundary == "" {
			continue
		}

		triggerContext, triggerCancel := context.WithTimeout(applicationContext, 8*time.Second)
		triggerError := service.EmailAlertService.TriggerAlert(triggerContext, alert, currentPrice, triggerBoundary)
		triggerCancel()
		if triggerError != nil {
			log.Printf("Email alert trigger failed for %s: %v", alert.TradingPairOrCurrency, triggerError)
		}
	}
}

func (service *EmailAlertMonitoringService) fetchCurrentPricesForAlerts(applicationContext context.Context, alerts []domain.EmailAlert) map[string]float64 {
	uniqueSymbols := make(map[string]struct{})
	for _, alert := range alerts {
		uniqueSymbols[alert.TradingPairOrCurrency] = struct{}{}
	}

	currentPrices := make(map[string]float64)
	for symbol := range uniqueSymbols {
		priceContext, priceCancel := context.WithTimeout(applicationContext, 6*time.Second)
		currentPrice, priceError := service.BinancePriceService.GetCurrentPrice(priceContext, symbol)
		priceCancel()
		if priceError != nil {
			log.Printf("Email alert price lookup failed for %s: %v", symbol, priceError)
			continue
		}
		currentPrices[symbol] = currentPrice
	}
	return currentPrices
}

func resolveTriggerBoundary(alert domain.EmailAlert, currentPrice float64) string {
	if currentPrice <= alert.MinimumThreshold {
		return "minimum"
	}
	if currentPrice >= alert.MaximumThreshold {
		return "maximum"
	}
	return ""
}
