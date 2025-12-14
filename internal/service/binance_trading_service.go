package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"coin-alert/internal/domain"
)

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

func (service *BinanceTradingService) PlaceLimitSell(requestContext context.Context, tradingPairSymbol string, quantity float64, targetPrice float64) (*binanceOrderResponse, error) {
	requestParameters := url.Values{}
	requestParameters.Set("symbol", tradingPairSymbol)
	requestParameters.Set("side", "SELL")
	requestParameters.Set("type", "LIMIT")
	requestParameters.Set("timeInForce", "GTC")
	requestParameters.Set("quantity", formatDecimal(quantity))
	requestParameters.Set("price", formatDecimal(targetPrice))
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
