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

const binanceAccountEndpointPath = "/api/v3/account"
const binanceTimeEndpointPath = "/api/v3/time"

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

	accountRequest, requestBuildError := validator.buildSignedAccountRequest(validationContext, apiKey, apiSecret, serverTimestamp)
	if requestBuildError != nil {
		return requestBuildError
	}

	binanceResponse, binanceResponseError := validator.HTTPClient.Do(accountRequest)
	if binanceResponseError != nil {
		return binanceResponseError
	}
	defer binanceResponse.Body.Close()

	if binanceResponse.StatusCode != http.StatusOK {
		responseBody, readError := io.ReadAll(binanceResponse.Body)
		if readError != nil {
			return fmt.Errorf("Binance rejected the credentials at %s (status %d)", validator.APIBaseURL+binanceAccountEndpointPath, binanceResponse.StatusCode)
		}
		return fmt.Errorf("Binance rejected the credentials at %s (status %d): %s", validator.APIBaseURL+binanceAccountEndpointPath, binanceResponse.StatusCode, string(responseBody))
	}

	return nil
}

func (validator *BinanceCredentialValidator) buildSignedAccountRequest(requestContext context.Context, apiKey string, apiSecret string, serverTimestamp int64) (*http.Request, error) {
	queryValues := url.Values{}
	queryValues.Set("timestamp", fmt.Sprintf("%d", serverTimestamp))
	queryValues.Set("recvWindow", "5000")
	unsignedQuery := queryValues.Encode()

	signatureSigner := hmac.New(sha256.New, []byte(apiSecret))
	_, signingError := signatureSigner.Write([]byte(unsignedQuery))
	if signingError != nil {
		return nil, signingError
	}
	signature := hex.EncodeToString(signatureSigner.Sum(nil))

	signedQuery := unsignedQuery + "&signature=" + signature
	signedEndpoint := validator.APIBaseURL + binanceAccountEndpointPath + "?" + signedQuery

	accountRequest, requestError := http.NewRequestWithContext(requestContext, http.MethodGet, signedEndpoint, nil)
	if requestError != nil {
		return nil, requestError
	}

	accountRequest.Header.Set("X-MBX-APIKEY", apiKey)
	return accountRequest, nil
}

type binanceServerTimeResponse struct {
	ServerTime int64 `json:"serverTime"`
}

func (validator *BinanceCredentialValidator) fetchBinanceServerTimestamp(requestContext context.Context) (int64, error) {
	serverTimeEndpoint := validator.APIBaseURL + binanceTimeEndpointPath

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
