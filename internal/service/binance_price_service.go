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

type BinanceKline struct {
	OpenTime   int64   `json:"openTime"`
	CloseTime  int64   `json:"closeTime"`
	ClosePrice float64 `json:"closePrice"`
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

func (service *BinancePriceService) GetKlines(requestContext context.Context, tradingPairSymbol string, interval string, limit int) ([]BinanceKline, error) {
	tickerEndpoint, urlBuildError := url.Parse(service.EnvironmentConfiguration.RESTBaseURL)
	if urlBuildError != nil {
		return nil, urlBuildError
	}
	tickerEndpoint.Path = "/api/v3/klines"

	queryParameters := tickerEndpoint.Query()
	queryParameters.Set("symbol", tradingPairSymbol)
	queryParameters.Set("interval", interval)
	queryParameters.Set("limit", strconv.Itoa(limit))
	tickerEndpoint.RawQuery = queryParameters.Encode()

	tickerRequest, requestBuildError := http.NewRequestWithContext(requestContext, http.MethodGet, tickerEndpoint.String(), nil)
	if requestBuildError != nil {
		return nil, requestBuildError
	}

	tickerResponse, responseError := service.HTTPClient.Do(tickerRequest)
	if responseError != nil {
		return nil, responseError
	}
	defer tickerResponse.Body.Close()

	if tickerResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Binance klines endpoint returned status %d", tickerResponse.StatusCode)
	}

	var parsedResponse [][]interface{}
	decodeError := json.NewDecoder(tickerResponse.Body).Decode(&parsedResponse)
	if decodeError != nil {
		return nil, decodeError
	}

	parsedKlines := make([]BinanceKline, 0, len(parsedResponse))
	for _, rawKline := range parsedResponse {
		if len(rawKline) < 7 {
			continue
		}
		openTimeValue, openTimeOk := rawKline[0].(float64)
		closePriceText, closePriceOk := rawKline[4].(string)
		closeTimeValue, closeTimeOk := rawKline[6].(float64)
		if !openTimeOk || !closePriceOk || !closeTimeOk {
			continue
		}

		closePrice, closePriceParseError := parseDecimalStringToFloat(closePriceText)
		if closePriceParseError != nil {
			continue
		}

		parsedKlines = append(parsedKlines, BinanceKline{
			OpenTime:   int64(openTimeValue),
			CloseTime:  int64(closeTimeValue),
			ClosePrice: closePrice,
		})
	}

	if len(parsedKlines) == 0 {
		return nil, errors.New("Binance klines response did not include valid data")
	}

	return parsedKlines, nil
}

func parseDecimalStringToFloat(decimalString string) (float64, error) {
        parsedValue, parseError := strconv.ParseFloat(decimalString, 64)
        if parseError != nil {
                return 0, fmt.Errorf("could not parse decimal value %s", decimalString)
        }
        return parsedValue, nil
}
