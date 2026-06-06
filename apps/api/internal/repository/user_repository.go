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
	FindByEmail(lookupContext context.Context, email string) (*domain.User, error)
	FindByIdentifier(lookupContext context.Context, userIdentifier int64) (*domain.User, error)
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
		 RETURNING id, email, password_hash, COALESCE(display_name, ''), is_active, created_at, updated_at`,
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

func (repository *PostgresUserRepository) FindByEmail(lookupContext context.Context, email string) (*domain.User, error) {
	row := repository.Database.QueryRowContext(
		lookupContext,
		`SELECT id, email, password_hash, COALESCE(display_name, ''), is_active, created_at, updated_at
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
		`SELECT id, email, password_hash, COALESCE(display_name, ''), is_active, created_at, updated_at
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

func scanUser(row *sql.Row) (*domain.User, error) {
	user := &domain.User{}
	scanError := row.Scan(
		&user.Identifier,
		&user.Email,
		&user.PasswordHash,
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
