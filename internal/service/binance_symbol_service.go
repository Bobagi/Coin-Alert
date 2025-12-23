package service

import (
        "context"
        "encoding/json"
        "errors"
        "net/http"
        "strings"
        "time"
)

type BinanceSymbolService struct {
        SymbolEndpoint string
        HTTPClient     *http.Client
        CacheDuration  time.Duration
        cachedSymbols  []string
        lastFetchTime  time.Time
}

type binanceExchangeInfoResponse struct {
    Symbols []binanceSymbol `json:"symbols"`
}

type binanceSymbol struct {
    Symbol string `json:"symbol"`
    Status string `json:"status"`
}

func NewBinanceSymbolService(binanceAPIBaseURL string) *BinanceSymbolService {
        sanitizedBaseURL := strings.TrimRight(binanceAPIBaseURL, "/")
        if sanitizedBaseURL == "" {
                sanitizedBaseURL = "https://api.binance.com"
        }

        return &BinanceSymbolService{
                SymbolEndpoint: sanitizedBaseURL + "/api/v3/exchangeInfo",
                HTTPClient: &http.Client{
                        Timeout: 6 * time.Second,
                },
                CacheDuration: 10 * time.Minute,
                cachedSymbols: []string{},
                lastFetchTime: time.Time{},
        }
}

func (service *BinanceSymbolService) FetchAvailableSymbols(fetchContext context.Context) ([]string, error) {
        if service.cachedSymbolsAvailable() {
                return service.cachedSymbols, nil
        }

        request, creationError := http.NewRequestWithContext(fetchContext, http.MethodGet, service.SymbolEndpoint, nil)
        if creationError != nil {
                return nil, creationError
        }

        response, httpError := service.HTTPClient.Do(request)
        if httpError != nil {
                return nil, httpError
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

        filteredSymbols := service.extractTradableSymbols(exchangeInformation)
        service.cachedSymbols = filteredSymbols
        service.lastFetchTime = time.Now()

        return filteredSymbols, nil
}

func (service *BinanceSymbolService) cachedSymbolsAvailable() bool {
        if len(service.cachedSymbols) == 0 {
                return false
        }

        return time.Since(service.lastFetchTime) < service.CacheDuration
}

func (service *BinanceSymbolService) extractTradableSymbols(exchangeInformation binanceExchangeInfoResponse) []string {
        tradableSymbols := []string{}
        for _, symbolDetails := range exchangeInformation.Symbols {
                if symbolDetails.Status == "TRADING" {
                        tradableSymbols = append(tradableSymbols, symbolDetails.Symbol)
                }
        }
        return tradableSymbols
}
