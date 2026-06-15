package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"coin-alert/internal/config"
	"coin-alert/internal/database"
	"coin-alert/internal/email"
	"coin-alert/internal/httpserver"
	"coin-alert/internal/repository"
	"coin-alert/internal/security"
	"coin-alert/internal/service"
)

func main() {
	applicationConfiguration := config.LoadApplicationConfiguration()

	postgresConnector, connectionError := database.InitializePostgresConnector(applicationConfiguration.DatabaseURL)
	if connectionError != nil {
		log.Fatalf("Could not connect to database: %v", connectionError)
	}
	defer postgresConnector.Close()

	// Repositories.
	userRepository := repository.NewPostgresUserRepository(postgresConnector.Database)
	userSessionRepository := repository.NewPostgresUserSessionRepository(postgresConnector.Database)
	userTradingSettingsRepository := repository.NewPostgresUserTradingSettingsRepository(postgresConnector.Database)
	binanceCredentialRepository := repository.NewPostgresBinanceCredentialRepository(postgresConnector.Database)
	tradingOperationRepository := repository.NewPostgresTradingOperationRepository(postgresConnector.Database)
	tradingOperationExecutionRepository := repository.NewPostgresTradingOperationExecutionRepository(postgresConnector.Database)
	tradingRobotRepository := repository.NewPostgresTradingRobotRepository(postgresConnector.Database)
	userPortfolioRepository := repository.NewPostgresUserPortfolioRepository(postgresConnector.Database)
	accountDeletionAuditRepository := repository.NewPostgresAccountDeletionAuditRepository(postgresConnector.Database)
	authTokenRepository := repository.NewPostgresAuthTokenRepository(postgresConnector.Database)

	// Encryption for Binance secrets at rest. Without a key, credential storage is refused at runtime.
	secretCipher, secretCipherError := security.NewSecretCipher(os.Getenv("CREDENTIALS_ENCRYPTION_KEY"))
	if secretCipherError != nil {
		log.Printf("WARNING: credential encryption is disabled until CREDENTIALS_ENCRYPTION_KEY is set: %v", secretCipherError)
	}

	testnetBaseURL := environmentValueOrDefault("BINANCE_TESTNET_BASE_URL", "https://testnet.binance.vision")
	productionBaseURL := environmentValueOrDefault("BINANCE_PRODUCTION_BASE_URL", "https://api.binance.com")

	// Authentication.
	passwordService := service.NewPasswordService()
	// 7-day sessions: short enough to limit the blast radius of a stolen cookie, long enough not to
	// nag a regular user. Override with SESSION_LIFETIME_HOURS.
	sessionService := service.NewSessionService(userSessionRepository, sessionLifetimeFromEnv(168*time.Hour))
	authService := service.NewAuthService(userRepository, userTradingSettingsRepository, accountDeletionAuditRepository, passwordService, secretCipher)
	secureSessionCookies := os.Getenv("APP_SECURE_COOKIES") != "false"
	googleOAuthService := service.NewGoogleOAuthService(
		os.Getenv("GOOGLE_OAUTH_CLIENT_ID"),
		os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET"),
		os.Getenv("GOOGLE_OAUTH_REDIRECT_URL"),
	)
	if googleOAuthService != nil {
		log.Println("Google sign-in is enabled")
	}
	emailSender := email.NewSenderFromEnv()
	accountEmailService := service.NewAccountEmailService(userRepository, authTokenRepository, userSessionRepository, passwordService, emailSender, environmentValueOrDefault("APP_BASE_URL", "https://coin.bobagi.space"))
	authHandler := httpserver.NewAuthHandler(authService, sessionService, googleOAuthService, accountEmailService, secureSessionCookies)
	accountHandler := httpserver.NewAccountHandler(authService, sessionService, authHandler.CookieName, secureSessionCookies)

	// Per-user trading configuration and Binance credentials.
	userCredentialService := service.NewUserCredentialService(binanceCredentialRepository, secretCipher, testnetBaseURL, productionBaseURL)
	apiHandler := httpserver.NewAPIHandler(sessionService, authService, authHandler.CookieName, userTradingSettingsRepository, userCredentialService, testnetBaseURL, productionBaseURL)

	userTradingService := service.NewUserTradingService(userCredentialService, userTradingSettingsRepository, tradingOperationRepository, tradingOperationExecutionRepository, maxQuoteAmountPerOrderFromEnv(100000))
	operationsHandler := httpserver.NewOperationsHandler(sessionService, authService, authHandler.CookieName, userTradingService)

	robotService := service.NewRobotService(tradingRobotRepository, userCredentialService)
	robotsHandler := httpserver.NewRobotsHandler(sessionService, authService, authHandler.CookieName, robotService)

	automationWorker := service.NewAutomationWorker(userRepository, userCredentialService, tradingRobotRepository, tradingOperationRepository, tradingOperationExecutionRepository, tradingOperationExecutionRepository, userTradingService, 30*time.Second)

	portfolioScraperClient := service.NewPortfolioScraperClient(environmentValueOrDefault("SCRAPER_BASE_URL", "http://scraper:5000"))
	portfolioHandler := httpserver.NewPortfolioHandler(sessionService, authService, authHandler.CookieName, userPortfolioRepository, portfolioScraperClient)

	rootRouter := http.NewServeMux()
	authHandler.RegisterRoutes(rootRouter)
	accountHandler.RegisterRoutes(rootRouter)
	apiHandler.RegisterRoutes(rootRouter)
	operationsHandler.RegisterRoutes(rootRouter)
	robotsHandler.RegisterRoutes(rootRouter)
	portfolioHandler.RegisterRoutes(rootRouter)
	rootRouter.HandleFunc("/health", func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte("ok"))
	})

	applicationContext, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	automationWorker.Start(applicationContext)
	sessionService.StartExpiredSessionCleanup(applicationContext, time.Hour)

	// Wrap every route with the body-size cap + same-origin (CSRF) guard. The allowed origin is the
	// public app URL the SPA is served from.
	allowedOrigin := environmentValueOrDefault("APP_BASE_URL", "https://coin.bobagi.space")
	securedHandler := httpserver.SecurityMiddleware(rootRouter, allowedOrigin)

	serverAddress := ":" + applicationConfiguration.ServerPort
	httpServer := &http.Server{Addr: serverAddress, Handler: securedHandler}

	go func() {
		log.Printf("Coin Hub API listening on %s", serverAddress)
		startError := httpServer.ListenAndServe()
		if startError != nil && startError != http.ErrServerClosed {
			log.Fatalf("Server error: %v", startError)
		}
	}()

	<-applicationContext.Done()
	shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if shutdownError := httpServer.Shutdown(shutdownContext); shutdownError != nil {
		log.Printf("Graceful shutdown failed: %v", shutdownError)
	}
	log.Println("Application stopped")
}

func environmentValueOrDefault(variableName string, fallbackValue string) string {
	if value := os.Getenv(variableName); value != "" {
		return value
	}
	return fallbackValue
}

// sessionLifetimeFromEnv reads SESSION_LIFETIME_HOURS (positive integer hours), falling back to the
// provided default when unset or invalid.
func sessionLifetimeFromEnv(fallback time.Duration) time.Duration {
	raw := os.Getenv("SESSION_LIFETIME_HOURS")
	if raw == "" {
		return fallback
	}
	hours, parseError := strconv.Atoi(raw)
	if parseError != nil || hours <= 0 {
		return fallback
	}
	return time.Duration(hours) * time.Hour
}

// maxQuoteAmountPerOrderFromEnv reads MAX_ORDER_QUOTE_AMOUNT (the per-order spending ceiling), falling
// back to the provided default. A value of 0 disables the cap.
func maxQuoteAmountPerOrderFromEnv(fallback float64) float64 {
	raw := os.Getenv("MAX_ORDER_QUOTE_AMOUNT")
	if raw == "" {
		return fallback
	}
	parsed, parseError := strconv.ParseFloat(raw, 64)
	if parseError != nil || parsed < 0 {
		return fallback
	}
	return parsed
}
