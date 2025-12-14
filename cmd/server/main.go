package main

import (
        "context"
        "html/template"
        "log"
        "net/http"
        "os"
        "os/signal"
        "path/filepath"
        "syscall"
        "time"

        "coin-alert/internal/config"
        "coin-alert/internal/database"
        "coin-alert/internal/domain"
        "coin-alert/internal/httpserver"
        "coin-alert/internal/repository"
        "coin-alert/internal/service"
)

func main() {
        applicationConfiguration := config.LoadApplicationConfiguration()

        postgresConnector, connectionError := database.InitializePostgresConnector(applicationConfiguration.DatabaseURL)
        if connectionError != nil {
                log.Fatalf("Could not connect to database: %v", connectionError)
        }
        defer postgresConnector.Close()

        tradingOperationRepository := repository.NewPostgresTradingOperationRepository(postgresConnector.Database)
        emailAlertRepository := repository.NewPostgresEmailAlertRepository(postgresConnector.Database)
        credentialRepository := repository.NewPostgresBinanceCredentialRepository(postgresConnector.Database)
        scheduledOperationRepository := repository.NewPostgresScheduledTradingOperationRepository(postgresConnector.Database)
        executionRepository := repository.NewPostgresTradingOperationExecutionRepository(postgresConnector.Database)

        initialEnvironmentConfiguration := domain.BinanceEnvironmentConfiguration{
                EnvironmentName: applicationConfiguration.BinanceEnvironment,
                RESTBaseURL:     applicationConfiguration.BinanceAPIBaseURL,
                APIKey:          applicationConfiguration.BinanceAPIKey,
                APISecret:       applicationConfiguration.BinanceAPISecret,
        }

        tradingOperationService := service.NewTradingOperationService(tradingOperationRepository, applicationConfiguration.TradingPairSymbol, applicationConfiguration.TradingCapitalThreshold, applicationConfiguration.TargetProfitPercent)
        emailAlertService := service.NewEmailAlertService(emailAlertRepository, applicationConfiguration.EmailSenderAddress, applicationConfiguration.EmailSenderPassword, applicationConfiguration.EmailSMTPHost, applicationConfiguration.EmailSMTPPort)
        tradingScheduleService := service.NewTradingScheduleService(scheduledOperationRepository, executionRepository, applicationConfiguration.AutomaticSellIntervalMinutes, applicationConfiguration.TradingPairSymbol, applicationConfiguration.TradingCapitalThreshold, applicationConfiguration.TargetProfitPercent)
        binancePriceService := service.NewBinancePriceService(initialEnvironmentConfiguration)
        automationService := service.NewTradingAutomationService(tradingOperationService, binancePriceService, tradingScheduleService, applicationConfiguration.TradingPairSymbol, applicationConfiguration.AutomaticSellIntervalMinutes)
        binanceCredentialValidator := service.NewBinanceCredentialValidator(initialEnvironmentConfiguration.RESTBaseURL)
        credentialService := service.NewCredentialService(credentialRepository, binanceCredentialValidator, initialEnvironmentConfiguration)
        credentialService.InitializeCredentials(context.Background())
        activeEnvironment := credentialService.GetActiveEnvironmentConfiguration()
        binancePriceService.UpdateEnvironmentConfiguration(activeEnvironment)
        binanceSymbolService := service.NewBinanceSymbolService(activeEnvironment)
        binanceTradingService := service.NewBinanceTradingService(activeEnvironment)
        initialScheduleContext, initialScheduleCancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer initialScheduleCancel()
        automationService.EvaluateAndSellProfitableOperations(initialScheduleContext)

        parsedTemplates, templateError := parseHTMLTemplates("templates")
        if templateError != nil {
                log.Fatalf("Could not parse templates: %v", templateError)
        }

        dashboardSettingsSummary := httpserver.DashboardSettingsSummary{
                AutomaticSellIntervalMinutes: applicationConfiguration.AutomaticSellIntervalMinutes,
                DailyPurchaseIntervalMinutes: applicationConfiguration.DailyPurchaseIntervalMinutes,
                BinanceAPIBaseURL:            activeEnvironment.RESTBaseURL,
                ActiveBinanceEnvironment:     activeEnvironment.EnvironmentName,
                ApplicationBaseURL:           applicationConfiguration.ApplicationBaseURL,
                TradingPairSymbol:            applicationConfiguration.TradingPairSymbol,
                CapitalThreshold:             applicationConfiguration.TradingCapitalThreshold,
                TargetProfitPercent:          applicationConfiguration.TargetProfitPercent,
        }

        server := httpserver.NewServer(tradingOperationService, emailAlertService, automationService, credentialService, binanceSymbolService, binancePriceService, binanceTradingService, tradingScheduleService, dashboardSettingsSummary, parsedTemplates)
        router := server.RegisterRoutes()

        applicationContext, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
        defer cancel()

        automationService.StartBackgroundJobs(applicationContext)

        serverAddress := ":" + applicationConfiguration.ServerPort
        httpServer := &http.Server{Addr: serverAddress, Handler: router}

        go func() {
                log.Printf("Server running on %s", serverAddress)
                log.Printf("Dashboard available at http://localhost:%s", applicationConfiguration.ServerPort)
                startError := httpServer.ListenAndServe()
                if startError != nil && startError != http.ErrServerClosed {
                        log.Fatalf("Server error: %v", startError)
                }
        }()

        <-applicationContext.Done()
        shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer shutdownCancel()

        shutdownError := httpServer.Shutdown(shutdownContext)
        if shutdownError != nil {
                log.Printf("Graceful shutdown failed: %v", shutdownError)
        }

        log.Println("Application stopped")
}

func parseHTMLTemplates(templatesDirectory string) (*template.Template, error) {
        rootTemplatesPattern := filepath.Join(templatesDirectory, "*.html")
        parsedRootTemplates, rootTemplatesParseError := template.ParseGlob(rootTemplatesPattern)
        if rootTemplatesParseError != nil {
                return nil, rootTemplatesParseError
        }

        partialTemplatesPattern := filepath.Join(templatesDirectory, "partials", "*.html")
        parsedTemplatesWithPartials, partialTemplatesParseError := parsedRootTemplates.ParseGlob(partialTemplatesPattern)
        if partialTemplatesParseError != nil {
                return nil, partialTemplatesParseError
        }

        return parsedTemplatesWithPartials, nil
}
