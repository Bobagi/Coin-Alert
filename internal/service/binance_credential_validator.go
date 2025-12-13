package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
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
		return errors.New("A chave e o segredo da Binance são obrigatórios")
	}

	if len(apiKey) < 10 || len(apiSecret) < 10 {
		return errors.New("Credenciais muito curtas. Verifique a API Key e Secret")
	}

	signedRequestURL, signingError := validator.buildSignedAccountURL(apiSecret)
	if signingError != nil {
		return signingError
	}

	accountRequest, requestError := http.NewRequestWithContext(validationContext, http.MethodGet, signedRequestURL, nil)
	if requestError != nil {
		return requestError
	}
	accountRequest.Header.Set("X-MBX-APIKEY", apiKey)

	response, httpError := validator.HTTPClient.Do(accountRequest)
	if httpError != nil {
		return httpError
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Binance rejeitou as credenciais (status %d)", response.StatusCode)
	}

	return nil
}

func (validator *BinanceCredentialValidator) buildSignedAccountURL(apiSecret string) (string, error) {
	currentTimestamp := time.Now().UnixMilli()
	queryValues := url.Values{}
	queryValues.Set("timestamp", fmt.Sprintf("%d", currentTimestamp))
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
