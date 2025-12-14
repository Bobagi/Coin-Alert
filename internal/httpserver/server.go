package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"coin-alert/internal/domain"
	"coin-alert/internal/service"
)

type Server struct {
	TradingOperationService *service.TradingOperationService
	EmailAlertService       *service.EmailAlertService
	AutomationService       *service.TradingAutomationService
	CredentialService       *service.CredentialService
	BinanceSymbolService    *service.BinanceSymbolService
	BinancePriceService     *service.BinancePriceService
	BinanceTradingService   *service.BinanceTradingService
	TradingScheduleService  *service.TradingScheduleService
	SettingsSummary         DashboardSettingsSummary
	Templates               *template.Template
}

func NewServer(tradingOperationService *service.TradingOperationService, emailAlertService *service.EmailAlertService, automationService *service.TradingAutomationService, credentialService *service.CredentialService, binanceSymbolService *service.BinanceSymbolService, binancePriceService *service.BinancePriceService, binanceTradingService *service.BinanceTradingService, tradingScheduleService *service.TradingScheduleService, settingsSummary DashboardSettingsSummary, templates *template.Template) *Server {
	return &Server{
		TradingOperationService: tradingOperationService,
		EmailAlertService:       emailAlertService,
		AutomationService:       automationService,
		CredentialService:       credentialService,
		BinanceSymbolService:    binanceSymbolService,
		BinancePriceService:     binancePriceService,
		BinanceTradingService:   binanceTradingService,
		TradingScheduleService:  tradingScheduleService,
		SettingsSummary:         settingsSummary,
		Templates:               templates,
	}
}

