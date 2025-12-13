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

	transactionRepository := repository.NewPostgresTransactionRepository(postgresConnector.Database)
	emailAlertRepository := repository.NewPostgresEmailAlertRepository(postgresConnector.Database)
	credentialRepository := repository.NewPostgresBinanceCredentialRepository(postgresConnector.Database)

	transactionService := service.NewTransactionService(transactionRepository)
	emailAlertService := service.NewEmailAlertService(emailAlertRepository, applicationConfiguration.EmailSenderAddress, applicationConfiguration.EmailSenderPassword, applicationConfiguration.EmailSMTPHost, applicationConfiguration.EmailSMTPPort)
	automationService := service.NewAutomationService(transactionService, applicationConfiguration.AutomaticSellIntervalMinutes, applicationConfiguration.DailyPurchaseIntervalMinutes)
        binanceCredentialValidator := service.NewBinanceCredentialValidator(applicationConfiguration.BinanceAPIBaseURL)
        credentialService := service.NewCredentialService(credentialRepository, binanceCredentialValidator, applicationConfiguration.BinanceAPIKey, applicationConfiguration.BinanceAPISecret)
        credentialService.InitializeCredentials(context.Background())
        binanceSymbolService := service.NewBinanceSymbolService(applicationConfiguration.BinanceAPIBaseURL)

	parsedTemplates, templateError := parseHTMLTemplates("templates")
	if templateError != nil {
		log.Fatalf("Could not parse templates: %v", templateError)
	}

	server := httpserver.NewServer(transactionService, emailAlertService, automationService, credentialService, binanceSymbolService, parsedTemplates)
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
