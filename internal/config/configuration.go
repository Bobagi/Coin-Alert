package config

import (
        "log"
        "os"
        "strconv"

        "coin-alert/internal/domain"
)

type ApplicationConfiguration struct {
        ServerPort                   string
        DatabaseURL                  string
        ApplicationBaseURL           string
        AutomaticSellIntervalMinutes int
        DailyPurchaseIntervalMinutes int
        TradingPairSymbol            string
        TradingCapitalThreshold      float64
        TargetProfitPercent          float64
        EmailSenderAddress           string
        EmailSenderPassword          string
        EmailSMTPHost                string
        EmailSMTPPort                int
        BinanceAPIKey                string
        BinanceAPISecret             string
        BinanceAPIBaseURL            string
        BinanceEnvironment           string
}

func LoadApplicationConfiguration() ApplicationConfiguration {
        automaticSellIntervalMinutes := parseIntegerWithDefault("AUTO_SELL_INTERVAL_MINUTES", 60)
        dailyPurchaseIntervalMinutes := parseIntegerWithDefault("DAILY_PURCHASE_INTERVAL_MINUTES", 1440)
        emailSMTPPort := parseIntegerWithDefault("EMAIL_SMTP_PORT", 587)
        binanceEnvironment := resolveBinanceEnvironment()
        binanceAPIBaseURL := resolveBinanceBaseURL(binanceEnvironment)
        tradingCapitalThreshold := parseFloatWithDefault("TRADING_CAPITAL_THRESHOLD", 100)
        targetProfitPercent := parseFloatWithDefault("TARGET_PROFIT_PERCENT", 10)

        configuration := ApplicationConfiguration{
                ServerPort:                   getEnvironmentValueWithDefault("API_PORT", "5020"),
                DatabaseURL:                  buildDatabaseURL(),
                ApplicationBaseURL:           getEnvironmentValueWithDefault("API_URL", "http://localhost:5020"),
                AutomaticSellIntervalMinutes: automaticSellIntervalMinutes,
                DailyPurchaseIntervalMinutes: dailyPurchaseIntervalMinutes,
                TradingPairSymbol:            getEnvironmentValueWithDefault("TRADE_SYMBOL", "BTCUSDT"),
                TradingCapitalThreshold:      tradingCapitalThreshold,
                TargetProfitPercent:          targetProfitPercent,
                EmailSenderAddress:           getEnvironmentValueWithDefault("EMAIL_SENDER_ADDRESS", ""),
                EmailSenderPassword:          getEnvironmentValueWithDefault("EMAIL_SENDER_PASSWORD", ""),
                EmailSMTPHost:                getEnvironmentValueWithDefault("EMAIL_SMTP_HOST", ""),
                EmailSMTPPort:                emailSMTPPort,
                BinanceAPIKey:                getEnvironmentValueWithDefault("BINANCE_API_KEY", ""),
                BinanceAPISecret:             getEnvironmentValueWithDefault("BINANCE_API_SECRET", ""),
                BinanceAPIBaseURL:            binanceAPIBaseURL,
                BinanceEnvironment:           binanceEnvironment,
        }

        logNonSensitiveConfiguration(configuration)

        return configuration
}

func logNonSensitiveConfiguration(configuration ApplicationConfiguration) {
        log.Printf(
                "Loaded configuration (non-sensitive): serverPort=%s applicationBaseURL=%s automaticSellIntervalMinutes=%d dailyPurchaseIntervalMinutes=%d tradingPair=%s capitalThreshold=%.2f targetProfitPercent=%.2f emailSMTPHost=%s emailSMTPPort=%d binanceEnvironment=%s",
                configuration.ServerPort,
                configuration.ApplicationBaseURL,
                configuration.AutomaticSellIntervalMinutes,
                configuration.DailyPurchaseIntervalMinutes,
                configuration.TradingPairSymbol,
                configuration.TradingCapitalThreshold,
                configuration.TargetProfitPercent,
                configuration.EmailSMTPHost,
                configuration.EmailSMTPPort,
                configuration.BinanceEnvironment,
        )
}

func resolveBinanceEnvironment() string {
        explicitEnvironment := os.Getenv("BINANCE_ENVIRONMENT")
        if explicitEnvironment != "" {
                return domain.NormalizeBinanceEnvironment(explicitEnvironment)
        }

        useTestnetFlag := parseBooleanWithDefault("BINANCE_TESTNET", false)
        if useTestnetFlag {
                return domain.BinanceEnvironmentTestnet
        }

        return domain.BinanceEnvironmentProduction
}

func resolveBinanceBaseURL(environmentName string) string {
        explicitBaseURL := os.Getenv("BINANCE_API_BASE_URL")

        if explicitBaseURL != "" {
                return explicitBaseURL
        }

        normalizedEnvironment := domain.NormalizeBinanceEnvironment(environmentName)
        if normalizedEnvironment == domain.BinanceEnvironmentProduction {
                return "https://api.binance.com"
        }

        return "https://testnet.binance.vision"
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

func parseBooleanWithDefault(variableName string, defaultValue bool) bool {
        environmentValue := os.Getenv(variableName)
        if environmentValue == "" {
                return defaultValue
        }

        switch environmentValue {
        case "true", "1", "yes", "on", "TRUE", "True":
                return true
        case "false", "0", "no", "off", "FALSE", "False":
                return false
        default:
                log.Printf("Invalid boolean for %s, using default %t", variableName, defaultValue)
                return defaultValue
        }
}

func parseFloatWithDefault(variableName string, defaultValue float64) float64 {
        environmentValue := os.Getenv(variableName)
        if environmentValue == "" {
                return defaultValue
        }

        parsedFloat, parsingError := strconv.ParseFloat(environmentValue, 64)
        if parsingError != nil {
                log.Printf("Invalid decimal for %s, using default %.2f", variableName, defaultValue)
                return defaultValue
        }

        return parsedFloat
}

func getEnvironmentValueWithDefault(variableName string, defaultValue string) string {
        environmentValue := os.Getenv(variableName)
        if environmentValue == "" {
                return defaultValue
        }

        return environmentValue
}
