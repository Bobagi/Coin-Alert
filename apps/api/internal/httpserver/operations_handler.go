package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"coin-alert/internal/domain"
	"coin-alert/internal/repository"
	"coin-alert/internal/service"
)

// OperationsHandler serves the user-scoped trading endpoints (operations, executions, open orders).
type OperationsHandler struct {
	sessionService *service.SessionService
	cookieName     string
	tradingService *service.UserTradingService
}

func NewOperationsHandler(sessionService *service.SessionService, cookieName string, tradingService *service.UserTradingService) *OperationsHandler {
	return &OperationsHandler{
		sessionService: sessionService,
		cookieName:     cookieName,
		tradingService: tradingService,
	}
}

func (handler *OperationsHandler) RegisterRoutes(router *http.ServeMux) {
	router.HandleFunc("/api/v1/operations", handler.handleOperations)
	router.HandleFunc("/api/v1/operations/sell", handler.handleSellOperation)
	router.HandleFunc("/api/v1/operations/place-sell", handler.handlePlaceSell)
	router.HandleFunc("/api/v1/operations/executions", handler.handleExecutions)
	router.HandleFunc("/api/v1/binance/open-orders", handler.handleOpenOrders)
}

func (handler *OperationsHandler) requireUser(responseWriter http.ResponseWriter, request *http.Request) (int64, bool) {
	sessionCookie, cookieError := request.Cookie(handler.cookieName)
	if cookieError != nil {
		writeJSONError(responseWriter, http.StatusUnauthorized, "Not authenticated.")
		return 0, false
	}
	resolveContext, cancel := context.WithTimeout(request.Context(), 5*time.Second)
	defer cancel()
	userIdentifier, resolveError := handler.sessionService.ResolveUserIdentifier(resolveContext, sessionCookie.Value)
	if resolveError != nil {
		writeJSONError(responseWriter, http.StatusUnauthorized, "Not authenticated.")
		return 0, false
	}
	return userIdentifier, true
}

type buyRequestPayload struct {
	Symbol              string  `json:"symbol"`
	QuoteAmount         float64 `json:"quote_amount"`
	TargetProfitPercent float64 `json:"target_profit_percent"`
}

type operationPayload struct {
	ID                     int64      `json:"id"`
	Symbol                 string     `json:"symbol"`
	Quantity               float64    `json:"quantity"`
	PurchasePricePerUnit   float64    `json:"purchase_price_per_unit"`
	TargetProfitPercent    float64    `json:"target_profit_percent"`
	Status                 string     `json:"status"`
	SellPricePerUnit       *float64   `json:"sell_price_per_unit"`
	SellTargetPricePerUnit *float64   `json:"sell_target_price_per_unit"`
	BuyOrderID             *string    `json:"buy_order_id"`
	SellOrderID            *string    `json:"sell_order_id"`
	PurchasedAt            time.Time  `json:"purchased_at"`
	SoldAt                 *time.Time `json:"sold_at"`
}

type executionPayload struct {
	ID            int64     `json:"id"`
	Symbol        string    `json:"symbol"`
	OperationType string    `json:"operation_type"`
	UnitPrice     float64   `json:"unit_price"`
	Quantity      float64   `json:"quantity"`
	TotalValue    float64   `json:"total_value"`
	ExecutedAt    time.Time `json:"executed_at"`
	Success       bool      `json:"success"`
	ErrorMessage  *string   `json:"error_message"`
	OrderID       *string   `json:"order_id"`
}

func (handler *OperationsHandler) handleOperations(responseWriter http.ResponseWriter, request *http.Request) {
	userIdentifier, authenticated := handler.requireUser(responseWriter, request)
	if !authenticated {
		return
	}

	switch request.Method {
	case http.MethodGet:
		operationContext, cancel := context.WithTimeout(request.Context(), 6*time.Second)
		defer cancel()
		operations, listError := handler.tradingService.ListOperations(operationContext, userIdentifier, 200)
		if listError != nil {
			writeJSONError(responseWriter, http.StatusInternalServerError, "Could not load operations.")
			return
		}
		writeJSON(responseWriter, http.StatusOK, toOperationPayloads(operations))

	case http.MethodPost:
		var payload buyRequestPayload
		if decodeError := json.NewDecoder(request.Body).Decode(&payload); decodeError != nil {
			writeJSONError(responseWriter, http.StatusBadRequest, "Invalid request body.")
			return
		}
		operationContext, cancel := context.WithTimeout(request.Context(), 25*time.Second)
		defer cancel()
		operation, buyError := handler.tradingService.ExecuteBuy(operationContext, userIdentifier, payload.Symbol, payload.QuoteAmount, payload.TargetProfitPercent)
		if buyError != nil {
			writeJSONError(responseWriter, http.StatusBadRequest, buyError.Error())
			return
		}
		writeJSON(responseWriter, http.StatusOK, toOperationPayload(*operation))

	default:
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
	}
}

type sellRequestPayload struct {
	OperationID int64 `json:"operation_id"`
}

