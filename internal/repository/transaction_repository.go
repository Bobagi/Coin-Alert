package repository

import (
    "context"
    "database/sql"
    "time"

    "coin-alert/internal/domain"
)

type TransactionRepository interface {
    CreateTransaction(context.Context, domain.Transaction) (int64, error)
    ListRecentTransactions(context.Context, int) ([]domain.Transaction, error)
}

type PostgresTransactionRepository struct {
    Database *sql.DB
}

func NewPostgresTransactionRepository(database *sql.DB) *PostgresTransactionRepository {
    return &PostgresTransactionRepository{Database: database}
}

func (repository *PostgresTransactionRepository) CreateTransaction(contextWithTimeout context.Context, transaction domain.Transaction) (int64, error) {
    insertSQL := `INSERT INTO transactions(operation_type, asset_symbol, quantity, price_per_unit, notes) VALUES($1, $2, $3, $4, $5) RETURNING id, created_at`
    statementContext, statementCancel := context.WithTimeout(contextWithTimeout, 5*time.Second)
    defer statementCancel()

    row := repository.Database.QueryRowContext(statementContext, insertSQL, transaction.OperationType, transaction.AssetSymbol, transaction.Quantity, transaction.PricePerUnit, transaction.Notes)
    var identifier int64
    var createdAt time.Time
    scanError := row.Scan(&identifier, &createdAt)
    if scanError != nil {
        return 0, scanError
    }

    return identifier, nil
}

func (repository *PostgresTransactionRepository) ListRecentTransactions(contextWithTimeout context.Context, limit int) ([]domain.Transaction, error) {
    querySQL := `SELECT id, operation_type, asset_symbol, quantity, price_per_unit, notes, created_at FROM transactions ORDER BY created_at DESC LIMIT $1`
    queryContext, queryCancel := context.WithTimeout(contextWithTimeout, 5*time.Second)
    defer queryCancel()

    rows, queryError := repository.Database.QueryContext(queryContext, querySQL, limit)
    if queryError != nil {
        return nil, queryError
    }
    defer rows.Close()

    var transactions []domain.Transaction
    for rows.Next() {
        var transaction domain.Transaction
        scanError := rows.Scan(&transaction.Identifier, &transaction.OperationType, &transaction.AssetSymbol, &transaction.Quantity, &transaction.PricePerUnit, &transaction.Notes, &transaction.CreatedAt)
        if scanError != nil {
            return nil, scanError
        }
        transactions = append(transactions, transaction)
    }

    return transactions, nil
}
