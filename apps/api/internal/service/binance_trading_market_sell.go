package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// CancelOrder cancels a resting order (used to free the balance before a stop-loss market sell).
func (service *BinanceTradingService) CancelOrder(requestContext context.Context, tradingPairSymbol string, orderIdentifier string) error {
	requestParameters := url.Values{}
	requestParameters.Set("symbol", tradingPairSymbol)
	requestParameters.Set("orderId", orderIdentifier)
	requestParameters.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))

	signedEndpoint, signingError := service.buildSignedEndpoint("/api/v3/order", requestParameters)
	if signingError != nil {
		return signingError
	}

	cancelRequest, buildError := http.NewRequestWithContext(requestContext, http.MethodDelete, signedEndpoint, nil)
	if buildError != nil {
		return buildError
	}
	cancelRequest.Header.Set("X-MBX-APIKEY", service.EnvironmentConfiguration.APIKey)

	cancelResponse, responseError := service.HTTPClient.Do(cancelRequest)
	if responseError != nil {
		return responseError
	}
	defer cancelResponse.Body.Close()

	if cancelResponse.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(cancelResponse.Body)
		return fmt.Errorf("Binance rejected cancel for order %s (status %d): %s", orderIdentifier, cancelResponse.StatusCode, string(responseBody))
	}
	return nil
}

// PlaceMarketSellByQuantity immediately sells a quantity at market price (used for stop-loss).
func (service *BinanceTradingService) PlaceMarketSellByQuantity(requestContext context.Context, tradingPairSymbol string, quantity float64) (*binanceOrderResponse, error) {
	requestParameters := url.Values{}
	requestParameters.Set("symbol", tradingPairSymbol)
	requestParameters.Set("side", "SELL")
	requestParameters.Set("type", "MARKET")
	requestParameters.Set("quantity", formatDecimal(quantity))
	requestParameters.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))

	signedEndpoint, signingError := service.buildSignedEndpoint("/api/v3/order", requestParameters)
	if signingError != nil {
		return nil, signingError
	}

	orderRequest, buildError := http.NewRequestWithContext(requestContext, http.MethodPost, signedEndpoint, nil)
	if buildError != nil {
		return nil, buildError
	}
	orderRequest.Header.Set("X-MBX-APIKEY", service.EnvironmentConfiguration.APIKey)

	orderResponse, responseError := service.HTTPClient.Do(orderRequest)
	if responseError != nil {
		return nil, responseError
	}
	defer orderResponse.Body.Close()

	if orderResponse.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(orderResponse.Body)
		return nil, fmt.Errorf("Binance rejected market sell (status %d): %s", orderResponse.StatusCode, string(responseBody))
	}

	var parsedResponse binanceOrderResponse
	if decodeError := json.NewDecoder(orderResponse.Body).Decode(&parsedResponse); decodeError != nil {
		return nil, decodeError
	}
	return &parsedResponse, nil
}
