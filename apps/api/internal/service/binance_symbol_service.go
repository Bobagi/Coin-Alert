package service

import (
        "context"
        "encoding/json"
        "errors"
        "net/http"
        "strings"
        "time"

        "coin-alert/internal/domain"
)

type BinanceSymbolService struct {
        EnvironmentConfiguration domain.BinanceEnvironmentConfiguration
        HTTPClient               *http.Client
        cachedSymbols            []string
        lastFetchTimestamp       time.Time
}

type binanceExchangeInfoResponse struct {
        Symbols []struct {
                Symbol                     string `json:"symbol"`
                Status                     string `json:"status"`
                IsSpotTradingAllowed       bool   `json:"isSpotTradingAllowed"`
                IsMarginTradingAllowed     bool   `json:"isMarginTradingAllowed"`
                QuoteAsset                 string `json:"quoteAsset"`
                BaseAsset                  string `json:"baseAsset"`
                QuoteAssetPrecision        int    `json:"quoteAssetPrecision"`
                BaseAssetPrecision         int    `json:"baseAssetPrecision"`
                QuotePrecision             int    `json:"quotePrecision"`
                OrderTypes                 []string `json:"orderTypes"`
                Permissions                []string `json:"permissions"`
        } `json:"symbols"`
}

func NewBinanceSymbolService(environmentConfiguration domain.BinanceEnvironmentConfiguration) *BinanceSymbolService {
        return &BinanceSymbolService{
                EnvironmentConfiguration: environmentConfiguration,
                HTTPClient:               &http.Client{Timeout: 8 * time.Second},
        }
}

func (service *BinanceSymbolService) UpdateEnvironmentConfiguration(newConfiguration domain.BinanceEnvironmentConfiguration) {
        service.EnvironmentConfiguration = newConfiguration
}

func (service *BinanceSymbolService) FetchAvailableSymbols(fetchContext context.Context) ([]string, error) {
        if service.cachedSymbolsAvailable() {
                return service.cachedSymbols, nil
        }

        exchangeInfoEndpoint := service.EnvironmentConfiguration.RESTBaseURL + "/api/v3/exchangeInfo"
        exchangeInfoRequest, requestError := http.NewRequestWithContext(fetchContext, http.MethodGet, exchangeInfoEndpoint, nil)
        if requestError != nil {
                return nil, requestError
        }

        response, responseError := service.HTTPClient.Do(exchangeInfoRequest)
        if responseError != nil {
                return nil, responseError
        }
        defer response.Body.Close()

        if response.StatusCode != http.StatusOK {
                return nil, errors.New("Binance symbols endpoint responded with a non-OK status")
        }

        var exchangeInformation binanceExchangeInfoResponse
        decodeError := json.NewDecoder(response.Body).Decode(&exchangeInformation)
        if decodeError != nil {
                return nil, decodeError
        }

        tradableSymbols := service.extractTradableSymbols(exchangeInformation)
        service.cachedSymbols = tradableSymbols
        service.lastFetchTimestamp = time.Now()

        return tradableSymbols, nil
}

func (service *BinanceSymbolService) cachedSymbolsAvailable() bool {
        return len(service.cachedSymbols) > 0 && time.Since(service.lastFetchTimestamp) < 10*time.Minute
}

func (service *BinanceSymbolService) extractTradableSymbols(exchangeInformation binanceExchangeInfoResponse) []string {
        var tradableSymbols []string
        for _, symbol := range exchangeInformation.Symbols {
                if strings.EqualFold(symbol.Status, "TRADING") && symbol.IsSpotTradingAllowed {
                        tradableSymbols = append(tradableSymbols, symbol.Symbol)
                }
        }
        return tradableSymbols
}
