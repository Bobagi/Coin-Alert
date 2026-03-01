package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"coin-alert/internal/domain"
)

type BinancePriceService struct {
	EnvironmentConfiguration domain.BinanceEnvironmentConfiguration
	HTTPClient               *http.Client
}

type binanceTickerPriceResponse struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}

type BinanceKlinePoint struct {
	CloseTime  int64
	ClosePrice float64
}

func NewBinancePriceService(environmentConfiguration domain.BinanceEnvironmentConfiguration) *BinancePriceService {
	return &BinancePriceService{
		EnvironmentConfiguration: environmentConfiguration,
		HTTPClient:               &http.Client{Timeout: 8 * time.Second},
	}
}

func (service *BinancePriceService) UpdateEnvironmentConfiguration(newConfiguration domain.BinanceEnvironmentConfiguration) {
	service.EnvironmentConfiguration = newConfiguration
}

func (service *BinancePriceService) GetCurrentPrice(requestContext context.Context, tradingPairSymbol string) (float64, error) {
	tickerEndpoint, urlBuildError := url.Parse(service.EnvironmentConfiguration.RESTBaseURL)
	if urlBuildError != nil {
		return 0, urlBuildError
	}
	tickerEndpoint.Path = "/api/v3/ticker/price"

	queryParameters := tickerEndpoint.Query()
	queryParameters.Set("symbol", tradingPairSymbol)
	tickerEndpoint.RawQuery = queryParameters.Encode()

	tickerRequest, requestBuildError := http.NewRequestWithContext(requestContext, http.MethodGet, tickerEndpoint.String(), nil)
	if requestBuildError != nil {
		return 0, requestBuildError
	}

	tickerResponse, responseError := service.HTTPClient.Do(tickerRequest)
	if responseError != nil {
		return 0, responseError
	}
	defer tickerResponse.Body.Close()

	if tickerResponse.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("Binance price endpoint returned status %d", tickerResponse.StatusCode)
	}

	var parsedResponse binanceTickerPriceResponse
	decodeError := json.NewDecoder(tickerResponse.Body).Decode(&parsedResponse)
	if decodeError != nil {
		return 0, decodeError
	}

	if parsedResponse.Price == "" {
		return 0, errors.New("Binance price response did not include a price")
	}

	parsedPrice, priceParseError := parseDecimalStringToFloat(parsedResponse.Price)
	if priceParseError != nil {
		return 0, priceParseError
	}

	return parsedPrice, nil
}

func (service *BinancePriceService) GetKlineClosePrices(requestContext context.Context, tradingPairSymbol string, interval string, limit int) ([]BinanceKlinePoint, error) {
	if limit <= 0 {
		return nil, errors.New("limit must be greater than zero")
	}

	klinesEndpoint, urlBuildError := url.Parse(service.EnvironmentConfiguration.RESTBaseURL)
	if urlBuildError != nil {
		return nil, urlBuildError
	}
	klinesEndpoint.Path = "/api/v3/klines"

	queryParameters := klinesEndpoint.Query()
	queryParameters.Set("symbol", tradingPairSymbol)
	queryParameters.Set("interval", interval)
	queryParameters.Set("limit", strconv.Itoa(limit))
	klinesEndpoint.RawQuery = queryParameters.Encode()

	klinesRequest, requestBuildError := http.NewRequestWithContext(requestContext, http.MethodGet, klinesEndpoint.String(), nil)
	if requestBuildError != nil {
		return nil, requestBuildError
	}

	klinesResponse, responseError := service.HTTPClient.Do(klinesRequest)
	if responseError != nil {
		return nil, responseError
	}
	defer klinesResponse.Body.Close()

	if klinesResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Binance klines endpoint returned status %d", klinesResponse.StatusCode)
	}

	var parsedResponse [][]interface{}
	decodeError := json.NewDecoder(klinesResponse.Body).Decode(&parsedResponse)
	if decodeError != nil {
		return nil, decodeError
	}

	points := make([]BinanceKlinePoint, 0, len(parsedResponse))
	for _, entry := range parsedResponse {
		if len(entry) < 7 {
			continue
		}

		closeTimeValue, ok := entry[6].(float64)
		if !ok {
			continue
		}

		closePriceText, ok := entry[4].(string)
		if !ok || closePriceText == "" {
			continue
		}

		closePrice, parseError := parseDecimalStringToFloat(closePriceText)
		if parseError != nil {
			continue
		}

		points = append(points, BinanceKlinePoint{CloseTime: int64(closeTimeValue), ClosePrice: closePrice})
	}

	if len(points) == 0 {
		return nil, errors.New("no kline data returned")
	}

	return points, nil
}

func parseDecimalStringToFloat(decimalString string) (float64, error) {
	parsedValue, parseError := strconv.ParseFloat(decimalString, 64)
	if parseError != nil {
		return 0, fmt.Errorf("could not parse decimal value %s", decimalString)
	}
	return parsedValue, nil
}
