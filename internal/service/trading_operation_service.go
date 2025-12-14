package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"coin-alert/internal/domain"
	"coin-alert/internal/repository"
)

type TradingOperationService struct {
	TradingOperationRepository repository.TradingOperationRepository
	ConfiguredTradingPair      string
	CapitalThreshold           float64
	ProfitTargetPercent        float64
}

func NewTradingOperationService(tradingOperationRepository repository.TradingOperationRepository, configuredTradingPair string, capitalThreshold float64, profitTargetPercent float64) *TradingOperationService {
	return &TradingOperationService{
		TradingOperationRepository: tradingOperationRepository,
		ConfiguredTradingPair:      strings.ToUpper(configuredTradingPair),
		CapitalThreshold:           capitalThreshold,
		ProfitTargetPercent:        profitTargetPercent,
	}
}

func (service *TradingOperationService) UpdateCapitalThreshold(newCapitalThreshold float64) {
	service.CapitalThreshold = newCapitalThreshold
}

func (service *TradingOperationService) RecordPurchaseOperation(contextWithTimeout context.Context, operation domain.TradingOperation) (int64, error) {
	validationError := service.validatePurchaseOperation(operation)
	if validationError != nil {
		return 0, validationError
	}

	allocationCheckError := service.ensureCapitalThresholdNotExceeded(contextWithTimeout, operation)
	if allocationCheckError != nil {
		return 0, allocationCheckError
	}

	operation.Identifier = 0
	operation.Status = domain.TradingOperationStatusOpen
	operation.PurchaseTimestamp = time.Now()
	if operation.TargetProfitPercent <= 0 {
		operation.TargetProfitPercent = service.ProfitTargetPercent
	}

	return service.TradingOperationRepository.CreatePurchaseOperation(contextWithTimeout, operation)
}

func (service *TradingOperationService) ListOperations(contextWithTimeout context.Context, limit int) ([]domain.TradingOperation, error) {
	return service.TradingOperationRepository.ListRecentOperations(contextWithTimeout, limit)
}

func (service *TradingOperationService) CloseOperationsThatReachedTargetPrice(contextWithTimeout context.Context, currentPricePerUnit float64) error {
        openOperations, fetchError := service.TradingOperationRepository.ListOpenOperations(contextWithTimeout)
        if fetchError != nil {
                return fetchError
        }

	for _, openOperation := range openOperations {
		if openOperation.HasReachedTarget(currentPricePerUnit) {
			updateError := service.TradingOperationRepository.UpdateOperationAsSold(contextWithTimeout, openOperation.Identifier, currentPricePerUnit)
			if updateError != nil {
				return updateError
			}
		}
	}

        return nil
}

func (service *TradingOperationService) ListOpenOperations(contextWithTimeout context.Context) ([]domain.TradingOperation, error) {
        return service.TradingOperationRepository.ListOpenOperations(contextWithTimeout)
}

func (service *TradingOperationService) validatePurchaseOperation(operation domain.TradingOperation) error {
	if strings.TrimSpace(operation.TradingPairSymbol) == "" {
		return errors.New("a trading pair symbol is required")
	}

	normalizedPair := strings.ToUpper(operation.TradingPairSymbol)
	if service.ConfiguredTradingPair != "" && normalizedPair != service.ConfiguredTradingPair {
		return fmt.Errorf("all operations must use the configured trading pair %s", service.ConfiguredTradingPair)
	}

	if operation.QuantityPurchased <= 0 {
		return errors.New("quantity must be greater than zero")
	}

	if operation.PurchasePricePerUnit <= 0 {
		return errors.New("purchase price per unit must be greater than zero")
	}

	return nil
}

func (service *TradingOperationService) ensureCapitalThresholdNotExceeded(contextWithTimeout context.Context, operation domain.TradingOperation) error {
	purchaseValueTotal := operation.PurchaseValueTotal()
	openAllocationTotal, allocationError := service.TradingOperationRepository.CalculateOpenAllocationTotal(contextWithTimeout)
	if allocationError != nil {
		return allocationError
	}

	if openAllocationTotal+purchaseValueTotal > service.CapitalThreshold {
		return fmt.Errorf("purchase would exceed the capital threshold of %.2f", service.CapitalThreshold)
	}

	return nil
}
