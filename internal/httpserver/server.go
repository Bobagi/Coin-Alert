package httpserver

import (
	"context"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"coin-alert/internal/domain"
	"coin-alert/internal/service"
)

type Server struct {
	TransactionService   *service.TransactionService
	EmailAlertService    *service.EmailAlertService
	AutomationService    *service.AutomationService
	CredentialService    *service.CredentialService
	BinanceSymbolService *service.BinanceSymbolService
	Templates            *template.Template
}

func NewServer(transactionService *service.TransactionService, emailAlertService *service.EmailAlertService, automationService *service.AutomationService, credentialService *service.CredentialService, binanceSymbolService *service.BinanceSymbolService, templates *template.Template) *Server {
	return &Server{
		TransactionService:   transactionService,
		EmailAlertService:    emailAlertService,
		AutomationService:    automationService,
		CredentialService:    credentialService,
		BinanceSymbolService: binanceSymbolService,
		Templates:            templates,
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
	router.HandleFunc("/settings/binance", server.handleUpdateBinanceCredentials)
	router.HandleFunc("/binance/symbols", server.handleBinanceSymbols)
	return router
}

func (server *Server) renderDashboard(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if !server.CredentialService.HasSuppliedBinanceCredentials() {
		responseWriter.WriteHeader(http.StatusServiceUnavailable)
		server.renderErrorPage(responseWriter, "Binance credentials are missing. Provide an API Key and Secret Key to enable dashboard actions.")
		return
	}

	dashboardContext, contextError := server.buildDashboardViewModel(request.Context())
	if contextError != nil {
		log.Printf("Dashboard data error: %v", contextError)
		http.Error(responseWriter, "Could not load dashboard data", http.StatusInternalServerError)
		return
	}

	templateError := server.Templates.ExecuteTemplate(responseWriter, "index.html", dashboardContext)
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

	if !server.CredentialService.HasValidBinanceCredentials() {
		responseWriter.WriteHeader(http.StatusServiceUnavailable)
		server.renderErrorPage(responseWriter, "Binance credentials are missing or invalid. Please provide a valid API Key and Secret Key to continue.")
		return
	}

	quantity, quantityError := strconv.ParseFloat(request.FormValue("quantity"), 64)
	if quantityError != nil {
		http.Error(responseWriter, "Invalid quantity", http.StatusBadRequest)
		return
	}

	pricePerUnit, priceParseError := strconv.ParseFloat(request.FormValue("price_per_unit"), 64)
	if priceParseError != nil {
		http.Error(responseWriter, "Invalid price", http.StatusBadRequest)
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

	if !server.CredentialService.HasValidBinanceCredentials() {
		responseWriter.WriteHeader(http.StatusServiceUnavailable)
		server.renderErrorPage(responseWriter, "Binance credentials are missing or invalid. Please provide a valid API Key and Secret Key to continue.")
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

	if !server.CredentialService.HasValidBinanceCredentials() {
		responseWriter.WriteHeader(http.StatusServiceUnavailable)
		server.renderErrorPage(responseWriter, "Binance credentials are missing or invalid. Please provide a valid API Key and Secret Key to continue.")
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

func (server *Server) handleUpdateBinanceCredentials(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	providedAPIKey := request.FormValue("binance_api_key")
	providedAPISecret := request.FormValue("binance_api_secret")

	validationContext, cancel := context.WithTimeout(request.Context(), 8*time.Second)
	defer cancel()

	validationError := server.CredentialService.ValidateAndPersistCredentials(validationContext, providedAPIKey, providedAPISecret)
	if validationError != nil {
		log.Printf("Binance credential validation failed: %v", validationError)
		responseWriter.WriteHeader(http.StatusBadRequest)
		server.renderErrorPage(responseWriter, validationError.Error())
		return
	}

	http.Redirect(responseWriter, request, "/", http.StatusSeeOther)
}

func (server *Server) handleBinanceSymbols(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	contextWithTimeout, cancel := context.WithTimeout(request.Context(), 6*time.Second)
	defer cancel()

	availableSymbols, fetchError := server.BinanceSymbolService.FetchAvailableSymbols(contextWithTimeout)
	if fetchError != nil {
		log.Printf("Could not fetch Binance symbols: %v", fetchError)
		http.Error(responseWriter, "Could not fetch Binance tradable symbols", http.StatusBadGateway)
		return
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	encodeError := json.NewEncoder(responseWriter).Encode(BinanceSymbolsResponse{Symbols: availableSymbols})
	if encodeError != nil {
		log.Printf("Could not encode symbols: %v", encodeError)
		http.Error(responseWriter, "Failed to serialize Binance symbols", http.StatusInternalServerError)
	}
}

func (server *Server) buildDashboardViewModel(requestContext context.Context) (*DashboardViewModel, error) {
	contextWithTimeout, cancel := context.WithTimeout(requestContext, 5*time.Second)
	defer cancel()

	transactions, listError := server.TransactionService.ListTransactions(contextWithTimeout, 100)
	if listError != nil {
		return nil, listError
	}

	return &DashboardViewModel{
		Transactions:           transactions,
		IsBinanceConfigured:    server.CredentialService.HasValidBinanceCredentials(),
		BinanceAPIKeyMasked:    server.CredentialService.GetMaskedBinanceAPIKey(),
		BinanceAPISecretMasked: server.CredentialService.GetMaskedBinanceAPISecret(),
	}, nil
}

func (server *Server) renderErrorPage(responseWriter http.ResponseWriter, message string) {
	errorContext := map[string]string{"Message": message}
	templateError := server.Templates.ExecuteTemplate(responseWriter, "error.html", errorContext)
	if templateError != nil {
		log.Printf("Error template render failed: %v", templateError)
		http.Error(responseWriter, message, http.StatusInternalServerError)
	}
}

type DashboardViewModel struct {
	Transactions           []domain.Transaction
	IsBinanceConfigured    bool
	BinanceAPIKeyMasked    string
	BinanceAPISecretMasked string
}

type BinanceSymbolsResponse struct {
	Symbols []string `json:"symbols"`
}

func (server *Server) handleHealthCheck(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.WriteHeader(http.StatusOK)
	responseWriter.Write([]byte("ok"))
}
