package repository

import (
	"context"
	"database/sql"
	"errors"

	"coin-alert/internal/domain"
)

// UserBinanceCredentialRepository persists Binance credentials scoped to a single user.
// Stored api_key/api_secret are expected to be encrypted by the caller.
type UserBinanceCredentialRepository interface {
	SaveCredentialForUser(saveContext context.Context, userIdentifier int64, credential domain.BinanceCredentialRecord) error
	LoadActiveCredentialForUser(loadContext context.Context, userIdentifier int64) (*domain.BinanceCredentialRecord, error)
	LoadLatestCredentialForUserByEnvironment(loadContext context.Context, userIdentifier int64, environmentName string) (*domain.BinanceCredentialRecord, error)
	ActivateEnvironmentForUser(operationContext context.Context, userIdentifier int64, environmentName string) error
	ListConfiguredEnvironmentsForUser(loadContext context.Context, userIdentifier int64) ([]string, error)
}

func (repository *PostgresBinanceCredentialRepository) SaveCredentialForUser(saveContext context.Context, userIdentifier int64, credential domain.BinanceCredentialRecord) error {
	transaction, transactionError := repository.Database.BeginTx(saveContext, nil)
	if transactionError != nil {
		return transactionError
	}

	if _, deactivateError := transaction.ExecContext(saveContext, "UPDATE binance_credentials SET is_active = false WHERE user_id = $1", userIdentifier); deactivateError != nil {
		transaction.Rollback()
		return deactivateError
	}

	_, insertError := transaction.ExecContext(
		saveContext,
		`INSERT INTO binance_credentials (user_id, api_key, api_secret, environment, api_base_url, is_active)
		 VALUES ($1, $2, $3, $4, $5, true)`,
		userIdentifier,
		credential.APIKey,
		credential.APISecret,
		credential.EnvironmentName,
		credential.APIBaseURL,
	)
	if insertError != nil {
		transaction.Rollback()
		return insertError
	}

	return transaction.Commit()
}

func (repository *PostgresBinanceCredentialRepository) LoadActiveCredentialForUser(loadContext context.Context, userIdentifier int64) (*domain.BinanceCredentialRecord, error) {
	row := repository.Database.QueryRowContext(
		loadContext,
		`SELECT api_key, api_secret, environment, api_base_url, is_active
		 FROM binance_credentials
		 WHERE user_id = $1 AND is_active = true
		 ORDER BY created_at DESC LIMIT 1`,
		userIdentifier,
	)
	return scanCredentialRecord(row)
}

func (repository *PostgresBinanceCredentialRepository) LoadLatestCredentialForUserByEnvironment(loadContext context.Context, userIdentifier int64, environmentName string) (*domain.BinanceCredentialRecord, error) {
	row := repository.Database.QueryRowContext(
		loadContext,
		`SELECT api_key, api_secret, environment, api_base_url, is_active
		 FROM binance_credentials
		 WHERE user_id = $1 AND environment = $2
		 ORDER BY created_at DESC LIMIT 1`,
		userIdentifier,
		environmentName,
	)
	return scanCredentialRecord(row)
}

func (repository *PostgresBinanceCredentialRepository) ActivateEnvironmentForUser(operationContext context.Context, userIdentifier int64, environmentName string) error {
	transaction, transactionError := repository.Database.BeginTx(operationContext, nil)
	if transactionError != nil {
		return transactionError
	}

	if _, deactivateError := transaction.ExecContext(operationContext, "UPDATE binance_credentials SET is_active = false WHERE user_id = $1", userIdentifier); deactivateError != nil {
		transaction.Rollback()
		return deactivateError
	}

	row := transaction.QueryRowContext(
		operationContext,
		"SELECT id FROM binance_credentials WHERE user_id = $1 AND environment = $2 ORDER BY created_at DESC LIMIT 1",
		userIdentifier,
		environmentName,
	)
	var credentialIdentifier int64
	if scanError := row.Scan(&credentialIdentifier); scanError != nil {
		transaction.Rollback()
		return scanError
	}

	if _, activateError := transaction.ExecContext(operationContext, "UPDATE binance_credentials SET is_active = true WHERE id = $1", credentialIdentifier); activateError != nil {
		transaction.Rollback()
		return activateError
	}

	return transaction.Commit()
}

func (repository *PostgresBinanceCredentialRepository) ListConfiguredEnvironmentsForUser(loadContext context.Context, userIdentifier int64) ([]string, error) {
	rows, queryError := repository.Database.QueryContext(
		loadContext,
		"SELECT DISTINCT environment FROM binance_credentials WHERE user_id = $1 ORDER BY environment",
		userIdentifier,
	)
	if queryError != nil {
		return nil, queryError
	}
	defer rows.Close()

	configuredEnvironments := make([]string, 0)
	for rows.Next() {
		var environmentName string
		if scanError := rows.Scan(&environmentName); scanError != nil {
			return nil, scanError
		}
		configuredEnvironments = append(configuredEnvironments, environmentName)
	}
	return configuredEnvironments, rows.Err()
}

func scanCredentialRecord(row *sql.Row) (*domain.BinanceCredentialRecord, error) {
	record := &domain.BinanceCredentialRecord{}
	scanError := row.Scan(&record.APIKey, &record.APISecret, &record.EnvironmentName, &record.APIBaseURL, &record.IsActive)
	if errors.Is(scanError, sql.ErrNoRows) {
		return nil, nil
	}
	if scanError != nil {
		return nil, scanError
	}
	return record, nil
}
