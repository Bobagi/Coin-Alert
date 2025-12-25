package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"coin-alert/internal/domain"
)

const (
	HistoricalPricePeriodOneYear     = "1Y"
	HistoricalPricePeriodThreeMonths = "3M"
	HistoricalPricePeriodOneMonth    = "1M"
	HistoricalPricePeriodOneWeek     = "1W"
	HistoricalPricePeriodOneDay      = "1D"
)

type BinanceHistoricalPriceService struct {
	EnvironmentConfiguration domain.BinanceEnvironmentConfiguration
	HTTPClient               *http.Client
}

type BinanceHistoricalPricePoint struct {
	Timestamp time.Time `json:"timestamp"`
	Price     float64   `json:"price"`
}

type HistoricalPriceSnapshot struct {
	PricePoints  []BinanceHistoricalPricePoint
	MinimumPrice float64
	MaximumPrice float64
}

type historicalPriceRange struct {
	Interval string
	Start    time.Time
	End      time.Time
}

func NewBinanceHistoricalPriceService(environmentConfiguration domain.BinanceEnvironmentConfiguration) *BinanceHistoricalPriceService {
	return &BinanceHistoricalPriceService{
		EnvironmentConfiguration: environmentConfiguration,
		HTTPClient:               &http.Client{Timeout: 12 * time.Second},
	}
}

func (service *BinanceHistoricalPriceService) UpdateEnvironmentConfiguration(newConfiguration domain.BinanceEnvironmentConfiguration) {
	service.EnvironmentConfiguration = newConfiguration
}

func (service *BinanceHistoricalPriceService) GetHistoricalPriceSnapshot(requestContext context.Context, tradingPairSymbol string, period string) (HistoricalPriceSnapshot, error) {
	requestRange, rangeError := buildHistoricalPriceRange(period, time.Now().UTC())
	if rangeError != nil {
		return HistoricalPriceSnapshot{}, rangeError
	}

	klinesEndpoint, urlBuildError := url.Parse(service.EnvironmentConfiguration.RESTBaseURL)
	if urlBuildError != nil {
		return HistoricalPriceSnapshot{}, urlBuildError
	}
	klinesEndpoint.Path = "/api/v3/klines"

	queryParameters := klinesEndpoint.Query()
	queryParameters.Set("symbol", tradingPairSymbol)
	queryParameters.Set("interval", requestRange.Interval)
	queryParameters.Set("startTime", strconv.FormatInt(requestRange.Start.UnixMilli(), 10))
	queryParameters.Set("endTime", strconv.FormatInt(requestRange.End.UnixMilli(), 10))
	queryParameters.Set("limit", "1000")
	klinesEndpoint.RawQuery = queryParameters.Encode()

	klinesRequest, requestBuildError := http.NewRequestWithContext(requestContext, http.MethodGet, klinesEndpoint.String(), nil)
	if requestBuildError != nil {
		return HistoricalPriceSnapshot{}, requestBuildError
	}

	klinesResponse, responseError := service.HTTPClient.Do(klinesRequest)
	if responseError != nil {
		return HistoricalPriceSnapshot{}, responseError
	}
	defer klinesResponse.Body.Close()

	if klinesResponse.StatusCode != http.StatusOK {
		return HistoricalPriceSnapshot{}, fmt.Errorf("Binance klines endpoint returned status %d", klinesResponse.StatusCode)
	}

	var parsedResponse [][]json.RawMessage
	decodeError := json.NewDecoder(klinesResponse.Body).Decode(&parsedResponse)
	if decodeError != nil {
		return HistoricalPriceSnapshot{}, decodeError
	}
	if len(parsedResponse) == 0 {
		return HistoricalPriceSnapshot{}, errors.New("Binance klines response returned no data")
	}

	pricePoints := make([]BinanceHistoricalPricePoint, 0, len(parsedResponse))
	minimumPrice := math.MaxFloat64
	maximumPrice := 0.0

	for _, kline := range parsedResponse {
		pricePoint, parseError := parseKlineToPricePoint(kline)
		if parseError != nil {
			return HistoricalPriceSnapshot{}, parseError
		}
		pricePoints = append(pricePoints, pricePoint)
		if pricePoint.Price < minimumPrice {
			minimumPrice = pricePoint.Price
		}
		if pricePoint.Price > maximumPrice {
			maximumPrice = pricePoint.Price
		}
	}

	if minimumPrice == math.MaxFloat64 {
		minimumPrice = 0
	}

	return HistoricalPriceSnapshot{
		PricePoints:  pricePoints,
		MinimumPrice: minimumPrice,
		MaximumPrice: maximumPrice,
	}, nil
}

func buildHistoricalPriceRange(period string, referenceTime time.Time) (historicalPriceRange, error) {
	switch period {
	case HistoricalPricePeriodOneYear:
		return historicalPriceRange{Interval: "1d", Start: referenceTime.AddDate(-1, 0, 0), End: referenceTime}, nil
	case HistoricalPricePeriodThreeMonths:
		return historicalPriceRange{Interval: "1d", Start: referenceTime.AddDate(0, -3, 0), End: referenceTime}, nil
	case HistoricalPricePeriodOneMonth:
		return historicalPriceRange{Interval: "1d", Start: referenceTime.AddDate(0, -1, 0), End: referenceTime}, nil
	case HistoricalPricePeriodOneWeek:
		return historicalPriceRange{Interval: "1h", Start: referenceTime.AddDate(0, 0, -7), End: referenceTime}, nil
	case HistoricalPricePeriodOneDay:
		return historicalPriceRange{Interval: "5m", Start: referenceTime.Add(-24 * time.Hour), End: referenceTime}, nil
	default:
		return historicalPriceRange{}, fmt.Errorf("unsupported historical period: %s", period)
	}
}

func parseKlineToPricePoint(kline []json.RawMessage) (BinanceHistoricalPricePoint, error) {
	if len(kline) < 5 {
		return BinanceHistoricalPricePoint{}, errors.New("Binance kline response did not include enough data")
	}
	openTime, openTimeError := parseRawMessageToInt64(kline[0])
	if openTimeError != nil {
		return BinanceHistoricalPricePoint{}, openTimeError
	}
	closePrice, closePriceError := parseRawMessageToFloat(kline[4])
	if closePriceError != nil {
		return BinanceHistoricalPricePoint{}, closePriceError
	}
	return BinanceHistoricalPricePoint{
		Timestamp: time.UnixMilli(openTime),
		Price:     closePrice,
	}, nil
}

func parseRawMessageToInt64(rawMessage json.RawMessage) (int64, error) {
	var parsedValue int64
	parseError := json.Unmarshal(rawMessage, &parsedValue)
	if parseError != nil {
		return 0, parseError
	}
	return parsedValue, nil
}

func parseRawMessageToFloat(rawMessage json.RawMessage) (float64, error) {
	var parsedValue string
	parseError := json.Unmarshal(rawMessage, &parsedValue)
	if parseError != nil {
		return 0, parseError
	}
	return parseDecimalStringToFloat(parsedValue)
}
