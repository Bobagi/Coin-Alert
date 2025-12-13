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
        sanitizedBaseURL := strings.TrimRight(apiBaseURL, "/")
        if sanitizedBaseURL == "" {
                sanitizedBaseURL = "https://api.binance.com"
        }

        return &BinanceCredentialValidator{
                APIBaseURL: sanitizedBaseURL,
                HTTPClient: &http.Client{Timeout: 8 * time.Second},
        }
}

func (validator *BinanceCredentialValidator) ValidateCredentials(validationContext context.Context, apiKey string, apiSecret string) error {
        if strings.TrimSpace(apiKey) == "" || strings.TrimSpace(apiSecret) == "" {
                return errors.New("Binance API Key and Secret Key are required")
        }

        if len(apiKey) < 10 || len(apiSecret) < 10 {
                return errors.New("Credentials are too short. Please verify the API Key and Secret Key")
        }

        serverTimestamp, serverTimeError := validator.fetchBinanceServerTimestamp(validationContext)
        if serverTimeError != nil {
                return serverTimeError
        }

        signedRequestURL, signingError := validator.buildSignedAccountURL(apiSecret, serverTimestamp)
        if signingError != nil {
                return signingError
        }

        accountRequest, requestError := http.NewRequestWithContext(validationContext, http.MethodGet, signedRequestURL, nil)
        if requestError != nil {
                return requestError
        }
	accountRequest.Header.Set("X-MBX-APIKEY", apiKey)

        response, responseError := validator.HTTPClient.Do(accountRequest)
        if responseError != nil {
                return responseError
        }
        defer response.Body.Close()

        if response.StatusCode != http.StatusOK {
                responseBody, readError := io.ReadAll(response.Body)
                if readError != nil {
                        return fmt.Errorf("Binance rejected the credentials (status %d)", response.StatusCode)
                }
                return fmt.Errorf("Binance rejected the credentials (status %d): %s", response.StatusCode, string(responseBody))
        }

        return nil
}

func (validator *BinanceCredentialValidator) buildSignedAccountURL(apiSecret string, serverTimestamp int64) (string, error) {
        queryValues := url.Values{}
        queryValues.Set("timestamp", fmt.Sprintf("%d", serverTimestamp))
        queryValues.Set("recvWindow", "5000")
        unsignedQuery := queryValues.Encode()

        signer := hmac.New(sha256.New, []byte(apiSecret))
        _, signingError := signer.Write([]byte(unsignedQuery))
        if signingError != nil {
                return "", signingError
        }
        signature := hex.EncodeToString(signer.Sum(nil))

        signedQuery := unsignedQuery + "&signature=" + signature
        return validator.APIBaseURL + "/api/v3/account?" + signedQuery, nil
}

type binanceServerTimeResponse struct {
        ServerTime int64 `json:"serverTime"`
}

func (validator *BinanceCredentialValidator) fetchBinanceServerTimestamp(requestContext context.Context) (int64, error) {
        serverTimeEndpoint := validator.APIBaseURL + "/api/v3/time"

        timeRequest, requestCreationError := http.NewRequestWithContext(requestContext, http.MethodGet, serverTimeEndpoint, nil)
        if requestCreationError != nil {
                return 0, requestCreationError
        }

        timeResponse, timeResponseError := validator.HTTPClient.Do(timeRequest)
        if timeResponseError != nil {
                return 0, timeResponseError
        }
        defer timeResponse.Body.Close()

        if timeResponse.StatusCode != http.StatusOK {
                return 0, fmt.Errorf("Binance time endpoint returned status %d", timeResponse.StatusCode)
        }

        var serverTimePayload binanceServerTimeResponse
        decodeError := json.NewDecoder(timeResponse.Body).Decode(&serverTimePayload)
        if decodeError != nil {
                return 0, decodeError
        }

        if serverTimePayload.ServerTime == 0 {
                return 0, errors.New("Binance time endpoint returned an empty timestamp")
        }

        return serverTimePayload.ServerTime, nil
}
