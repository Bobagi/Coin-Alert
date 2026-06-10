package repository

import (
	"context"
	"database/sql"
	"time"
)

// AccountDeletionAuditRepository records a minimal, non-identifying audit row when an account is
// deleted. It stores no PII — only a keyed email fingerprint (computed by the caller) and metadata.
type AccountDeletionAuditRepository interface {
	RecordDeletion(operationContext context.Context, userIdentifier int64, emailFingerprint string, authMethod string, accountCreatedAt time.Time) error
}

type PostgresAccountDeletionAuditRepository struct {
	Database *sql.DB
}

func NewPostgresAccountDeletionAuditRepository(database *sql.DB) *PostgresAccountDeletionAuditRepository {
	return &PostgresAccountDeletionAuditRepository{Database: database}
}

// RecordDeletion writes the audit row. It must run BEFORE the user (and the cascade) is deleted, so
// the had-credentials / operation-count subqueries can still see the user's rows. An empty
// emailFingerprint is stored as NULL.
func (repository *PostgresAccountDeletionAuditRepository) RecordDeletion(operationContext context.Context, userIdentifier int64, emailFingerprint string, authMethod string, accountCreatedAt time.Time) error {
	var fingerprintArgument interface{}
	if emailFingerprint != "" {
		fingerprintArgument = emailFingerprint
	}

	_, executionError := repository.Database.ExecContext(
		operationContext,
		`INSERT INTO account_deletion_audit
		    (email_fingerprint, auth_method, account_created_at, had_binance_credentials, operation_count)
		 VALUES (
		    $1,
		    $2,
		    $3,
		    EXISTS(SELECT 1 FROM binance_credentials WHERE user_id = $4),
		    (SELECT COUNT(*) FROM trading_operations WHERE user_id = $4)
		 )`,
		fingerprintArgument,
		authMethod,
		accountCreatedAt,
		userIdentifier,
	)
	return executionError
}
