package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"coin-alert/internal/domain"
)

// SymbolFilters holds the Binance trading rules we must honor when placing limit orders. A limit
// price must be a multiple of TickSize and a quantity a multiple of StepSize, otherwise Binance
// rejects the order with -1013 (PRICE_FILTER / LOT_SIZE).
type SymbolFilters struct {
	TickSize         float64
	StepSize         float64
	MinNotional      float64 // minimum order value (price * quantity), from the NOTIONAL filter
	PriceDecimals    int
	QuantityDecimals int
}

type BinanceTradingService struct {
	EnvironmentConfiguration domain.BinanceEnvironmentConfiguration
	HTTPClient               *http.Client
}

type binanceOrderResponse struct {
	OrderID         int64  `json:"orderId"`
	Symbol          string `json:"symbol"`
	ExecutedQty     string `json:"executedQty"`
	Price           string `json:"price"`
	Status          string `json:"status"`
	ClientOrderID   string `json:"clientOrderId"`
	TransactTime    int64  `json:"transactTime"`
	CumulativeQuote string `json:"cummulativeQuoteQty"`
}

type BinanceOpenOrder struct {
        OrderID int64  `json:"orderId"`
        Symbol  string `json:"symbol"`
        Price   string `json:"price"`
        Side    string `json:"side"`
        Status  string `json:"status"`
}

type BinanceOrderStatus struct {
        OrderID         int64  `json:"orderId"`
        Symbol          string `json:"symbol"`
        Status          string `json:"status"`
        ExecutedQty     string `json:"executedQty"`
        Price           string `json:"price"`
        CumulativeQuote string `json:"cummulativeQuoteQty"`
}

func NewBinanceTradingService(environmentConfiguration domain.BinanceEnvironmentConfiguration) *BinanceTradingService {
	return &BinanceTradingService{
		EnvironmentConfiguration: environmentConfiguration,
		HTTPClient:               &http.Client{Timeout: 10 * time.Second},
	}
}

func (service *BinanceTradingService) UpdateEnvironmentConfiguration(newConfiguration domain.BinanceEnvironmentConfiguration) {
	service.EnvironmentConfiguration = newConfiguration
}

func (service *BinanceTradingService) PlaceMarketBuyByQuote(requestContext context.Context, tradingPairSymbol string, quoteAmount float64) (*binanceOrderResponse, error) {
	requestParameters := url.Values{}
	requestParameters.Set("symbol", tradingPairSymbol)
	requestParameters.Set("side", "BUY")
	requestParameters.Set("type", "MARKET")
	requestParameters.Set("quoteOrderQty", formatDecimal(quoteAmount))
	requestParameters.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))

	signedEndpoint, signingError := service.buildSignedEndpoint("/api/v3/order", requestParameters)
	if signingError != nil {
		return nil, signingError
	}

	orderRequest, requestBuildError := http.NewRequestWithContext(requestContext, http.MethodPost, signedEndpoint, nil)
	if requestBuildError != nil {
		return nil, requestBuildError
	}
	orderRequest.Header.Set("X-MBX-APIKEY", service.EnvironmentConfiguration.APIKey)

	orderResponse, responseError := service.HTTPClient.Do(orderRequest)
	if responseError != nil {
		return nil, responseError
	}
	defer orderResponse.Body.Close()

	if orderResponse.StatusCode != http.StatusOK {
		responseBody, responseReadError := io.ReadAll(orderResponse.Body)
		if responseReadError != nil {
			return nil, fmt.Errorf("Binance rejected buy order (status %d) and the response could not be read", orderResponse.StatusCode)
		}
		return nil, fmt.Errorf("Binance rejected buy order (status %d): %s", orderResponse.StatusCode, string(responseBody))
	}

	var parsedResponse binanceOrderResponse
	decodeError := json.NewDecoder(orderResponse.Body).Decode(&parsedResponse)
	if decodeError != nil {
		return nil, decodeError
	}

	if parsedResponse.OrderID == 0 {
		return nil, fmt.Errorf("Binance did not return an orderId for the buy request")
	}

	return &parsedResponse, nil
}

