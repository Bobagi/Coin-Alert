package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Auth token purposes.
const (
	AuthTokenPurposePasswordReset     = "password_reset"
	AuthTokenPurposeEmailVerification = "email_verification"
)

// ErrAuthTokenInvalid is returned when a token does not exist, is expired, or was already used.
var ErrAuthTokenInvalid = errors.New("token is invalid or has expired")

// AuthToken is a one-time token; only its hash is stored.
type AuthToken struct {
	Identifier     int64
	UserIdentifier int64
	Purpose        string
}

// AuthTokenRepository persists one-time tokens for password reset / email verification.
type AuthTokenRepository interface {
	CreateToken(operationContext context.Context, userIdentifier int64, purpose string, tokenHash string, expiresAt time.Time) error
	FindValidByHash(lookupContext context.Context, tokenHash string, purpose string) (*AuthToken, error)
	MarkUsed(operationContext context.Context, tokenIdentifier int64) error
	InvalidateUserTokens(operationContext context.Context, userIdentifier int64, purpose string) error
}

type PostgresAuthTokenRepository struct {
	Database *sql.DB
}

func NewPostgresAuthTokenRepository(database *sql.DB) *PostgresAuthTokenRepository {
	return &PostgresAuthTokenRepository{Database: database}
}

func (repository *PostgresAuthTokenRepository) CreateToken(operationContext context.Context, userIdentifier int64, purpose string, tokenHash string, expiresAt time.Time) error {
	_, executionError := repository.Database.ExecContext(
		operationContext,
		`INSERT INTO auth_tokens (user_id, purpose, token_hash, expires_at) VALUES ($1, $2, $3, $4)`,
		userIdentifier, purpose, tokenHash, expiresAt,
	)
	return executionError
}

// FindValidByHash returns the token for a hash if it is unused, unexpired, and of the given purpose.
func (repository *PostgresAuthTokenRepository) FindValidByHash(lookupContext context.Context, tokenHash string, purpose string) (*AuthToken, error) {
	row := repository.Database.QueryRowContext(
		lookupContext,
		`SELECT id, user_id, purpose FROM auth_tokens
		 WHERE token_hash = $1 AND purpose = $2 AND used_at IS NULL AND expires_at > NOW()`,
		tokenHash, purpose,
	)
	token := &AuthToken{}
	scanError := row.Scan(&token.Identifier, &token.UserIdentifier, &token.Purpose)
	if errors.Is(scanError, sql.ErrNoRows) {
		return nil, ErrAuthTokenInvalid
	}
	if scanError != nil {
		return nil, scanError
	}
	return token, nil
}

func (repository *PostgresAuthTokenRepository) MarkUsed(operationContext context.Context, tokenIdentifier int64) error {
	_, executionError := repository.Database.ExecContext(
		operationContext,
		`UPDATE auth_tokens SET used_at = NOW() WHERE id = $1`,
		tokenIdentifier,
	)
	return executionError
}

// InvalidateUserTokens marks every still-valid token of a purpose for a user as used, so issuing a
// new one (e.g. a fresh password-reset link) silently retires any previous links.
func (repository *PostgresAuthTokenRepository) InvalidateUserTokens(operationContext context.Context, userIdentifier int64, purpose string) error {
	_, executionError := repository.Database.ExecContext(
		operationContext,
		`UPDATE auth_tokens SET used_at = NOW() WHERE user_id = $1 AND purpose = $2 AND used_at IS NULL`,
		userIdentifier, purpose,
	)
	return executionError
}
