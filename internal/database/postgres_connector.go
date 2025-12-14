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

	log.Println("Connected to PostgreSQL")
	return &PostgresConnector{Database: databaseConnection}, nil
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
