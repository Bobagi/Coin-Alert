package database

import (
        "context"
        "database/sql"
        "log"
        "strings"
        "time"

        _ "github.com/lib/pq"
)

type PostgresConnector struct {
	Database *sql.DB
}

func InitializePostgresConnector(databaseURL string) (*PostgresConnector, error) {
        databaseConnection, connectionError := sql.Open("postgres", databaseURL)
        if connectionError != nil {
                return nil, connectionError
        }

        pingContext, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer pingCancel()

        pingError := databaseConnection.PingContext(pingContext)
        if pingError != nil {
                logConnectionTroubleshootingGuidance(pingError)
                return nil, pingError
        }

        connector := &PostgresConnector{Database: databaseConnection}
        migrationError := connector.ensureSchema()
        if migrationError != nil {
                return nil, migrationError
        }

        log.Println("Connected to PostgreSQL and ensured schema")
        return connector, nil
}

func (connector *PostgresConnector) ensureSchema() error {
        schemaCreationSQL := `
CREATE TABLE IF NOT EXISTS transactions (
    id SERIAL PRIMARY KEY,
    operation_type VARCHAR(10) NOT NULL,
    asset_symbol VARCHAR(20) NOT NULL,
    quantity NUMERIC(20,8) NOT NULL,
    price_per_unit NUMERIC(20,8) NOT NULL,
    notes TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS email_alerts (
    id SERIAL PRIMARY KEY,
    recipient_address VARCHAR(255) NOT NULL,
    subject TEXT NOT NULL,
    message_body TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS binance_credentials (
    id SERIAL PRIMARY KEY,
    api_key TEXT NOT NULL,
    api_secret TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
`

	_, executionError := connector.Database.Exec(schemaCreationSQL)
	return executionError
}

func logConnectionTroubleshootingGuidance(connectionError error) {
        errorMessage := connectionError.Error()

        if strings.Contains(errorMessage, "role") && strings.Contains(errorMessage, "does not exist") {
                log.Println("The configured database user does not exist inside the PostgreSQL data volume.")
                log.Println("If you recently changed DB_USER or DB_PASSWORD, recreate the db_data volume or align credentials with the original database owner.")
                return
        }

        if strings.Contains(errorMessage, "password authentication failed") {
                log.Println("PostgreSQL rejected the supplied credentials. Confirm DB_USER and DB_PASSWORD match the initialized database or recreate the db_data volume.")
        }
}

func (connector *PostgresConnector) Close() error {
	return connector.Database.Close()
}
