package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"coin-alert/internal/domain"
)

type BinanceHistoricalPricePeriod string

const (
	BinanceHistoricalPricePeriodOneYear  BinanceHistoricalPricePeriod = "1Y"
	BinanceHistoricalPricePeriodThreeMonth BinanceHistoricalPricePeriod = "3M"
	BinanceHistoricalPricePeriodOneMonth BinanceHistoricalPricePeriod = "1M"
	BinanceHistoricalPricePeriodOneWeek  BinanceHistoricalPricePeriod = "1W"
	BinanceHistoricalPricePeriodOneDay   BinanceHistoricalPricePeriod = "1D"
)

type BinanceHistoricalPricePoint struct {
	Timestamp int64   `json:"timestamp"`
	Price     float64 `json:"price"`
}

type BinanceHistoricalPriceService struct {
	EnvironmentConfiguration domain.BinanceEnvironmentConfiguration
	HTTPClient               *http.Client
}

type binanceKlineEntry struct {
	OpenTime   int64
	ClosePrice string
}

func NewBinanceHistoricalPriceService(environmentConfiguration domain.BinanceEnvironmentConfiguration) *BinanceHistoricalPriceService {
	return &BinanceHistoricalPriceService{
		EnvironmentConfiguration: environmentConfiguration,
		HTTPClient:               &http.Client{Timeout: 10 * time.Second},
	}
}

func (service *BinanceHistoricalPriceService) UpdateEnvironmentConfiguration(newConfiguration domain.BinanceEnvironmentConfiguration) {
	service.EnvironmentConfiguration = newConfiguration
}

func (service *BinanceHistoricalPriceService) GetHistoricalPrices(requestContext context.Context, tradingPairSymbol string, periodValue string) ([]BinanceHistoricalPricePoint, BinanceHistoricalPricePeriod, error) {
	normalizedPeriod, periodError := ParseBinanceHistoricalPricePeriod(periodValue)
	if periodError != nil {
		return nil, "", periodError
	}

	intervalValue, periodDuration, requestLimit, intervalError := resolveKlineRequestConfiguration(normalizedPeriod)
	if intervalError != nil {
		return nil, "", intervalError
	}

	klinesEndpoint, urlBuildError := url.Parse(service.EnvironmentConfiguration.RESTBaseURL)
	if urlBuildError != nil {
		return nil, "", urlBuildError
	}
	klinesEndpoint.Path = "/api/v3/klines"

	endTime := time.Now()
	startTime := endTime.Add(-periodDuration)
	queryParameters := klinesEndpoint.Query()
	queryParameters.Set("symbol", tradingPairSymbol)
	queryParameters.Set("interval", intervalValue)
	queryParameters.Set("startTime", fmt.Sprintf("%d", startTime.UnixMilli()))
	queryParameters.Set("endTime", fmt.Sprintf("%d", endTime.UnixMilli()))
	queryParameters.Set("limit", fmt.Sprintf("%d", requestLimit))
	klinesEndpoint.RawQuery = queryParameters.Encode()

	klinesRequest, requestBuildError := http.NewRequestWithContext(requestContext, http.MethodGet, klinesEndpoint.String(), nil)
	if requestBuildError != nil {
		return nil, "", requestBuildError
	}

	klinesResponse, responseError := service.HTTPClient.Do(klinesRequest)
	if responseError != nil {
		return nil, "", responseError
	}
	defer klinesResponse.Body.Close()

	if klinesResponse.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("Binance klines endpoint returned status %d", klinesResponse.StatusCode)
	}

	var parsedResponse []binanceKlineEntry
	decodeError := json.NewDecoder(klinesResponse.Body).Decode(&parsedResponse)
	if decodeError != nil {
		return nil, "", decodeError
	}

	historicalPoints := make([]BinanceHistoricalPricePoint, 0, len(parsedResponse))
	for _, entry := range parsedResponse {
		parsedPrice, parseError := parseDecimalStringToFloat(entry.ClosePrice)
		if parseError != nil {
			return nil, "", parseError
		}
		historicalPoints = append(historicalPoints, BinanceHistoricalPricePoint{
			Timestamp: entry.OpenTime,
			Price:     parsedPrice,
		})
	}

	return historicalPoints, normalizedPeriod, nil
}

func ParseBinanceHistoricalPricePeriod(periodValue string) (BinanceHistoricalPricePeriod, error) {
	normalizedPeriod := strings.ToUpper(strings.TrimSpace(periodValue))
	switch BinanceHistoricalPricePeriod(normalizedPeriod) {
	case BinanceHistoricalPricePeriodOneYear,
		BinanceHistoricalPricePeriodThreeMonth,
		BinanceHistoricalPricePeriodOneMonth,
		BinanceHistoricalPricePeriodOneWeek,
		BinanceHistoricalPricePeriodOneDay:
		return BinanceHistoricalPricePeriod(normalizedPeriod), nil
	default:
		return "", fmt.Errorf("unsupported historical period %s", periodValue)
	}
}

func resolveKlineRequestConfiguration(period BinanceHistoricalPricePeriod) (string, time.Duration, int, error) {
	switch period {
	case BinanceHistoricalPricePeriodOneYear:
		return "1d", 365 * 24 * time.Hour, 400, nil
	case BinanceHistoricalPricePeriodThreeMonth:
		return "4h", 90 * 24 * time.Hour, 600, nil
	case BinanceHistoricalPricePeriodOneMonth:
		return "1h", 30 * 24 * time.Hour, 800, nil
	case BinanceHistoricalPricePeriodOneWeek:
		return "15m", 7 * 24 * time.Hour, 700, nil
	case BinanceHistoricalPricePeriodOneDay:
		return "5m", 24 * time.Hour, 500, nil
	default:
		return "", 0, 0, errors.New("unsupported historical period")
	}
}

func (entry *binanceKlineEntry) UnmarshalJSON(data []byte) error {
	var rawValues []json.RawMessage
	decodeError := json.Unmarshal(data, &rawValues)
	if decodeError != nil {
		return decodeError
	}
	if len(rawValues) < 5 {
		return errors.New("Binance kline response does not include expected columns")
	}

	var openTime int64
	openTimeError := json.Unmarshal(rawValues[0], &openTime)
	if openTimeError != nil {
		return openTimeError
	}

	var closePrice string
	closePriceError := json.Unmarshal(rawValues[4], &closePrice)
	if closePriceError != nil {
		return closePriceError
	}

	entry.OpenTime = openTime
	entry.ClosePrice = closePrice
	return nil
}
