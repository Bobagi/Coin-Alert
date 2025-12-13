package config

import (
    "log"
    "os"
    "strconv"
)

type ApplicationConfiguration struct {
    ServerPort                    string
    DatabaseURL                   string
    ApplicationBaseURL            string
    AutomaticSellIntervalMinutes  int
    DailyPurchaseIntervalMinutes  int
    EmailSenderAddress            string
    EmailSenderPassword           string
    EmailSMTPHost                 string
    EmailSMTPPort                 int
}

func LoadApplicationConfiguration() ApplicationConfiguration {
    automaticSellIntervalMinutes := parseIntegerWithDefault("AUTO_SELL_INTERVAL_MINUTES", 60)
    dailyPurchaseIntervalMinutes := parseIntegerWithDefault("DAILY_PURCHASE_INTERVAL_MINUTES", 1440)
    emailSMTPPort := parseIntegerWithDefault("EMAIL_SMTP_PORT", 587)

    configuration := ApplicationConfiguration{
        ServerPort:                   getEnvironmentValueWithDefault("API_PORT", "5020"),
        DatabaseURL:                  buildDatabaseURL(),
        ApplicationBaseURL:           getEnvironmentValueWithDefault("API_URL", "http://localhost:5020"),
        AutomaticSellIntervalMinutes: automaticSellIntervalMinutes,
        DailyPurchaseIntervalMinutes: dailyPurchaseIntervalMinutes,
        EmailSenderAddress:           getEnvironmentValueWithDefault("EMAIL_SENDER_ADDRESS", ""),
        EmailSenderPassword:          getEnvironmentValueWithDefault("EMAIL_SENDER_PASSWORD", ""),
        EmailSMTPHost:                getEnvironmentValueWithDefault("EMAIL_SMTP_HOST", ""),
        EmailSMTPPort:                emailSMTPPort,
    }

    log.Printf("Loaded configuration: %+v", configuration)

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

func parseIntegerWithDefault(variableName string, defaultValue int) int {
    environmentValue := os.Getenv(variableName)
    if environmentValue == "" {
        return defaultValue
    }

    parsedInteger, parsingError := strconv.Atoi(environmentValue)
    if parsingError != nil {
        log.Printf("Invalid integer for %s, using default %d", variableName, defaultValue)
        return defaultValue
    }

    return parsedInteger
}

func getEnvironmentValueWithDefault(variableName string, defaultValue string) string {
    environmentValue := os.Getenv(variableName)
    if environmentValue == "" {
        return defaultValue
    }

    return environmentValue
}
