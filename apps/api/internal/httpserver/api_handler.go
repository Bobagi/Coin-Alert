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

// APIHandler serves the user-scoped JSON API consumed by the SvelteKit frontend. Every endpoint
// resolves the current user from the session cookie before doing anything.
type APIHandler struct {
	sessionService            *service.SessionService
	cookieName                string
	tradingSettingsRepository repository.UserTradingSettingsRepository
	credentialService         *service.UserCredentialService
	testnetBaseURL            string
	productionBaseURL         string
}

func NewAPIHandler(sessionService *service.SessionService, cookieName string, tradingSettingsRepository repository.UserTradingSettingsRepository, credentialService *service.UserCredentialService, testnetBaseURL string, productionBaseURL string) *APIHandler {
	return &APIHandler{
		sessionService:            sessionService,
		cookieName:                cookieName,
		tradingSettingsRepository: tradingSettingsRepository,
		credentialService:         credentialService,
		testnetBaseURL:            testnetBaseURL,
		productionBaseURL:         productionBaseURL,
	}
}

func (handler *APIHandler) RegisterRoutes(router *http.ServeMux) {
	router.HandleFunc("/api/v1/settings", handler.handleSettings)
	router.HandleFunc("/api/v1/binance/credentials", handler.handleCredentials)
	router.HandleFunc("/api/v1/binance/credentials/activate", handler.handleActivateEnvironment)
	router.HandleFunc("/api/v1/binance/price", handler.handlePrice)
	router.HandleFunc("/api/v1/binance/symbols", handler.handleSymbols)
	router.HandleFunc("/api/v1/binance/symbol-filters", handler.handleSymbolFilters)
	router.HandleFunc("/api/v1/binance/klines", handler.handleKlines)
}

