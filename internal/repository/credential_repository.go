package repository

import (
        "context"
        "database/sql"

        "coin-alert/internal/domain"
)

type BinanceCredentialRepository interface {
        SaveCredentials(saveContext context.Context, credential domain.BinanceCredentialRecord) error
        LoadActiveCredentials(loadContext context.Context) (*domain.BinanceCredentialRecord, error)
        LoadLatestCredentialsByEnvironment(loadContext context.Context, environmentName string) (*domain.BinanceCredentialRecord, error)
        ActivateEnvironment(loadContext context.Context, environmentName string) error
}

type PostgresBinanceCredentialRepository struct {
        Database *sql.DB
}

func NewPostgresBinanceCredentialRepository(database *sql.DB) *PostgresBinanceCredentialRepository {
        return &PostgresBinanceCredentialRepository{Database: database}
}

func (repository *PostgresBinanceCredentialRepository) SaveCredentials(saveContext context.Context, credential domain.BinanceCredentialRecord) error {
        transaction, transactionError := repository.Database.BeginTx(saveContext, nil)
        if transactionError != nil {
                return transactionError
        }

        _, deactivateError := transaction.ExecContext(saveContext, "UPDATE binance_credentials SET is_active = false")
        if deactivateError != nil {
                transaction.Rollback()
                return deactivateError
        }

        _, executionError := transaction.ExecContext(
                saveContext,
                "INSERT INTO binance_credentials (api_key, api_secret, environment, api_base_url, is_active) VALUES ($1, $2, $3, $4, $5)",
                credential.APIKey,
                credential.APISecret,
                credential.EnvironmentName,
                credential.APIBaseURL,
                credential.IsActive,
        )
        if executionError != nil {
                transaction.Rollback()
                return executionError
        }

        return transaction.Commit()
}

func (repository *PostgresBinanceCredentialRepository) LoadActiveCredentials(loadContext context.Context) (*domain.BinanceCredentialRecord, error) {
        row := repository.Database.QueryRowContext(loadContext, "SELECT api_key, api_secret, environment, api_base_url, is_active FROM binance_credentials WHERE is_active = true ORDER BY created_at DESC LIMIT 1")

        var apiKey string
        var apiSecret string
        var environmentName string
        var apiBaseURL string
        var isActive bool

        scanError := row.Scan(&apiKey, &apiSecret, &environmentName, &apiBaseURL, &isActive)
        if scanError == sql.ErrNoRows {
                return nil, nil
        }
        if scanError != nil {
                return nil, scanError
        }

        return &domain.BinanceCredentialRecord{APIKey: apiKey, APISecret: apiSecret, EnvironmentName: environmentName, APIBaseURL: apiBaseURL, IsActive: isActive}, nil
}

func (repository *PostgresBinanceCredentialRepository) LoadLatestCredentialsByEnvironment(loadContext context.Context, environmentName string) (*domain.BinanceCredentialRecord, error) {
        row := repository.Database.QueryRowContext(loadContext, "SELECT api_key, api_secret, environment, api_base_url, is_active FROM binance_credentials WHERE environment = $1 ORDER BY created_at DESC LIMIT 1", environmentName)

        var apiKey string
        var apiSecret string
        var environment string
        var apiBaseURL string
        var isActive bool

        scanError := row.Scan(&apiKey, &apiSecret, &environment, &apiBaseURL, &isActive)
        if scanError == sql.ErrNoRows {
                return nil, nil
        }
        if scanError != nil {
                return nil, scanError
        }

        return &domain.BinanceCredentialRecord{APIKey: apiKey, APISecret: apiSecret, EnvironmentName: environment, APIBaseURL: apiBaseURL, IsActive: isActive}, nil
}

func (repository *PostgresBinanceCredentialRepository) ActivateEnvironment(loadContext context.Context, environmentName string) error {
        transaction, transactionError := repository.Database.BeginTx(loadContext, nil)
        if transactionError != nil {
                return transactionError
        }

        _, deactivateError := transaction.ExecContext(loadContext, "UPDATE binance_credentials SET is_active = false")
        if deactivateError != nil {
                transaction.Rollback()
                return deactivateError
        }

        row := transaction.QueryRowContext(loadContext, "SELECT id FROM binance_credentials WHERE environment = $1 ORDER BY created_at DESC LIMIT 1", environmentName)
        var credentialID int64
        scanError := row.Scan(&credentialID)
        if scanError != nil {
                transaction.Rollback()
                return scanError
        }

        _, activationError := transaction.ExecContext(loadContext, "UPDATE binance_credentials SET is_active = true WHERE id = $1", credentialID)
        if activationError != nil {
                transaction.Rollback()
                return activationError
        }

        return transaction.Commit()
}
