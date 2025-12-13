package main

import (
    "context"
    "html/template"
    "log"
    "net/http"
    "os"
    "os/signal"
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

    transactionService := service.NewTransactionService(transactionRepository)
    emailAlertService := service.NewEmailAlertService(emailAlertRepository, applicationConfiguration.EmailSenderAddress, applicationConfiguration.EmailSenderPassword, applicationConfiguration.EmailSMTPHost, applicationConfiguration.EmailSMTPPort)
    automationService := service.NewAutomationService(transactionService, applicationConfiguration.AutomaticSellIntervalMinutes, applicationConfiguration.DailyPurchaseIntervalMinutes)

    parsedTemplates, templateError := template.ParseGlob("templates/**/*.html")
    if templateError != nil {
        log.Fatalf("Could not parse templates: %v", templateError)
    }

    server := httpserver.NewServer(transactionService, emailAlertService, automationService, parsedTemplates)
    router := server.RegisterRoutes()

    applicationContext, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()

    automationService.StartBackgroundJobs(applicationContext)

    serverAddress := ":" + applicationConfiguration.ServerPort
    httpServer := &http.Server{Addr: serverAddress, Handler: router}

    go func() {
        log.Printf("Server running on %s", serverAddress)
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
