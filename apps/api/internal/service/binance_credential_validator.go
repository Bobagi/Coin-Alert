package service

import (
        "context"
        "crypto/hmac"
        "crypto/sha256"
        "encoding/hex"
        "encoding/json"
        "errors"
        "fmt"
        "io"
        "net/http"
        "net/url"
        "strings"
        "time"
)

type BinanceCredentialValidator struct {
        APIBaseURL string
        HTTPClient *http.Client
}

func NewBinanceCredentialValidator(apiBaseURL string) *BinanceCredentialValidator {
        return &BinanceCredentialValidator{APIBaseURL: apiBaseURL, HTTPClient: &http.Client{Timeout: 8 * time.Second}}
}

func (validator *BinanceCredentialValidator) UpdateAPIBaseURL(newBaseURL string) {
        validator.APIBaseURL = newBaseURL
}

func (validator *BinanceCredentialValidator) ValidateCredentials(validationContext context.Context, apiKey string, apiSecret string) error {
        if validator == nil {
                return errors.New("Binance credential validator is not configured")
        }

        if strings.TrimSpace(apiKey) == "" || strings.TrimSpace(apiSecret) == "" {
                return errors.New("Binance API Key and Secret Key are required")
        }

        serverTimestamp, serverTimeError := validator.fetchBinanceServerTimestamp(validationContext)
        if serverTimeError != nil {
                return serverTimeError
        }

        signedRequest, signingError := validator.buildSignedAccountRequest(validationContext, apiKey, apiSecret, serverTimestamp)
        if signingError != nil {
                return signingError
        }

        binanceResponse, responseError := validator.HTTPClient.Do(signedRequest)
        if responseError != nil {
                return responseError
        }
        defer binanceResponse.Body.Close()

        if binanceResponse.StatusCode != http.StatusOK {
                responseBody, _ := io.ReadAll(binanceResponse.Body)
                if len(responseBody) == 0 {
                        return fmt.Errorf("Binance rejected the credentials at %s (status %d)", validator.APIBaseURL+binanceAccountEndpointPath, binanceResponse.StatusCode)
                }
                return fmt.Errorf("Binance rejected the credentials at %s (status %d): %s", validator.APIBaseURL+binanceAccountEndpointPath, binanceResponse.StatusCode, string(responseBody))
        }

        return nil
}

func (validator *BinanceCredentialValidator) buildSignedAccountRequest(requestContext context.Context, apiKey string, apiSecret string, serverTimestamp int64) (*http.Request, error) {
        accountEndpoint, parseError := url.Parse(validator.APIBaseURL)
        if parseError != nil {
                return nil, parseError
        }
        accountEndpoint.Path = binanceAccountEndpointPath

        parameters := accountEndpoint.Query()
        parameters.Set("timestamp", fmt.Sprintf("%d", serverTimestamp))

        signature := computeHMACSignature(parameters.Encode(), apiSecret)
        parameters.Set("signature", signature)
        accountEndpoint.RawQuery = parameters.Encode()

        signedRequest, buildError := http.NewRequestWithContext(requestContext, http.MethodGet, accountEndpoint.String(), nil)
        if buildError != nil {
                return nil, buildError
        }

        signedRequest.Header.Set("X-MBX-APIKEY", apiKey)
        return signedRequest, nil
}

func (validator *BinanceCredentialValidator) fetchBinanceServerTimestamp(requestContext context.Context) (int64, error) {
        timeEndpoint, parseError := url.Parse(validator.APIBaseURL)
        if parseError != nil {
                return 0, parseError
        }
        timeEndpoint.Path = binanceTimeEndpointPath

        timeRequest, requestBuildError := http.NewRequestWithContext(requestContext, http.MethodGet, timeEndpoint.String(), nil)
        if requestBuildError != nil {
                return 0, requestBuildError
        }

        timeResponse, timeError := validator.HTTPClient.Do(timeRequest)
        if timeError != nil {
                return 0, timeError
        }
        defer timeResponse.Body.Close()

        if timeResponse.StatusCode != http.StatusOK {
                return 0, fmt.Errorf("Binance time endpoint returned status %d", timeResponse.StatusCode)
        }

        var timePayload struct {
                ServerTime int64 `json:"serverTime"`
        }

        decodeError := json.NewDecoder(timeResponse.Body).Decode(&timePayload)
        if decodeError != nil {
                return 0, decodeError
        }

        if timePayload.ServerTime == 0 {
                return 0, errors.New("Binance time endpoint returned an empty timestamp")
        }

        return timePayload.ServerTime, nil
}

func computeHMACSignature(message string, secret string) string {
        mac := hmac.New(sha256.New, []byte(secret))
        mac.Write([]byte(message))
        return hex.EncodeToString(mac.Sum(nil))
}

const (
        binanceAccountEndpointPath = "/api/v3/account"
        binanceTimeEndpointPath    = "/api/v3/time"
)
