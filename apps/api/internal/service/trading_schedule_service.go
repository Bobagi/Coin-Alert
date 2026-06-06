package service

import (
	"context"
	"time"

	"coin-alert/internal/domain"
	"coin-alert/internal/repository"
)

type TradingScheduleService struct {
	ScheduledOperationRepository repository.ScheduledTradingOperationRepository
	ExecutionRepository          repository.TradingOperationExecutionRepository
	AutomaticSellInterval        time.Duration
	TradingPairSymbol            string
	CapitalThreshold             float64
	TargetProfitPercent          float64
}

func NewTradingScheduleService(scheduledOperationRepository repository.ScheduledTradingOperationRepository, executionRepository repository.TradingOperationExecutionRepository, automaticSellIntervalMinutes int, tradingPairSymbol string, capitalThreshold float64, targetProfitPercent float64) *TradingScheduleService {
	return &TradingScheduleService{
		ScheduledOperationRepository: scheduledOperationRepository,
		ExecutionRepository:          executionRepository,
		AutomaticSellInterval:        time.Duration(automaticSellIntervalMinutes) * time.Minute,
		TradingPairSymbol:            tradingPairSymbol,
		CapitalThreshold:             capitalThreshold,
		TargetProfitPercent:          targetProfitPercent,
	}
}

func (service *TradingScheduleService) UpdateCapitalThreshold(newCapitalThreshold float64) {
	service.CapitalThreshold = newCapitalThreshold
}

func (service *TradingScheduleService) UpdateTargetProfitPercent(newTargetProfitPercent float64) {
	service.TargetProfitPercent = newTargetProfitPercent
}

func (service *TradingScheduleService) EnqueueNextSellOperation(contextWithTimeout context.Context) (int64, error) {
	scheduledOperation := domain.ScheduledTradingOperation{
		TradingPairSymbol:      service.TradingPairSymbol,
		CapitalThreshold:       service.CapitalThreshold,
		TargetProfitPercent:    service.TargetProfitPercent,
		OperationType:          domain.TradingOperationTypeSell,
		ScheduledExecutionTime: time.Now().Add(service.AutomaticSellInterval),
		Status:                 domain.ScheduledOperationStatusScheduled,
	}
	return service.ScheduledOperationRepository.CreateScheduledOperation(contextWithTimeout, scheduledOperation)
}

func (service *TradingScheduleService) GetNextScheduledOperation(contextWithTimeout context.Context) (*domain.ScheduledTradingOperation, error) {
	return service.ScheduledOperationRepository.GetNextScheduledOperation(contextWithTimeout)
}

func (service *TradingScheduleService) ListScheduledOperations(contextWithTimeout context.Context, limit int) ([]domain.ScheduledTradingOperation, error) {
	return service.ScheduledOperationRepository.ListScheduledOperations(contextWithTimeout, limit)
}

func (service *TradingScheduleService) StartExecutionForNextOperation(contextWithTimeout context.Context) (*domain.ScheduledTradingOperation, error) {
	nextOperation, fetchError := service.ScheduledOperationRepository.GetNextScheduledOperation(contextWithTimeout)
	if fetchError != nil || nextOperation == nil {
		return nextOperation, fetchError
	}

	statusUpdateError := service.ScheduledOperationRepository.UpdateStatus(contextWithTimeout, nextOperation.Identifier, domain.ScheduledOperationStatusExecuting)
	if statusUpdateError != nil {
		return nil, statusUpdateError
	}
	nextOperation.Status = domain.ScheduledOperationStatusExecuting
	return nextOperation, nil
}

func (service *TradingScheduleService) CompleteScheduledOperation(contextWithTimeout context.Context, operationIdentifier int64) error {
	return service.ScheduledOperationRepository.UpdateStatus(contextWithTimeout, operationIdentifier, domain.ScheduledOperationStatusCancelled)
}

func (service *TradingScheduleService) LogExecution(contextWithTimeout context.Context, execution domain.TradingOperationExecution) (int64, error) {
	return service.ExecutionRepository.LogExecution(contextWithTimeout, execution)
}

func (service *TradingScheduleService) ListRecentExecutions(contextWithTimeout context.Context, limit int) ([]domain.TradingOperationExecution, error) {
	return service.ExecutionRepository.ListRecentExecutions(contextWithTimeout, limit)
}

func (service *TradingScheduleService) ListExecutionsPage(contextWithTimeout context.Context, limit int, pageNumber int) ([]domain.TradingOperationExecution, error) {
	if pageNumber < 1 {
		pageNumber = 1
	}

	offset := (pageNumber - 1) * limit
	return service.ExecutionRepository.ListExecutionsPage(contextWithTimeout, limit, offset)
}

func (service *TradingScheduleService) ListRecentExecutionsByOperationType(contextWithTimeout context.Context, limit int, operationType string) ([]domain.TradingOperationExecution, error) {
	return service.ExecutionRepository.ListRecentExecutionsByOperationType(contextWithTimeout, limit, operationType)
}

func (service *TradingScheduleService) ListExecutionsPageByOperationType(contextWithTimeout context.Context, limit int, pageNumber int, operationType string) ([]domain.TradingOperationExecution, error) {
	if pageNumber < 1 {
		pageNumber = 1
	}

	offset := (pageNumber - 1) * limit
	return service.ExecutionRepository.ListExecutionsPageByOperationType(contextWithTimeout, limit, offset, operationType)
}
