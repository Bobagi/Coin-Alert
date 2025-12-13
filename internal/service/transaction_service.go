package service

import (
    "context"
    "errors"
    "strings"
    "time"

    "coin-alert/internal/domain"
    "coin-alert/internal/repository"
)

type TransactionService struct {
    TransactionRepository repository.TransactionRepository
}

func NewTransactionService(transactionRepository repository.TransactionRepository) *TransactionService {
    return &TransactionService{TransactionRepository: transactionRepository}
}

func (service *TransactionService) RecordTransaction(contextWithTimeout context.Context, transaction domain.Transaction) (int64, error) {
    validationError := validateTransaction(transaction)
    if validationError != nil {
        return 0, validationError
    }

    transaction.Identifier = 0
    transaction.CreatedAt = time.Now()
    return service.TransactionRepository.CreateTransaction(contextWithTimeout, transaction)
}

func (service *TransactionService) ListTransactions(contextWithTimeout context.Context, limit int) ([]domain.Transaction, error) {
    return service.TransactionRepository.ListRecentTransactions(contextWithTimeout, limit)
}

func validateTransaction(transaction domain.Transaction) error {
    if transaction.AssetSymbol == "" {
        return errors.New("asset symbol must be provided")
    }

    upperOperationType := strings.ToUpper(transaction.OperationType)
    if upperOperationType != "BUY" && upperOperationType != "SELL" {
        return errors.New("operation type must be BUY or SELL")
    }

    if transaction.Quantity <= 0 {
        return errors.New("quantity must be positive")
    }

    if transaction.PricePerUnit <= 0 {
        return errors.New("price per unit must be positive")
    }

    return nil
}