func (service *BinanceTradingService) PlaceLimitSell(requestContext context.Context, tradingPairSymbol string, quantity float64, targetPrice float64, filters SymbolFilters) (*binanceOrderResponse, error) {
	// Snap the price/quantity to the symbol's tick/step so Binance accepts the order. When filters
	// are unavailable (fetch failed) we fall back to raw formatting rather than mis-round to integers.
	roundedPrice := targetPrice
	priceText := formatDecimal(targetPrice)
	if filters.TickSize > 0 {
		roundedPrice = roundToIncrement(targetPrice, filters.TickSize)
		priceText = formatWithDecimals(roundedPrice, filters.PriceDecimals)
	}
	roundedQuantity := quantity
	quantityText := formatDecimal(quantity)
	if filters.StepSize > 0 {
		roundedQuantity = floorToIncrement(quantity, filters.StepSize)
		quantityText = formatWithDecimals(roundedQuantity, filters.QuantityDecimals)
	}

	// A limit order's value must meet the symbol's NOTIONAL minimum, or Binance rejects it (-1013).
	if filters.MinNotional > 0 && roundedPrice*roundedQuantity < filters.MinNotional {
		return nil, fmt.Errorf("this position is too small for a sell order: its value %s is below Binance's minimum order value (NOTIONAL %s) for %s",
			formatDecimal(roundedPrice*roundedQuantity), formatDecimal(filters.MinNotional), tradingPairSymbol)
	}

	requestParameters := url.Values{}
	requestParameters.Set("symbol", tradingPairSymbol)
	requestParameters.Set("side", "SELL")
	requestParameters.Set("type", "LIMIT")
	requestParameters.Set("timeInForce", "GTC")
	requestParameters.Set("quantity", quantityText)
	requestParameters.Set("price", priceText)
	requestParameters.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))

	signedEndpoint, signingError := service.buildSignedEndpoint("/api/v3/order", requestParameters)
	if signingError != nil {
		return nil, signingError
	}

	orderRequest, requestBuildError := http.NewRequestWithContext(requestContext, http.MethodPost, signedEndpoint, nil)
	if requestBuildError != nil {
		return nil, requestBuildError
	}
	orderRequest.Header.Set("X-MBX-APIKEY", service.EnvironmentConfiguration.APIKey)

	orderResponse, responseError := service.HTTPClient.Do(orderRequest)
	if responseError != nil {
		return nil, responseError
	}
	defer orderResponse.Body.Close()

	if orderResponse.StatusCode != http.StatusOK {
		responseBody, responseReadError := io.ReadAll(orderResponse.Body)
		if responseReadError != nil {
			return nil, fmt.Errorf("Binance rejected sell order (status %d) and the response could not be read", orderResponse.StatusCode)
		}
		return nil, fmt.Errorf("Binance rejected sell order (status %d): %s", orderResponse.StatusCode, string(responseBody))
	}

	var parsedResponse binanceOrderResponse
	decodeError := json.NewDecoder(orderResponse.Body).Decode(&parsedResponse)
	if decodeError != nil {
		return nil, decodeError
	}

	if parsedResponse.OrderID == 0 {
		return nil, fmt.Errorf("Binance did not return an orderId for the sell request")
	}

	return &parsedResponse, nil
}

func (service *BinanceTradingService) ListOpenOrders(requestContext context.Context, tradingPairSymbol string) ([]BinanceOpenOrder, error) {
	requestParameters := url.Values{}
	requestParameters.Set("symbol", tradingPairSymbol)
	requestParameters.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))

	signedEndpoint, signingError := service.buildSignedEndpoint("/api/v3/openOrders", requestParameters)
	if signingError != nil {
		return nil, signingError
	}

	request, requestBuildError := http.NewRequestWithContext(requestContext, http.MethodGet, signedEndpoint, nil)
	if requestBuildError != nil {
		return nil, requestBuildError
	}
	request.Header.Set("X-MBX-APIKEY", service.EnvironmentConfiguration.APIKey)

	response, responseError := service.HTTPClient.Do(request)
	if responseError != nil {
		return nil, responseError
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Binance rejected open orders request (status %d)", response.StatusCode)
	}

	var parsedResponse []BinanceOpenOrder
	decodeError := json.NewDecoder(response.Body).Decode(&parsedResponse)
	if decodeError != nil {
		return nil, decodeError
	}

        return parsedResponse, nil
}

func (service *BinanceTradingService) GetOrderStatus(requestContext context.Context, tradingPairSymbol string, orderIdentifier string) (*BinanceOrderStatus, error) {
        requestParameters := url.Values{}
        requestParameters.Set("symbol", tradingPairSymbol)
        requestParameters.Set("orderId", orderIdentifier)
        requestParameters.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))

        signedEndpoint, signingError := service.buildSignedEndpoint("/api/v3/order", requestParameters)
        if signingError != nil {
                return nil, signingError
        }

        orderRequest, requestBuildError := http.NewRequestWithContext(requestContext, http.MethodGet, signedEndpoint, nil)
        if requestBuildError != nil {
                return nil, requestBuildError
        }
        orderRequest.Header.Set("X-MBX-APIKEY", service.EnvironmentConfiguration.APIKey)

        orderResponse, responseError := service.HTTPClient.Do(orderRequest)
        if responseError != nil {
                return nil, responseError
        }
        defer orderResponse.Body.Close()

        if orderResponse.StatusCode != http.StatusOK {
            return nil, fmt.Errorf("Binance rejected order status request (status %d)", orderResponse.StatusCode)
        }

        var parsedResponse BinanceOrderStatus
        decodeError := json.NewDecoder(orderResponse.Body).Decode(&parsedResponse)
        if decodeError != nil {
                return nil, decodeError
        }

        if parsedResponse.OrderID == 0 {
                return nil, fmt.Errorf("Binance did not return an orderId for the order status request")
        }

        return &parsedResponse, nil
}

