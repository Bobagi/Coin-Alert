package repository

import (
	"context"
	"database/sql"
)

type BinanceCredentialRepository interface {
	SaveCredentials(saveContext context.Context, apiKey string, apiSecret string) error
	LoadLatestCredentials(loadContext context.Context) (string, string, error)
}

type PostgresBinanceCredentialRepository struct {
	Database *sql.DB
}

func NewPostgresBinanceCredentialRepository(database *sql.DB) *PostgresBinanceCredentialRepository {
	return &PostgresBinanceCredentialRepository{Database: database}
}

func (repository *PostgresBinanceCredentialRepository) SaveCredentials(saveContext context.Context, apiKey string, apiSecret string) error {
	_, executionError := repository.Database.ExecContext(saveContext, "INSERT INTO binance_credentials (api_key, api_secret) VALUES ($1, $2)", apiKey, apiSecret)
	return executionError
}

func (repository *PostgresBinanceCredentialRepository) LoadLatestCredentials(loadContext context.Context) (string, string, error) {
	row := repository.Database.QueryRowContext(loadContext, "SELECT api_key, api_secret FROM binance_credentials ORDER BY created_at DESC LIMIT 1")

	var apiKey string
	var apiSecret string
	scanError := row.Scan(&apiKey, &apiSecret)
	if scanError == sql.ErrNoRows {
		return "", "", nil
	}
	if scanError != nil {
		return "", "", scanError
	}

	return apiKey, apiSecret, nil
}