func (server *Server) RegisterRoutes() http.Handler {
	router := http.NewServeMux()
	router.HandleFunc("/", server.renderDashboard)
	router.HandleFunc("/operations/purchase", server.handlePurchaseRequest)
	router.HandleFunc("/alerts/email", server.handleEmailAlertRequest)
	router.HandleFunc("/health", server.handleHealthCheck)
	router.HandleFunc("/operations", server.handleListOperations)
	router.HandleFunc("/operations/history", server.handleOperationHistory)
	router.HandleFunc("/settings/binance", server.handleUpdateBinanceCredentials)
	router.HandleFunc("/settings/binance/environment", server.handleUpdateBinanceEnvironment)
	router.HandleFunc("/settings/binance/revalidate", server.handleRevalidateBinanceCredentials)
	router.HandleFunc("/binance/symbols", server.handleBinanceSymbols)
	router.HandleFunc("/operations/execute-next", server.handleExecuteNextOperation)
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

func (server *Server) renderDashboardWithPurchaseError(responseWriter http.ResponseWriter, request *http.Request, statusCode int, userFacingMessage string) {
	responseWriter.WriteHeader(statusCode)
	dashboardContext, dashboardError := server.buildDashboardViewModel(request.Context())
	if dashboardError != nil {
		log.Printf("Dashboard reload failed while handling purchase error: %v", dashboardError)
		server.renderMinimalPurchaseError(responseWriter, userFacingMessage)
		return
	}
	dashboardContext.PurchaseErrorMessage = userFacingMessage
	templateError := server.Templates.ExecuteTemplate(responseWriter, "index.html", dashboardContext)
	if templateError != nil {
		log.Printf("Template render error while showing purchase error: %v", templateError)
		server.renderMinimalPurchaseError(responseWriter, userFacingMessage)
	}
}

func (server *Server) renderMinimalPurchaseError(responseWriter http.ResponseWriter, userFacingMessage string) {
	responseWriter.Header().Set("Content-Type", "text/html")
	_, writeError := responseWriter.Write([]byte("<html><body style=\"font-family:Arial; background:#0f172a; color:#e2e8f0; padding:24px;\"><h2>Purchase error</h2><p>" + template.HTMLEscapeString(userFacingMessage) + "</p><p>Please return to the dashboard and try again.</p></body></html>"))
	if writeError != nil {
		log.Printf("Could not render minimal purchase error message: %v", writeError)
	}
}

func (server *Server) handlePurchaseRequest(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if !server.CredentialService.HasValidBinanceCredentials() {
		responseWriter.WriteHeader(http.StatusServiceUnavailable)
		server.renderErrorPage(responseWriter, "Binance credentials are missing or invalid. Please provide a valid API Key and Secret Key to continue.")
		return
	}

	capitalThresholdText := request.FormValue("capital_threshold")
	capitalThreshold, capitalThresholdError := strconv.ParseFloat(capitalThresholdText, 64)
	if capitalThresholdError != nil {
		log.Printf("Invalid capital threshold supplied: %v", capitalThresholdError)
		server.renderDashboardWithPurchaseError(responseWriter, request, http.StatusBadRequest, "Invalid capital threshold. Please provide a numeric value.")
		return
	}

	targetProfitPercent, targetParseError := strconv.ParseFloat(request.FormValue("target_profit_percent"), 64)
	if targetParseError != nil {
		log.Printf("Invalid target profit percent supplied: %v", targetParseError)
		server.renderDashboardWithPurchaseError(responseWriter, request, http.StatusBadRequest, "Invalid profit percent. Please provide a numeric value.")
		return
	}

	currentPriceContext, priceCancel := context.WithTimeout(request.Context(), 6*time.Second)
	defer priceCancel()

	currentPricePerUnit, priceLookupError := server.BinancePriceService.GetCurrentPrice(currentPriceContext, request.FormValue("trading_pair_symbol"))
	if priceLookupError != nil {
		log.Printf("Could not fetch current price for purchase: %v", priceLookupError)
		server.renderDashboardWithPurchaseError(responseWriter, request, http.StatusBadGateway, "Could not fetch current price for the selected pair. Please try again.")
		return
	}

	if currentPricePerUnit <= 0 {
		log.Printf("Binance returned non-positive price for %s", request.FormValue("trading_pair_symbol"))
		server.renderDashboardWithPurchaseError(responseWriter, request, http.StatusBadRequest, "Current price is unavailable for this pair. Please try again later.")
		return
	}

	if capitalThreshold <= 0 {
		log.Printf("Capital threshold less than or equal to zero: %f", capitalThreshold)
		server.renderDashboardWithPurchaseError(responseWriter, request, http.StatusBadRequest, "Capital threshold must be greater than zero.")
		return
	}

	server.TradingOperationService.UpdateCapitalThreshold(capitalThreshold)
	server.TradingScheduleService.UpdateCapitalThreshold(capitalThreshold)
	server.SettingsSummary.CapitalThreshold = capitalThreshold

	server.TradingScheduleService.UpdateTargetProfitPercent(targetProfitPercent)
	server.SettingsSummary.TargetProfitPercent = targetProfitPercent

	buyExecutionContext, buyCancel := context.WithTimeout(request.Context(), 10*time.Second)
	defer buyCancel()

	buyOrderResponse, buyError := server.BinanceTradingService.PlaceMarketBuyByQuote(buyExecutionContext, request.FormValue("trading_pair_symbol"), capitalThreshold)
	if buyError != nil {
		server.logExecutionFailure(request.Context(), domain.TradingOperationTypeBuy, request.FormValue("trading_pair_symbol"), buyError)
		log.Printf("Buy failed for %s: %v", request.FormValue("trading_pair_symbol"), buyError)
		server.renderDashboardWithPurchaseError(responseWriter, request, http.StatusBadGateway, "Buy failed: "+buyError.Error())
		return
	}

	executedQuantity, executedQuantityError := strconv.ParseFloat(buyOrderResponse.ExecutedQty, 64)
	if executedQuantityError != nil || executedQuantity <= 0 {
		server.logExecutionFailure(request.Context(), domain.TradingOperationTypeBuy, request.FormValue("trading_pair_symbol"), errors.New("Binance returned an invalid executed quantity"))
		log.Printf("Buy failed for %s due to invalid executed quantity: %v", request.FormValue("trading_pair_symbol"), executedQuantityError)
		server.renderDashboardWithPurchaseError(responseWriter, request, http.StatusBadGateway, "Buy failed: Binance returned an invalid executed quantity.")
		return
	}

	purchaseUnitPrice := currentPricePerUnit
	cumulativeQuoteValue, cumulativeQuoteError := strconv.ParseFloat(buyOrderResponse.CumulativeQuote, 64)
	if cumulativeQuoteError == nil && executedQuantity > 0 {
		calculatedPrice := cumulativeQuoteValue / executedQuantity
		if calculatedPrice > 0 {
			purchaseUnitPrice = calculatedPrice
		}
	}

	targetSellPricePerUnit := purchaseUnitPrice * (1 + (targetProfitPercent / 100))

	sellExecutionContext, sellCancel := context.WithTimeout(request.Context(), 10*time.Second)
	defer sellCancel()

	sellOrderResponse, sellError := server.BinanceTradingService.PlaceLimitSell(sellExecutionContext, request.FormValue("trading_pair_symbol"), executedQuantity, targetSellPricePerUnit)
	if sellError != nil {
		server.logExecutionFailure(request.Context(), domain.TradingOperationTypeSell, request.FormValue("trading_pair_symbol"), sellError)
		log.Printf("Sell order placement failed for %s: %v", request.FormValue("trading_pair_symbol"), sellError)
	}

	buyOrderID := strconv.FormatInt(buyOrderResponse.OrderID, 10)
	var sellOrderID *string
	if sellOrderResponse != nil {
		sellOrderIdentifier := strconv.FormatInt(sellOrderResponse.OrderID, 10)
		sellOrderID = &sellOrderIdentifier
		server.logExecutionSuccess(request.Context(), domain.TradingOperationTypeSell, request.FormValue("trading_pair_symbol"), targetSellPricePerUnit, executedQuantity, sellOrderIdentifier)
	}

	operation := domain.TradingOperation{
		TradingPairSymbol:    request.FormValue("trading_pair_symbol"),
		QuantityPurchased:    executedQuantity,
		PurchasePricePerUnit: purchaseUnitPrice,
		TargetProfitPercent:  targetProfitPercent,
		BuyOrderIdentifier:   &buyOrderID,
		SellOrderIdentifier:  sellOrderID,
	}

	contextWithTimeout, cancel := context.WithTimeout(request.Context(), 5*time.Second)
	defer cancel()

	_, creationError := server.TradingOperationService.RecordPurchaseOperation(contextWithTimeout, operation)
	if creationError != nil {
		log.Printf("Trading operation creation failed: %v", creationError)
		http.Error(responseWriter, creationError.Error(), http.StatusBadRequest)
		return
	}

	server.logExecutionSuccess(request.Context(), domain.TradingOperationTypeBuy, operation.TradingPairSymbol, purchaseUnitPrice, executedQuantity, buyOrderID)

	if sellError != nil {
		server.renderDashboardWithPurchaseError(responseWriter, request, http.StatusBadGateway, "Sell order could not be created: "+sellError.Error())
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
		RecipientAddress:      request.FormValue("recipient_address"),
		TradingPairOrCurrency: request.FormValue("alert_trading_pair_symbol"),
	}

	thresholdValue, thresholdParseError := strconv.ParseFloat(request.FormValue("alert_threshold"), 64)
	if thresholdParseError != nil {
		http.Error(responseWriter, "Invalid threshold", http.StatusBadRequest)
		return
	}
	alert.ThresholdValue = thresholdValue

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

func (server *Server) handleListOperations(responseWriter http.ResponseWriter, request *http.Request) {
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

	tradingOperations, listError := server.TradingOperationService.ListOperations(contextWithTimeout, 100)
	if listError != nil {
		log.Printf("List transactions failed: %v", listError)
		http.Error(responseWriter, listError.Error(), http.StatusInternalServerError)
		return
	}

	renderError := server.Templates.ExecuteTemplate(responseWriter, "partials/transactions.html", tradingOperations)
	if renderError != nil {
		log.Printf("Partial render failed: %v", renderError)
		http.Error(responseWriter, "Could not render transactions", http.StatusInternalServerError)
		return
	}
}

func (server *Server) handleOperationHistory(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	operationPageNumber := parsePageNumber(request.URL.Query().Get("operations_page"))
	executionPageNumber := parsePageNumber(request.URL.Query().Get("executions_page"))
	pageSize := 25

	contextWithTimeout, cancel := context.WithTimeout(request.Context(), 8*time.Second)
	defer cancel()

	operations, operationsError := server.TradingOperationService.ListOperationsPage(contextWithTimeout, pageSize+1, operationPageNumber)
	if operationsError != nil {
		log.Printf("Could not fetch paginated operations: %v", operationsError)
		http.Error(responseWriter, "Could not fetch operation history", http.StatusInternalServerError)
		return
	}

	executions, executionsError := server.TradingScheduleService.ListExecutionsPage(contextWithTimeout, pageSize+1, executionPageNumber)
	if executionsError != nil {
		log.Printf("Could not fetch paginated executions: %v", executionsError)
		http.Error(responseWriter, "Could not fetch execution history", http.StatusInternalServerError)
		return
	}

	historyViewModel := server.buildOperationHistoryViewModel(operations, executions, operationPageNumber, executionPageNumber, pageSize)

	templateError := server.Templates.ExecuteTemplate(responseWriter, "operations_history.html", historyViewModel)
	if templateError != nil {
		log.Printf("Operation history template render error: %v", templateError)
		http.Error(responseWriter, "Could not render operation history", http.StatusInternalServerError)
		return
	}
}

func (server *Server) handleExecuteNextOperation(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if !server.CredentialService.HasValidBinanceCredentials() {
		responseWriter.WriteHeader(http.StatusServiceUnavailable)
		server.renderErrorPage(responseWriter, "Binance credentials are missing or invalid. Please provide a valid API Key and Secret Key to continue.")
		return
	}

	executionContext, executionCancel := context.WithTimeout(request.Context(), 15*time.Second)
	defer executionCancel()
	server.AutomationService.EvaluateAndSellProfitableOperations(executionContext)
	http.Redirect(responseWriter, request, "/", http.StatusSeeOther)
}

func (server *Server) handleUpdateBinanceCredentials(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	providedAPIKey := request.FormValue("binance_api_key")
	providedAPISecret := request.FormValue("binance_api_secret")
	selectedEnvironment := request.FormValue("binance_environment")

	validationContext, cancel := context.WithTimeout(request.Context(), 8*time.Second)
	defer cancel()

	validationError := server.CredentialService.ValidateAndPersistCredentials(validationContext, providedAPIKey, providedAPISecret, selectedEnvironment)
	if validationError != nil {
		log.Printf("Binance credential validation failed: %v", validationError)
		responseWriter.WriteHeader(http.StatusBadRequest)
		server.renderErrorPage(responseWriter, validationError.Error())
		return
	}

	server.refreshEnvironmentConfiguration()

	http.Redirect(responseWriter, request, "/", http.StatusSeeOther)
}

func (server *Server) handleUpdateBinanceEnvironment(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	requestedEnvironment := request.FormValue("binance_environment")
	activationContext, cancel := context.WithTimeout(request.Context(), 8*time.Second)
	defer cancel()

	activationError := server.CredentialService.ActivateEnvironment(activationContext, requestedEnvironment)
	if activationError != nil {
		responseWriter.WriteHeader(http.StatusBadRequest)
		server.renderErrorPage(responseWriter, activationError.Error())
		return
	}

	server.refreshEnvironmentConfiguration()
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
	activeEnvironment := server.CredentialService.GetActiveEnvironmentConfiguration()
	contextWithTimeout, cancel := context.WithTimeout(requestContext, 5*time.Second)
	defer cancel()

	tradingOperations, listError := server.TradingOperationService.ListOperations(contextWithTimeout, 100)
	if listError != nil {
		return nil, listError
	}

	executionHistory, executionError := server.TradingScheduleService.ListRecentExecutions(contextWithTimeout, 50)
	if executionError != nil {
		return nil, executionError
	}

	openOrders, openOrdersError := server.fetchOpenOrders(contextWithTimeout)
	var openOrdersErrorMessage string
	if openOrdersError != nil {
		openOrdersErrorMessage = openOrdersError.Error()
	}

	return &DashboardViewModel{
		TradingOperations:            tradingOperations,
		ExecutionHistory:             executionHistory,
		IsBinanceConfigured:          server.CredentialService.HasValidBinanceCredentials(),
		BinanceAPIKeyMasked:          server.CredentialService.GetMaskedBinanceAPIKey(),
		BinanceAPISecretMasked:       server.CredentialService.GetMaskedBinanceAPISecret(),
		AutomaticSellIntervalMinutes: server.SettingsSummary.AutomaticSellIntervalMinutes,
		DailyPurchaseIntervalMinutes: server.SettingsSummary.DailyPurchaseIntervalMinutes,
		BinanceAPIBaseURL:            activeEnvironment.RESTBaseURL,
		ApplicationBaseURL:           server.SettingsSummary.ApplicationBaseURL,
		TradingPairSymbol:            server.SettingsSummary.TradingPairSymbol,
		CapitalThreshold:             server.SettingsSummary.CapitalThreshold,
		TargetProfitPercent:          server.SettingsSummary.TargetProfitPercent,
		ActiveBinanceEnvironment:     activeEnvironment.EnvironmentName,
		OpenOrders:                   openOrders,
		OpenOrdersError:              openOrdersErrorMessage,
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
	TradingOperations            []domain.TradingOperation
	ExecutionHistory             []domain.TradingOperationExecution
	IsBinanceConfigured          bool
	BinanceAPIKeyMasked          string
	BinanceAPISecretMasked       string
	AutomaticSellIntervalMinutes int
	DailyPurchaseIntervalMinutes int
	BinanceAPIBaseURL            string
	ApplicationBaseURL           string
	TradingPairSymbol            string
	CapitalThreshold             float64
	TargetProfitPercent          float64
	ActiveBinanceEnvironment     string
	OpenOrders                   []service.BinanceOpenOrder
	OpenOrdersError              string
	PurchaseErrorMessage         string
}

type OperationHistoryViewModel struct {
	ActiveBinanceEnvironment string
	BinanceAPIBaseURL        string
	OperationPageNumber      int
	HasPreviousOperationPage bool
	HasNextOperationPage     bool
	ExecutionPageNumber      int
	HasPreviousExecutionPage bool
	HasNextExecutionPage     bool
	OpenOperations           []domain.TradingOperation
	CompletedOperations      []domain.TradingOperation
	ExecutionAttempts        []domain.TradingOperationExecution
}

type BinanceSymbolsResponse struct {
	Symbols []string `json:"symbols"`
}

func (server *Server) handleHealthCheck(responseWriter http.ResponseWriter, request *http.Request) {
	responseWriter.WriteHeader(http.StatusOK)
	responseWriter.Write([]byte("ok"))
}

func (server *Server) handleRevalidateBinanceCredentials(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	revalidationContext, cancel := context.WithTimeout(request.Context(), 8*time.Second)
	defer cancel()

	revalidationError := server.CredentialService.RevalidateStoredCredentials(revalidationContext)
	if revalidationError != nil {
		log.Printf("Binance credential revalidation failed: %v", revalidationError)
		http.Error(responseWriter, revalidationError.Error(), http.StatusBadRequest)
		return
	}

	server.refreshEnvironmentConfiguration()

	responseWriter.Header().Set("Content-Type", "application/json")
	json.NewEncoder(responseWriter).Encode(map[string]string{"message": "Credentials successfully revalidated."})
}

type DashboardSettingsSummary struct {
	AutomaticSellIntervalMinutes int
	DailyPurchaseIntervalMinutes int
	BinanceAPIBaseURL            string
	ActiveBinanceEnvironment     string
	ApplicationBaseURL           string
	TradingPairSymbol            string
	CapitalThreshold             float64
	TargetProfitPercent          float64
}

func (server *Server) fetchOpenOrders(requestContext context.Context) ([]service.BinanceOpenOrder, error) {
	if !server.CredentialService.HasValidBinanceCredentials() {
		return nil, errors.New("Binance credentials are missing or invalid. Cannot list open orders.")
	}

	openOrdersContext, openOrdersCancel := context.WithTimeout(requestContext, 10*time.Second)
	defer openOrdersCancel()

	return server.BinanceTradingService.ListOpenOrders(openOrdersContext, server.SettingsSummary.TradingPairSymbol)
}

func (server *Server) refreshEnvironmentConfiguration() {
	activeEnvironment := server.CredentialService.GetActiveEnvironmentConfiguration()
	server.BinancePriceService.UpdateEnvironmentConfiguration(activeEnvironment)
	server.BinanceSymbolService.UpdateEnvironmentConfiguration(activeEnvironment)
	server.BinanceTradingService.UpdateEnvironmentConfiguration(activeEnvironment)
	server.SettingsSummary.BinanceAPIBaseURL = activeEnvironment.RESTBaseURL
	server.SettingsSummary.ActiveBinanceEnvironment = activeEnvironment.EnvironmentName
}

func (server *Server) buildOperationHistoryViewModel(operations []domain.TradingOperation, executions []domain.TradingOperationExecution, operationPageNumber int, executionPageNumber int, pageSize int) OperationHistoryViewModel {
	hasNextOperationPage := false
	hasNextExecutionPage := false

	if len(operations) > pageSize {
		hasNextOperationPage = true
		operations = operations[:pageSize]
	}

	if len(executions) > pageSize {
		hasNextExecutionPage = true
		executions = executions[:pageSize]
	}

	openOperations := make([]domain.TradingOperation, 0)
	completedOperations := make([]domain.TradingOperation, 0)
	for _, tradingOperation := range operations {
		if tradingOperation.Status == domain.TradingOperationStatusSold {
			completedOperations = append(completedOperations, tradingOperation)
		} else {
			openOperations = append(openOperations, tradingOperation)
		}
	}

	activeEnvironment := server.CredentialService.GetActiveEnvironmentConfiguration()

	return OperationHistoryViewModel{
		ActiveBinanceEnvironment: activeEnvironment.EnvironmentName,
		BinanceAPIBaseURL:        activeEnvironment.RESTBaseURL,
		OperationPageNumber:      operationPageNumber,
		HasPreviousOperationPage: operationPageNumber > 1,
		HasNextOperationPage:     hasNextOperationPage,
		ExecutionPageNumber:      executionPageNumber,
		HasPreviousExecutionPage: executionPageNumber > 1,
		HasNextExecutionPage:     hasNextExecutionPage,
		OpenOperations:           openOperations,
		CompletedOperations:      completedOperations,
		ExecutionAttempts:        executions,
	}
}

func parsePageNumber(pageValue string) int {
	parsedPage, parseError := strconv.Atoi(pageValue)
	if parseError != nil || parsedPage < 1 {
		return 1
	}
	return parsedPage
}

func (server *Server) logExecutionFailure(requestContext context.Context, operationType string, tradingPairSymbol string, cause error) {
	executionContext, executionCancel := context.WithTimeout(requestContext, 5*time.Second)
	defer executionCancel()
	errorMessage := cause.Error()
	executionRecord := domain.TradingOperationExecution{
		TradingPairSymbol: tradingPairSymbol,
		OperationType:     operationType,
		ExecutedAt:        time.Now(),
		Success:           false,
		ErrorMessage:      &errorMessage,
	}
	_, logError := server.TradingScheduleService.LogExecution(executionContext, executionRecord)
	if logError != nil {
		log.Printf("Could not log failed execution: %v", logError)
	}
}

func (server *Server) logExecutionSuccess(requestContext context.Context, operationType string, tradingPairSymbol string, unitPrice float64, quantity float64, orderIdentifier string) {
	executionContext, executionCancel := context.WithTimeout(requestContext, 5*time.Second)
	defer executionCancel()
	executionRecord := domain.TradingOperationExecution{
		TradingPairSymbol: tradingPairSymbol,
		OperationType:     operationType,
		UnitPrice:         unitPrice,
		Quantity:          quantity,
		TotalValue:        unitPrice * quantity,
		ExecutedAt:        time.Now(),
		Success:           true,
		OrderIdentifier:   &orderIdentifier,
	}
	_, logError := server.TradingScheduleService.LogExecution(executionContext, executionRecord)
	if logError != nil {
		log.Printf("Could not log execution: %v", logError)
	}
}
