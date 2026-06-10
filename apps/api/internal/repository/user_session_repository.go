package repository

import (
	"context"
	"database/sql"
	"errors"

	"coin-alert/internal/domain"
)

type UserSessionRepository interface {
	CreateSession(creationContext context.Context, session domain.UserSession) error
	FindActiveByTokenHash(lookupContext context.Context, sessionTokenHash string) (*domain.UserSession, error)
	DeleteByTokenHash(deletionContext context.Context, sessionTokenHash string) error
	DeleteAllForUser(deletionContext context.Context, userIdentifier int64) error
	DeleteExpiredSessions(deletionContext context.Context) (int64, error)
}

type PostgresUserSessionRepository struct {
	Database *sql.DB
}

func NewPostgresUserSessionRepository(database *sql.DB) *PostgresUserSessionRepository {
	return &PostgresUserSessionRepository{Database: database}
}

func (repository *PostgresUserSessionRepository) CreateSession(creationContext context.Context, session domain.UserSession) error {
	_, executionError := repository.Database.ExecContext(
		creationContext,
		`INSERT INTO user_sessions (user_id, session_token_hash, expires_at, user_agent, ip_address)
		 VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''))`,
		session.UserIdentifier,
		session.SessionTokenHash,
		session.ExpiresAt,
		session.UserAgent,
		session.IPAddress,
	)
	return executionError
}

// FindActiveByTokenHash returns the unexpired session for a token hash, or (nil, nil) if none.
func (repository *PostgresUserSessionRepository) FindActiveByTokenHash(lookupContext context.Context, sessionTokenHash string) (*domain.UserSession, error) {
	row := repository.Database.QueryRowContext(
		lookupContext,
		`SELECT id, user_id, session_token_hash, expires_at, created_at, last_seen_at,
		        COALESCE(user_agent, ''), COALESCE(ip_address, '')
		 FROM user_sessions
		 WHERE session_token_hash = $1 AND expires_at > NOW()`,
		sessionTokenHash,
	)

	session := &domain.UserSession{}
	scanError := row.Scan(
		&session.Identifier,
		&session.UserIdentifier,
		&session.SessionTokenHash,
		&session.ExpiresAt,
		&session.CreatedAt,
		&session.LastSeenAt,
		&session.UserAgent,
		&session.IPAddress,
	)
	if errors.Is(scanError, sql.ErrNoRows) {
		return nil, nil
	}
	if scanError != nil {
		return nil, scanError
	}
	return session, nil
}

func (repository *PostgresUserSessionRepository) DeleteByTokenHash(deletionContext context.Context, sessionTokenHash string) error {
	_, executionError := repository.Database.ExecContext(
		deletionContext,
		`DELETE FROM user_sessions WHERE session_token_hash = $1`,
		sessionTokenHash,
	)
	return executionError
}

// DeleteAllForUser revokes every session of a user (used after a password reset).
func (repository *PostgresUserSessionRepository) DeleteAllForUser(deletionContext context.Context, userIdentifier int64) error {
	_, executionError := repository.Database.ExecContext(
		deletionContext,
		`DELETE FROM user_sessions WHERE user_id = $1`,
		userIdentifier,
	)
	return executionError
}

func (repository *PostgresUserSessionRepository) DeleteExpiredSessions(deletionContext context.Context) (int64, error) {
	result, executionError := repository.Database.ExecContext(
		deletionContext,
		`DELETE FROM user_sessions WHERE expires_at <= NOW()`,
	)
	if executionError != nil {
		return 0, executionError
	}
	return result.RowsAffected()
}
