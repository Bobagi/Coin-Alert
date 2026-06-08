package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/lib/pq"

	"coin-alert/internal/domain"
)

// ErrUserNotFound is returned when no user matches the lookup.
var ErrUserNotFound = errors.New("user not found")

// ErrEmailAlreadyRegistered is returned when an email is already taken.
var ErrEmailAlreadyRegistered = errors.New("email is already registered")

type UserRepository interface {
	CreateUser(creationContext context.Context, email string, passwordHash string, displayName string) (*domain.User, error)
	CreateGoogleUser(creationContext context.Context, email string, googleSubject string, displayName string) (*domain.User, error)
	FindByEmail(lookupContext context.Context, email string) (*domain.User, error)
	FindByIdentifier(lookupContext context.Context, userIdentifier int64) (*domain.User, error)
	FindByGoogleSubject(lookupContext context.Context, googleSubject string) (*domain.User, error)
	LinkGoogleSubject(updateContext context.Context, userIdentifier int64, googleSubject string) error
	UpdateDisplayName(updateContext context.Context, userIdentifier int64, displayName string) (*domain.User, error)
	UpdatePasswordHash(updateContext context.Context, userIdentifier int64, passwordHash string) error
	DeleteUser(deletionContext context.Context, userIdentifier int64) error
}

type PostgresUserRepository struct {
	Database *sql.DB
}

func NewPostgresUserRepository(database *sql.DB) *PostgresUserRepository {
	return &PostgresUserRepository{Database: database}
}

func (repository *PostgresUserRepository) CreateUser(creationContext context.Context, email string, passwordHash string, displayName string) (*domain.User, error) {
	row := repository.Database.QueryRowContext(
		creationContext,
		`INSERT INTO users (email, password_hash, display_name)
		 VALUES ($1, $2, NULLIF($3, ''))
		 RETURNING id, email, COALESCE(password_hash, ''), COALESCE(google_subject, ''), COALESCE(display_name, ''), is_active, created_at, updated_at`,
		strings.TrimSpace(email),
		passwordHash,
		strings.TrimSpace(displayName),
	)

	createdUser, scanError := scanUser(row)
	if scanError != nil {
		if isUniqueViolation(scanError) {
			return nil, ErrEmailAlreadyRegistered
		}
		return nil, scanError
	}
	return createdUser, nil
}

// CreateGoogleUser provisions an account from a verified Google identity. It has no password until
// the user explicitly sets one.
func (repository *PostgresUserRepository) CreateGoogleUser(creationContext context.Context, email string, googleSubject string, displayName string) (*domain.User, error) {
	row := repository.Database.QueryRowContext(
		creationContext,
		`INSERT INTO users (email, password_hash, google_subject, display_name)
		 VALUES ($1, NULL, $2, NULLIF($3, ''))
		 RETURNING id, email, COALESCE(password_hash, ''), COALESCE(google_subject, ''), COALESCE(display_name, ''), is_active, created_at, updated_at`,
		strings.TrimSpace(email),
		strings.TrimSpace(googleSubject),
		strings.TrimSpace(displayName),
	)

	createdUser, scanError := scanUser(row)
	if scanError != nil {
		if isUniqueViolation(scanError) {
			return nil, ErrEmailAlreadyRegistered
		}
		return nil, scanError
	}
	return createdUser, nil
}

func (repository *PostgresUserRepository) FindByEmail(lookupContext context.Context, email string) (*domain.User, error) {
	row := repository.Database.QueryRowContext(
		lookupContext,
		`SELECT id, email, COALESCE(password_hash, ''), COALESCE(google_subject, ''), COALESCE(display_name, ''), is_active, created_at, updated_at
		 FROM users WHERE LOWER(email) = LOWER($1)`,
		strings.TrimSpace(email),
	)

	foundUser, scanError := scanUser(row)
	if errors.Is(scanError, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if scanError != nil {
		return nil, scanError
	}
	return foundUser, nil
}

func (repository *PostgresUserRepository) FindByIdentifier(lookupContext context.Context, userIdentifier int64) (*domain.User, error) {
	row := repository.Database.QueryRowContext(
		lookupContext,
		`SELECT id, email, COALESCE(password_hash, ''), COALESCE(google_subject, ''), COALESCE(display_name, ''), is_active, created_at, updated_at
		 FROM users WHERE id = $1`,
		userIdentifier,
	)

	foundUser, scanError := scanUser(row)
	if errors.Is(scanError, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if scanError != nil {
		return nil, scanError
	}
	return foundUser, nil
}

func (repository *PostgresUserRepository) FindByGoogleSubject(lookupContext context.Context, googleSubject string) (*domain.User, error) {
	row := repository.Database.QueryRowContext(
		lookupContext,
		`SELECT id, email, COALESCE(password_hash, ''), COALESCE(google_subject, ''), COALESCE(display_name, ''), is_active, created_at, updated_at
		 FROM users WHERE google_subject = $1`,
		strings.TrimSpace(googleSubject),
	)

	foundUser, scanError := scanUser(row)
	if errors.Is(scanError, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if scanError != nil {
		return nil, scanError
	}
	return foundUser, nil
}

func (repository *PostgresUserRepository) LinkGoogleSubject(updateContext context.Context, userIdentifier int64, googleSubject string) error {
	_, executionError := repository.Database.ExecContext(
		updateContext,
		`UPDATE users SET google_subject = $2, updated_at = NOW() WHERE id = $1`,
		userIdentifier,
		strings.TrimSpace(googleSubject),
	)
	if isUniqueViolation(executionError) {
		return ErrEmailAlreadyRegistered
	}
	return executionError
}

func (repository *PostgresUserRepository) UpdateDisplayName(updateContext context.Context, userIdentifier int64, displayName string) (*domain.User, error) {
	row := repository.Database.QueryRowContext(
		updateContext,
		`UPDATE users SET display_name = NULLIF($2, ''), updated_at = NOW() WHERE id = $1
		 RETURNING id, email, COALESCE(password_hash, ''), COALESCE(google_subject, ''), COALESCE(display_name, ''), is_active, created_at, updated_at`,
		userIdentifier,
		strings.TrimSpace(displayName),
	)

	updatedUser, scanError := scanUser(row)
	if errors.Is(scanError, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if scanError != nil {
		return nil, scanError
	}
	return updatedUser, nil
}

func (repository *PostgresUserRepository) UpdatePasswordHash(updateContext context.Context, userIdentifier int64, passwordHash string) error {
	_, executionError := repository.Database.ExecContext(
		updateContext,
		`UPDATE users SET password_hash = $2, updated_at = NOW() WHERE id = $1`,
		userIdentifier,
		passwordHash,
	)
	return executionError
}

func (repository *PostgresUserRepository) DeleteUser(deletionContext context.Context, userIdentifier int64) error {
	_, executionError := repository.Database.ExecContext(
		deletionContext,
		`DELETE FROM users WHERE id = $1`,
		userIdentifier,
	)
	return executionError
}

func scanUser(row *sql.Row) (*domain.User, error) {
	user := &domain.User{}
	scanError := row.Scan(
		&user.Identifier,
		&user.Email,
		&user.PasswordHash,
		&user.GoogleSubject,
		&user.DisplayName,
		&user.IsActive,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if scanError != nil {
		return nil, scanError
	}
	return user, nil
}

func isUniqueViolation(candidateError error) bool {
	var postgresError *pq.Error
	if errors.As(candidateError, &postgresError) {
		return postgresError.Code == "23505"
	}
	return false
}