func (handler *APIHandler) requireUser(responseWriter http.ResponseWriter, request *http.Request) (int64, bool) {
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

type tradingSettingsPayload struct {
	TradingPairSymbol            string   `json:"trading_pair_symbol"`
	CapitalThreshold             float64  `json:"capital_threshold"`
	TargetProfitPercent          float64  `json:"target_profit_percent"`
	StopLossPercent              *float64 `json:"stop_loss_percent"`
	AutomaticSellIntervalMinutes int      `json:"auto_sell_interval_minutes"`
	DailyPurchaseHourUTC         int      `json:"daily_purchase_hour_utc"`
	DailyPurchaseEnabled         bool     `json:"daily_purchase_enabled"`
	SellOrderValidityDays        int      `json:"sell_order_validity_days"`
	LiveTradingEnabled           bool     `json:"live_trading_enabled"`
	ActiveBinanceEnvironment     string   `json:"active_binance_environment"`
}

func (handler *APIHandler) handleSettings(responseWriter http.ResponseWriter, request *http.Request) {
	userIdentifier, authenticated := handler.requireUser(responseWriter, request)
	if !authenticated {
		return
	}

	switch request.Method {
	case http.MethodGet:
		operationContext, cancel := context.WithTimeout(request.Context(), 5*time.Second)
		defer cancel()
		environmentName := handler.credentialService.ActiveEnvironmentName(operationContext, userIdentifier)
		settings, settingsError := handler.tradingSettingsRepository.EnsureDefaults(operationContext, userIdentifier, environmentName)
		if settingsError != nil || settings == nil {
			writeJSONError(responseWriter, http.StatusInternalServerError, "Could not load settings.")
			return
		}
		writeJSON(responseWriter, http.StatusOK, toTradingSettingsPayload(settings))

	case http.MethodPut:
		var payload tradingSettingsPayload
		if decodeError := json.NewDecoder(request.Body).Decode(&payload); decodeError != nil {
			writeJSONError(responseWriter, http.StatusBadRequest, "Invalid request body.")
			return
		}
		operationContext, cancel := context.WithTimeout(request.Context(), 5*time.Second)
		defer cancel()
		environmentName := handler.credentialService.ActiveEnvironmentName(operationContext, userIdentifier)
		updatedSettings := domain.UserTradingSettings{
			UserIdentifier:               userIdentifier,
			TradingPairSymbol:            normalizeSymbolOrDefault(payload.TradingPairSymbol),
			CapitalThreshold:             payload.CapitalThreshold,
			TargetProfitPercent:          payload.TargetProfitPercent,
			StopLossPercent:              payload.StopLossPercent,
			AutomaticSellIntervalMinutes: clampIntervalMinutes(payload.AutomaticSellIntervalMinutes),
			DailyPurchaseHourUTC:         clampHourOfDay(payload.DailyPurchaseHourUTC),
			DailyPurchaseEnabled:         payload.DailyPurchaseEnabled,
			SellOrderValidityDays:        clampValidityDays(payload.SellOrderValidityDays),
			LiveTradingEnabled:           payload.LiveTradingEnabled,
			ActiveBinanceEnvironment:     environmentName,
			BinanceEnvironment:           environmentName,
		}
		if upsertError := handler.tradingSettingsRepository.Upsert(operationContext, updatedSettings); upsertError != nil {
			writeJSONError(responseWriter, http.StatusInternalServerError, "Could not save settings.")
			return
		}
		writeJSON(responseWriter, http.StatusOK, toTradingSettingsPayload(&updatedSettings))

	default:
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
	}
}

type credentialStatusPayload struct {
	HasActiveCredential    bool     `json:"has_active_credential"`
	ActiveEnvironment      string   `json:"active_environment"`
	MaskedAPIKey           string   `json:"masked_api_key"`
	ConfiguredEnvironments []string `json:"configured_environments"`
}

type saveCredentialPayload struct {
	Environment string `json:"environment"`
	APIKey      string `json:"api_key"`
	APISecret   string `json:"api_secret"`
}

func (handler *APIHandler) handleCredentials(responseWriter http.ResponseWriter, request *http.Request) {
	userIdentifier, authenticated := handler.requireUser(responseWriter, request)
	if !authenticated {
		return
	}

	switch request.Method {
	case http.MethodGet:
		operationContext, cancel := context.WithTimeout(request.Context(), 5*time.Second)
		defer cancel()
		status, statusError := handler.credentialService.GetStatus(operationContext, userIdentifier)
		if statusError != nil {
			writeJSONError(responseWriter, http.StatusInternalServerError, "Could not load credential status.")
			return
		}
		writeJSON(responseWriter, http.StatusOK, credentialStatusPayload{
			HasActiveCredential:    status.HasActiveCredential,
			ActiveEnvironment:      status.ActiveEnvironment,
			MaskedAPIKey:           status.MaskedAPIKey,
			ConfiguredEnvironments: status.ConfiguredEnvironments,
		})

	case http.MethodPost:
		var payload saveCredentialPayload
		if decodeError := json.NewDecoder(request.Body).Decode(&payload); decodeError != nil {
			writeJSONError(responseWriter, http.StatusBadRequest, "Invalid request body.")
			return
		}
		if strings.TrimSpace(payload.APIKey) == "" || strings.TrimSpace(payload.APISecret) == "" {
			writeJSONError(responseWriter, http.StatusBadRequest, "API key and secret are required.")
			return
		}
		operationContext, cancel := context.WithTimeout(request.Context(), 12*time.Second)
		defer cancel()
		saveError := handler.credentialService.SaveAndValidate(operationContext, userIdentifier, payload.APIKey, payload.APISecret, payload.Environment)
		if saveError != nil {
			if errors.Is(saveError, service.ErrCredentialEncryptionUnavailable) {
				writeJSONError(responseWriter, http.StatusServiceUnavailable, "Server is not configured to store credentials securely yet.")
				return
			}
			writeJSONError(responseWriter, http.StatusBadRequest, "Binance rejected these credentials: "+saveError.Error())
			return
		}
		writeJSON(responseWriter, http.StatusOK, map[string]string{"message": "Credentials validated and saved."})

	default:
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (handler *APIHandler) handleActivateEnvironment(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	userIdentifier, authenticated := handler.requireUser(responseWriter, request)
	if !authenticated {
		return
	}

	var payload struct {
		Environment string `json:"environment"`
	}
	if decodeError := json.NewDecoder(request.Body).Decode(&payload); decodeError != nil {
		writeJSONError(responseWriter, http.StatusBadRequest, "Invalid request body.")
		return
	}

	operationContext, cancel := context.WithTimeout(request.Context(), 5*time.Second)
	defer cancel()
	if activationError := handler.credentialService.ActivateEnvironment(operationContext, userIdentifier, payload.Environment); activationError != nil {
		writeJSONError(responseWriter, http.StatusBadRequest, activationError.Error())
		return
	}
	writeJSON(responseWriter, http.StatusOK, map[string]string{"message": "Active environment updated."})
}

func (handler *APIHandler) handlePrice(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	userIdentifier, authenticated := handler.requireUser(responseWriter, request)
	if !authenticated {
		return
	}

	tradingPairSymbol := strings.ToUpper(strings.TrimSpace(request.URL.Query().Get("symbol")))
	if tradingPairSymbol == "" {
		writeJSONError(responseWriter, http.StatusBadRequest, "Missing symbol parameter.")
		return
	}

	priceService := service.NewBinancePriceService(handler.resolveEnvironmentConfiguration(request.Context(), userIdentifier))
	operationContext, cancel := context.WithTimeout(request.Context(), 6*time.Second)
	defer cancel()
	currentPrice, priceError := priceService.GetCurrentPrice(operationContext, tradingPairSymbol)
	if priceError != nil {
		writeJSONError(responseWriter, http.StatusBadGateway, "Could not fetch the current price.")
		return
	}
	writeJSON(responseWriter, http.StatusOK, map[string]interface{}{"symbol": tradingPairSymbol, "price": currentPrice})
}

func (handler *APIHandler) handleSymbols(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	userIdentifier, authenticated := handler.requireUser(responseWriter, request)
	if !authenticated {
		return
	}

	symbolService := service.NewBinanceSymbolService(handler.resolveEnvironmentConfiguration(request.Context(), userIdentifier))
	operationContext, cancel := context.WithTimeout(request.Context(), 8*time.Second)
	defer cancel()
	availableSymbols, fetchError := symbolService.FetchAvailableSymbols(operationContext)
	if fetchError != nil {
		writeJSONError(responseWriter, http.StatusBadGateway, "Could not fetch tradable symbols.")
		return
	}
	writeJSON(responseWriter, http.StatusOK, map[string]interface{}{"symbols": availableSymbols})
}

// handleSymbolFilters returns the pair's trading rules (minimum order value, price/quantity steps) so
// the UI can show them and validate an order before placing it.
func (handler *APIHandler) handleSymbolFilters(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	userIdentifier, authenticated := handler.requireUser(responseWriter, request)
	if !authenticated {
		return
	}

	tradingPairSymbol := strings.ToUpper(strings.TrimSpace(request.URL.Query().Get("symbol")))
	if tradingPairSymbol == "" {
		writeJSONError(responseWriter, http.StatusBadRequest, "Missing symbol parameter.")
		return
	}

	tradingService := service.NewBinanceTradingService(handler.resolveEnvironmentConfiguration(request.Context(), userIdentifier))
	operationContext, cancel := context.WithTimeout(request.Context(), 8*time.Second)
	defer cancel()
	filters, filtersError := tradingService.FetchSymbolFilters(operationContext, tradingPairSymbol)
	if filtersError != nil {
		writeJSONError(responseWriter, http.StatusBadGateway, "Could not load the trading rules for this pair.")
		return
	}
	writeJSON(responseWriter, http.StatusOK, map[string]interface{}{
		"symbol":       tradingPairSymbol,
		"min_notional": filters.MinNotional,
		"tick_size":    filters.TickSize,
		"step_size":    filters.StepSize,
	})
}

// handleKlines returns the close-price series for a pair over a named period, used to draw the
// allocation history chart.
func (handler *APIHandler) handleKlines(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	userIdentifier, authenticated := handler.requireUser(responseWriter, request)
	if !authenticated {
		return
	}

	tradingPairSymbol := strings.ToUpper(strings.TrimSpace(request.URL.Query().Get("symbol")))
	if tradingPairSymbol == "" {
		writeJSONError(responseWriter, http.StatusBadRequest, "Missing symbol parameter.")
		return
	}
	period := strings.TrimSpace(request.URL.Query().Get("period"))
	interval, limit := klineParametersForPeriod(period)

	priceService := service.NewBinancePriceService(handler.resolveEnvironmentConfiguration(request.Context(), userIdentifier))
	operationContext, cancel := context.WithTimeout(request.Context(), 8*time.Second)
	defer cancel()
	points, seriesError := priceService.FetchCloseSeries(operationContext, tradingPairSymbol, interval, limit)
	if seriesError != nil {
		writeJSONError(responseWriter, http.StatusBadGateway, "Could not load price history for this pair.")
		return
	}
	writeJSON(responseWriter, http.StatusOK, map[string]interface{}{
		"symbol": tradingPairSymbol,
		"period": period,
		"points": points,
	})
}

// klineParametersForPeriod maps a UI period to a Binance kline interval + point count.
func klineParametersForPeriod(period string) (string, int) {
	switch period {
	case "7d":
		return "4h", 42
	case "1M":
		return "1d", 30
	case "3M":
		return "1d", 90
	default: // 24h
		return "1h", 24
	}
}

// resolveEnvironmentConfiguration returns the user's active environment (for the correct base URL),
// falling back to the public testnet endpoint when the user has no credentials yet.
func (handler *APIHandler) resolveEnvironmentConfiguration(parentContext context.Context, userIdentifier int64) domain.BinanceEnvironmentConfiguration {
	lookupContext, cancel := context.WithTimeout(parentContext, 5*time.Second)
	defer cancel()
	activeConfiguration, loadError := handler.credentialService.LoadActiveEnvironmentConfiguration(lookupContext, userIdentifier)
	if loadError == nil && activeConfiguration != nil {
		return *activeConfiguration
	}
	return domain.BinanceEnvironmentConfiguration{
		EnvironmentName: domain.BinanceEnvironmentTestnet,
		RESTBaseURL:     handler.testnetBaseURL,
	}
}

func toTradingSettingsPayload(settings *domain.UserTradingSettings) tradingSettingsPayload {
	return tradingSettingsPayload{
		TradingPairSymbol:            settings.TradingPairSymbol,
		CapitalThreshold:             settings.CapitalThreshold,
		TargetProfitPercent:          settings.TargetProfitPercent,
		StopLossPercent:              settings.StopLossPercent,
		AutomaticSellIntervalMinutes: settings.AutomaticSellIntervalMinutes,
		DailyPurchaseHourUTC:         settings.DailyPurchaseHourUTC,
		DailyPurchaseEnabled:         settings.DailyPurchaseEnabled,
		SellOrderValidityDays:        settings.SellOrderValidityDays,
		LiveTradingEnabled:           settings.LiveTradingEnabled,
		ActiveBinanceEnvironment:     settings.ActiveBinanceEnvironment,
	}
}

func normalizeSymbolOrDefault(tradingPairSymbol string) string {
	normalized := strings.ToUpper(strings.TrimSpace(tradingPairSymbol))
	if normalized == "" {
		return "BTCUSDT"
	}
	return normalized
}

func clampIntervalMinutes(minutes int) int {
	if minutes < 1 {
		return 60
	}
	return minutes
}

func clampHourOfDay(hour int) int {
	if hour < 0 || hour > 23 {
		return 4
	}
	return hour
}

func clampValidityDays(days int) int {
	if days < 0 {
		return 0
	}
	if days > 365 {
		return 365
	}
	return days
}