func (handler *OperationsHandler) handleSellOperation(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	userIdentifier, authenticated := handler.requireUser(responseWriter, request)
	if !authenticated {
		return
	}

	var payload sellRequestPayload
	if decodeError := json.NewDecoder(request.Body).Decode(&payload); decodeError != nil {
		writeJSONError(responseWriter, http.StatusBadRequest, "Invalid request body.")
		return
	}
	if payload.OperationID <= 0 {
		writeJSONError(responseWriter, http.StatusBadRequest, "An operation id is required.")
		return
	}

	operationContext, cancel := context.WithTimeout(request.Context(), 25*time.Second)
	defer cancel()
	operation, sellError := handler.tradingService.CloseOperationNow(operationContext, userIdentifier, payload.OperationID)
	if sellError != nil {
		if errors.Is(sellError, repository.ErrOperationNotFound) {
			writeJSONError(responseWriter, http.StatusNotFound, "Operation not found.")
			return
		}
		writeJSONError(responseWriter, http.StatusBadRequest, sellError.Error())
		return
	}
	writeJSON(responseWriter, http.StatusOK, toOperationPayload(*operation))
}

func (handler *OperationsHandler) handlePlaceSell(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	userIdentifier, authenticated := handler.requireUser(responseWriter, request)
	if !authenticated {
		return
	}

	var payload sellRequestPayload
	if decodeError := json.NewDecoder(request.Body).Decode(&payload); decodeError != nil {
		writeJSONError(responseWriter, http.StatusBadRequest, "Invalid request body.")
		return
	}
	if payload.OperationID <= 0 {
		writeJSONError(responseWriter, http.StatusBadRequest, "An operation id is required.")
		return
	}

	operationContext, cancel := context.WithTimeout(request.Context(), 25*time.Second)
	defer cancel()
	operation, placeError := handler.tradingService.PlaceTakeProfitForOperation(operationContext, userIdentifier, payload.OperationID)
	if placeError != nil {
		if errors.Is(placeError, repository.ErrOperationNotFound) {
			writeJSONError(responseWriter, http.StatusNotFound, "Operation not found.")
			return
		}
		writeJSONError(responseWriter, http.StatusBadRequest, placeError.Error())
		return
	}
	writeJSON(responseWriter, http.StatusOK, toOperationPayload(*operation))
}

func (handler *OperationsHandler) handleExecutions(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	userIdentifier, authenticated := handler.requireUser(responseWriter, request)
	if !authenticated {
		return
	}

	operationContext, cancel := context.WithTimeout(request.Context(), 6*time.Second)
	defer cancel()
	executions, listError := handler.tradingService.ListExecutions(operationContext, userIdentifier, 200)
	if listError != nil {
		writeJSONError(responseWriter, http.StatusInternalServerError, "Could not load executions.")
		return
	}
	writeJSON(responseWriter, http.StatusOK, toExecutionPayloads(executions))
}

func (handler *OperationsHandler) handleOpenOrders(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	userIdentifier, authenticated := handler.requireUser(responseWriter, request)
	if !authenticated {
		return
	}

	requestedSymbol := strings.ToUpper(strings.TrimSpace(request.URL.Query().Get("symbol")))
	operationContext, cancel := context.WithTimeout(request.Context(), 10*time.Second)
	defer cancel()
	openOrders, listError := handler.tradingService.ListOpenOrders(operationContext, userIdentifier, requestedSymbol)
	if listError != nil {
		writeJSONError(responseWriter, http.StatusBadGateway, listError.Error())
		return
	}
	if openOrders == nil {
		openOrders = make([]service.BinanceOpenOrder, 0)
	}
	writeJSON(responseWriter, http.StatusOK, openOrders)
}

func toOperationPayload(operation domain.TradingOperation) operationPayload {
	return operationPayload{
		ID:                     operation.Identifier,
		Symbol:                 operation.TradingPairSymbol,
		Quantity:               operation.QuantityPurchased,
		PurchasePricePerUnit:   operation.PurchasePricePerUnit,
		TargetProfitPercent:    operation.TargetProfitPercent,
		Status:                 operation.Status,
		SellPricePerUnit:       operation.SellPricePerUnit,
		SellTargetPricePerUnit: operation.SellTargetPricePerUnit,
		BuyOrderID:             operation.BuyOrderIdentifier,
		SellOrderID:            operation.SellOrderIdentifier,
		PurchasedAt:            operation.PurchaseTimestamp,
		SoldAt:                 operation.SellTimestamp,
	}
}

func toOperationPayloads(operations []domain.TradingOperation) []operationPayload {
	payloads := make([]operationPayload, 0, len(operations))
	for _, operation := range operations {
		payloads = append(payloads, toOperationPayload(operation))
	}
	return payloads
}

func toExecutionPayloads(executions []domain.TradingOperationExecution) []executionPayload {
	payloads := make([]executionPayload, 0, len(executions))
	for _, execution := range executions {
		payloads = append(payloads, executionPayload{
			ID:            execution.Identifier,
			Symbol:        execution.TradingPairSymbol,
			OperationType: execution.OperationType,
			UnitPrice:     execution.UnitPrice,
			Quantity:      execution.Quantity,
			TotalValue:    execution.TotalValue,
			ExecutedAt:    execution.ExecutedAt,
			Success:       execution.Success,
			ErrorMessage:  execution.ErrorMessage,
			OrderID:       execution.OrderIdentifier,
		})
	}
	return payloads
}