func (service *BinanceTradingService) buildSignedEndpoint(path string, parameters url.Values) (string, error) {
	apiBaseURL, parseError := url.Parse(service.EnvironmentConfiguration.RESTBaseURL)
	if parseError != nil {
		return "", parseError
	}
	apiBaseURL.Path = path

	signature := signQuery(parameters.Encode(), service.EnvironmentConfiguration.APISecret)
	parameters.Set("signature", signature)
	apiBaseURL.RawQuery = parameters.Encode()

	return apiBaseURL.String(), nil
}

func signQuery(message string, secret string) string {
	hash := hmac.New(sha256.New, []byte(secret))
	hash.Write([]byte(message))
	return hex.EncodeToString(hash.Sum(nil))
}

func formatDecimal(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// FetchSymbolFilters reads the PRICE_FILTER tickSize and LOT_SIZE stepSize for one symbol from
// exchangeInfo. A zero-value result (e.g. on error) makes callers fall back to raw formatting.
func (service *BinanceTradingService) FetchSymbolFilters(requestContext context.Context, tradingPairSymbol string) (SymbolFilters, error) {
	endpoint := service.EnvironmentConfiguration.RESTBaseURL + "/api/v3/exchangeInfo?symbol=" + url.QueryEscape(tradingPairSymbol)
	request, requestError := http.NewRequestWithContext(requestContext, http.MethodGet, endpoint, nil)
	if requestError != nil {
		return SymbolFilters{}, requestError
	}

	response, responseError := service.HTTPClient.Do(request)
	if responseError != nil {
		return SymbolFilters{}, responseError
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return SymbolFilters{}, fmt.Errorf("Binance exchangeInfo responded with status %d", response.StatusCode)
	}

	var payload struct {
		Symbols []struct {
			Filters []struct {
				FilterType  string `json:"filterType"`
				TickSize    string `json:"tickSize"`
				StepSize    string `json:"stepSize"`
				MinNotional string `json:"minNotional"`
			} `json:"filters"`
		} `json:"symbols"`
	}
	if decodeError := json.NewDecoder(response.Body).Decode(&payload); decodeError != nil {
		return SymbolFilters{}, decodeError
	}
	if len(payload.Symbols) == 0 {
		return SymbolFilters{}, fmt.Errorf("Binance returned no filters for %s", tradingPairSymbol)
	}

	filters := SymbolFilters{}
	for _, filter := range payload.Symbols[0].Filters {
		switch filter.FilterType {
		case "PRICE_FILTER":
			filters.TickSize, _ = strconv.ParseFloat(filter.TickSize, 64)
			filters.PriceDecimals = decimalPlaces(filter.TickSize)
		case "LOT_SIZE":
			filters.StepSize, _ = strconv.ParseFloat(filter.StepSize, 64)
			filters.QuantityDecimals = decimalPlaces(filter.StepSize)
		case "NOTIONAL", "MIN_NOTIONAL":
			filters.MinNotional, _ = strconv.ParseFloat(filter.MinNotional, 64)
		}
	}
	return filters, nil
}

func roundToIncrement(value float64, increment float64) float64 {
	if increment <= 0 {
		return value
	}
	return math.Round(value/increment) * increment
}

func floorToIncrement(value float64, increment float64) float64 {
	if increment <= 0 {
		return value
	}
	return math.Floor(value/increment) * increment
}

func formatWithDecimals(value float64, decimals int) string {
	return strconv.FormatFloat(value, 'f', decimals, 64)
}

// decimalPlaces returns the number of significant decimal places in a Binance increment string such
// as "0.01000000" (=> 2) or "0.00001000" (=> 5).
func decimalPlaces(numberText string) int {
	numberText = strings.TrimRight(strings.TrimSpace(numberText), "0")
	dotIndex := strings.IndexByte(numberText, '.')
	if dotIndex < 0 {
		return 0
	}
	return len(numberText) - dotIndex - 1
}
