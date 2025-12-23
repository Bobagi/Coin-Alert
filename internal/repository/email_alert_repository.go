package repository

import (
    "context"
    "database/sql"
    "time"

    "coin-alert/internal/domain"
)

type EmailAlertRepository interface {
    CreateAlertDefinition(context.Context, domain.EmailAlert) (int64, error)
    ListActiveAlerts(context.Context, int) ([]domain.EmailAlert, error)
    MarkAlertTriggered(context.Context, int64) error
}

type PostgresEmailAlertRepository struct {
    Database *sql.DB
}

func NewPostgresEmailAlertRepository(database *sql.DB) *PostgresEmailAlertRepository {
    return &PostgresEmailAlertRepository{Database: database}
}

func (repository *PostgresEmailAlertRepository) CreateAlertDefinition(contextWithTimeout context.Context, alert domain.EmailAlert) (int64, error) {
    insertSQL := `INSERT INTO email_alerts(recipient_address, trading_pair_or_currency, threshold_value, min_threshold, max_threshold, is_active) VALUES($1, $2, $3, $4, $5, true) RETURNING id, created_at`
    statementContext, statementCancel := context.WithTimeout(contextWithTimeout, 5*time.Second)
    defer statementCancel()

    row := repository.Database.QueryRowContext(statementContext, insertSQL, alert.RecipientAddress, alert.TradingPairOrCurrency, alert.MaximumThreshold, alert.MinimumThreshold, alert.MaximumThreshold)
    var identifier int64
    var createdAt time.Time
    scanError := row.Scan(&identifier, &createdAt)
    if scanError != nil {
        return 0, scanError
    }

    return identifier, nil
}

func (repository *PostgresEmailAlertRepository) ListActiveAlerts(contextWithTimeout context.Context, limit int) ([]domain.EmailAlert, error) {
    querySQL := `SELECT id, recipient_address, trading_pair_or_currency, min_threshold, max_threshold, is_active, created_at, triggered_at FROM email_alerts WHERE is_active = true ORDER BY created_at DESC LIMIT $1`
    queryContext, queryCancel := context.WithTimeout(contextWithTimeout, 5*time.Second)
    defer queryCancel()

    rows, queryError := repository.Database.QueryContext(queryContext, querySQL, limit)
    if queryError != nil {
        return nil, queryError
    }
    defer rows.Close()

    var alerts []domain.EmailAlert
    for rows.Next() {
        var alert domain.EmailAlert
        var triggeredAt sql.NullTime
        scanError := rows.Scan(&alert.Identifier, &alert.RecipientAddress, &alert.TradingPairOrCurrency, &alert.MinimumThreshold, &alert.MaximumThreshold, &alert.IsActive, &alert.CreatedAt, &triggeredAt)
        if scanError != nil {
            return nil, scanError
        }
        if triggeredAt.Valid {
            value := triggeredAt.Time
            alert.TriggeredAt = &value
        }
        alerts = append(alerts, alert)
    }

    return alerts, nil
}

func (repository *PostgresEmailAlertRepository) MarkAlertTriggered(contextWithTimeout context.Context, alertIdentifier int64) error {
    updateSQL := `UPDATE email_alerts SET is_active = false, triggered_at = NOW() WHERE id = $1`
    statementContext, statementCancel := context.WithTimeout(contextWithTimeout, 5*time.Second)
    defer statementCancel()

    _, updateError := repository.Database.ExecContext(statementContext, updateSQL, alertIdentifier)
    return updateError
}
