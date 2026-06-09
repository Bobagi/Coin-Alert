package config

import (
	"log"
	"os"
)

// ApplicationConfiguration holds the process-level settings the server needs at startup. Per-user
// trading settings and Binance credentials live in the database (encrypted, scoped per user), not
// here; this struct intentionally stays minimal.
type ApplicationConfiguration struct {
	ServerPort  string
	DatabaseURL string
}

func LoadApplicationConfiguration() ApplicationConfiguration {
	configuration := ApplicationConfiguration{
		ServerPort:  getEnvironmentValueWithDefault("API_PORT", "5020"),
		DatabaseURL: buildDatabaseURL(),
	}

	// Never log the database URL or any secrets — only the non-sensitive port.
	log.Printf("Loaded configuration (non-sensitive): serverPort=%s", configuration.ServerPort)

	return configuration
}

func buildDatabaseURL() string {
	databaseUser := getEnvironmentValueWithDefault("DB_USER", "postgres")
	databasePassword := getEnvironmentValueWithDefault("DB_PASSWORD", "postgres")
	databaseName := getEnvironmentValueWithDefault("DB_NAME", "coin_alert")
	databaseHost := getEnvironmentValueWithDefault("DB_HOST", "db")
	databasePort := getEnvironmentValueWithDefault("DB_PORT", "5432")

	return "postgres://" + databaseUser + ":" + databasePassword + "@" + databaseHost + ":" + databasePort + "/" + databaseName + "?sslmode=disable"
}

func getEnvironmentValueWithDefault(variableName string, defaultValue string) string {
	environmentValue := os.Getenv(variableName)
	if environmentValue == "" {
		return defaultValue
	}

	return environmentValue
}
