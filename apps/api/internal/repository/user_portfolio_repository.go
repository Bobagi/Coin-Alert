package repository

import (
	"context"
	"database/sql"
	"errors"
)

// UserPortfolioRepository stores each user's Investidor10 public wallet URL.
type UserPortfolioRepository interface {
	GetWalletURL(loadContext context.Context, userIdentifier int64) (string, error)
	UpsertWalletURL(operationContext context.Context, userIdentifier int64, walletURL string) error
}

type PostgresUserPortfolioRepository struct {
	Database *sql.DB
}

func NewPostgresUserPortfolioRepository(database *sql.DB) *PostgresUserPortfolioRepository {
	return &PostgresUserPortfolioRepository{Database: database}
}

func (repository *PostgresUserPortfolioRepository) GetWalletURL(loadContext context.Context, userIdentifier int64) (string, error) {
	row := repository.Database.QueryRowContext(
		loadContext,
		"SELECT COALESCE(investidor10_wallet_url, '') FROM user_portfolio_sources WHERE user_id = $1",
		userIdentifier,
	)
	var walletURL string
	scanError := row.Scan(&walletURL)
	if errors.Is(scanError, sql.ErrNoRows) {
		return "", nil
	}
	if scanError != nil {
		return "", scanError
	}
	return walletURL, nil
}

func (repository *PostgresUserPortfolioRepository) UpsertWalletURL(operationContext context.Context, userIdentifier int64, walletURL string) error {
	_, executionError := repository.Database.ExecContext(
		operationContext,
		`INSERT INTO user_portfolio_sources (user_id, investidor10_wallet_url)
		 VALUES ($1, NULLIF($2, ''))
		 ON CONFLICT (user_id) DO UPDATE SET investidor10_wallet_url = EXCLUDED.investidor10_wallet_url, updated_at = NOW()`,
		userIdentifier, walletURL,
	)
	return executionError
}
