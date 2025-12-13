package httpserver

import (
    "context"
    "html/template"
    "log"
    "net/http"
    "strconv"
    "time"

    "coin-alert/internal/domain"
    "coin-alert/internal/service"
)

type Server struct {
    TransactionService *service.TransactionService
    EmailAlertService  *service.EmailAlertService
    AutomationService  *service.AutomationService
    Templates          *template.Template
}

func NewServer(transactionService *service.TransactionService, emailAlertService *service.EmailAlertService, automationService *service.AutomationService, templates *template.Template) *Server {
    return &Server{
        TransactionService: transactionService,
        EmailAlertService:  emailAlertService,
        AutomationService:  automationService,
        Templates:          templates,
    }
}

func (server *Server) RegisterRoutes() http.Handler {
    router := http.NewServeMux()
    router.HandleFunc("/", server.renderDashboard)
    router.HandleFunc("/transactions/buy", server.handleBuyRequest)
    router.HandleFunc("/transactions/sell", server.handleSellRequest)
    router.HandleFunc("/alerts/email", server.handleEmailAlertRequest)
    router.HandleFunc("/health", server.handleHealthCheck)
    router.HandleFunc("/transactions", server.handleListTransactions)
    return router
}

func (server *Server) renderDashboard(responseWriter http.ResponseWriter, request *http.Request) {
    if request.Method != http.MethodGet {
        responseWriter.WriteHeader(http.StatusMethodNotAllowed)
        return
    }

    renderContext := map[string]interface{}{}
    templateError := server.Templates.ExecuteTemplate(responseWriter, "index.html", renderContext)
    if templateError != nil {
        log.Printf("Template render error: %v", templateError)
        http.Error(responseWriter, "Could not render page", http.StatusInternalServerError)
    }
}

func (server *Server) handleBuyRequest(responseWriter http.ResponseWriter, request *http.Request) {
    server.handleTransactionRequest(responseWriter, request, "BUY")
}

func (server *Server) handleSellRequest(responseWriter http.ResponseWriter, request *http.Request) {
    server.handleTransactionRequest(responseWriter, request, "SELL")
}

func (server *Server) handleTransactionRequest(responseWriter http.ResponseWriter, request *http.Request, operationType string) {
    if request.Method != http.MethodPost {
        responseWriter.WriteHeader(http.StatusMethodNotAllowed)
        return
    }

    quantity, quantityError := strconv.ParseFloat(request.FormValue("quantity"), 64)
    if quantityError != nil {
        http.Error(responseWriter, "Quantidade inválida", http.StatusBadRequest)
        return
    }

    pricePerUnit, priceParseError := strconv.ParseFloat(request.FormValue("price_per_unit"), 64)
    if priceParseError != nil {
        http.Error(responseWriter, "Preço inválido", http.StatusBadRequest)
        return
    }

    transaction := domain.Transaction{
        OperationType: operationType,
        AssetSymbol:   request.FormValue("asset_symbol"),
        Quantity:      quantity,
        PricePerUnit:  pricePerUnit,
        Notes:         request.FormValue("notes"),
    }

    contextWithTimeout, cancel := context.WithTimeout(request.Context(), 5*time.Second)
    defer cancel()

    _, creationError := server.TransactionService.RecordTransaction(contextWithTimeout, transaction)
    if creationError != nil {
        log.Printf("Transaction creation failed: %v", creationError)
        http.Error(responseWriter, creationError.Error(), http.StatusBadRequest)
        return
    }

    http.Redirect(responseWriter, request, "/", http.StatusSeeOther)
}

func (server *Server) handleEmailAlertRequest(responseWriter http.ResponseWriter, request *http.Request) {
    if request.Method != http.MethodPost {
        responseWriter.WriteHeader(http.StatusMethodNotAllowed)
        return
    }

    alert := domain.EmailAlert{
        RecipientAddress: request.FormValue("recipient_address"),
        Subject:          request.FormValue("subject"),
        MessageBody:      request.FormValue("message_body"),
    }

    contextWithTimeout, cancel := context.WithTimeout(request.Context(), 10*time.Second)
    defer cancel()

    _, sendError := server.EmailAlertService.SendAndLogAlert(contextWithTimeout, alert)
    if sendError != nil {
        log.Printf("Email alert failed: %v", sendError)
        http.Error(responseWriter, sendError.Error(), http.StatusBadRequest)
        return
    }

    http.Redirect(responseWriter, request, "/", http.StatusSeeOther)
}

func (server *Server) handleListTransactions(responseWriter http.ResponseWriter, request *http.Request) {
    if request.Method != http.MethodGet {
        responseWriter.WriteHeader(http.StatusMethodNotAllowed)
        return
    }

    contextWithTimeout, cancel := context.WithTimeout(request.Context(), 5*time.Second)
    defer cancel()

    transactions, listError := server.TransactionService.ListTransactions(contextWithTimeout, 100)
    if listError != nil {
        log.Printf("List transactions failed: %v", listError)
        http.Error(responseWriter, listError.Error(), http.StatusInternalServerError)
        return
    }

    renderError := server.Templates.ExecuteTemplate(responseWriter, "partials/transactions.html", transactions)
    if renderError != nil {
        log.Printf("Partial render failed: %v", renderError)
        http.Error(responseWriter, "Could not render transactions", http.StatusInternalServerError)
        return
    }
}

func (server *Server) handleHealthCheck(responseWriter http.ResponseWriter, request *http.Request) {
    responseWriter.WriteHeader(http.StatusOK)
    responseWriter.Write([]byte("ok"))
}
