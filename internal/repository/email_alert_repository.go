package repository

import (
    "context"
    "database/sql"
    "time"

    "coin-alert/internal/domain"
)

type EmailAlertRepository interface {
    LogEmailAlert(context.Context, domain.EmailAlert) (int64, error)
}

type PostgresEmailAlertRepository struct {
    Database *sql.DB
}

func NewPostgresEmailAlertRepository(database *sql.DB) *PostgresEmailAlertRepository {
    return &PostgresEmailAlertRepository{Database: database}
}

func (repository *PostgresEmailAlertRepository) LogEmailAlert(contextWithTimeout context.Context, alert domain.EmailAlert) (int64, error) {
    insertSQL := `INSERT INTO email_alerts(recipient_address, trading_pair_or_currency, threshold_value) VALUES($1, $2, $3) RETURNING id, created_at`
    statementContext, statementCancel := context.WithTimeout(contextWithTimeout, 5*time.Second)
    defer statementCancel()

    row := repository.Database.QueryRowContext(statementContext, insertSQL, alert.RecipientAddress, alert.TradingPairOrCurrency, alert.ThresholdValue)
    var identifier int64
    var createdAt time.Time
    scanError := row.Scan(&identifier, &createdAt)
    if scanError != nil {
        return 0, scanError
    }

    return identifier, nil
}
