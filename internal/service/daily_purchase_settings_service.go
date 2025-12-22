package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"coin-alert/internal/domain"
	"coin-alert/internal/repository"
)

type DailyPurchaseSettingsService struct {
	SettingsRepository      repository.DailyPurchaseSettingsRepository
	DefaultExecutionHourUTC int
}

func NewDailyPurchaseSettingsService(settingsRepository repository.DailyPurchaseSettingsRepository, defaultExecutionHourUTC int) *DailyPurchaseSettingsService {
	return &DailyPurchaseSettingsService{
		SettingsRepository:      settingsRepository,
		DefaultExecutionHourUTC: defaultExecutionHourUTC,
	}
}

func (service *DailyPurchaseSettingsService) SaveSettings(contextWithTimeout context.Context, tradingPairSymbol string, purchaseAmount float64) (*domain.DailyPurchaseSettings, error) {
	cleanTradingPair := strings.TrimSpace(tradingPairSymbol)
	if cleanTradingPair == "" {
		return nil, errors.New("Daily purchase trading pair is required")
	}
	if purchaseAmount <= 0 {
		return nil, errors.New("Daily purchase amount must be greater than zero")
	}

	settings := domain.DailyPurchaseSettings{
		TradingPairSymbol: cleanTradingPair,
		PurchaseAmount:    purchaseAmount,
		ExecutionHourUTC:  service.DefaultExecutionHourUTC,
		IsActive:          true,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	identifier, saveError := service.SettingsRepository.UpsertActiveSettings(contextWithTimeout, settings)
	if saveError != nil {
		return nil, saveError
	}
	settings.Identifier = identifier
	return &settings, nil
}

func (service *DailyPurchaseSettingsService) GetActiveSettings(contextWithTimeout context.Context) (*domain.DailyPurchaseSettings, error) {
	return service.SettingsRepository.LoadActiveSettings(contextWithTimeout)
}

func (service *DailyPurchaseSettingsService) ResolveExecutionHourUTC(contextWithTimeout context.Context) int {
	settings, settingsError := service.GetActiveSettings(contextWithTimeout)
	if settingsError != nil || settings == nil {
		return service.DefaultExecutionHourUTC
	}
	if settings.ExecutionHourUTC >= 0 && settings.ExecutionHourUTC <= 23 {
		return settings.ExecutionHourUTC
	}
	return service.DefaultExecutionHourUTC
}
