package service

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"time"
)

// PortfolioScraperClient talks to the internal investidor10 scraper service. Scraping uses
// Selenium and can be slow, so the timeout is generous.
type PortfolioScraperClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewPortfolioScraperClient(baseURL string) *PortfolioScraperClient {
	return &PortfolioScraperClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 180 * time.Second},
	}
}

// FetchRaw performs a GET against the scraper and returns its status code and raw JSON body,
// which the API passes through to the frontend.
func (client *PortfolioScraperClient) FetchRaw(requestContext context.Context, path string, query url.Values) (int, []byte, error) {
	endpoint := client.baseURL + path
	if encodedQuery := query.Encode(); encodedQuery != "" {
		endpoint += "?" + encodedQuery
	}

	scraperRequest, buildError := http.NewRequestWithContext(requestContext, http.MethodGet, endpoint, nil)
	if buildError != nil {
		return 0, nil, buildError
	}

	scraperResponse, responseError := client.httpClient.Do(scraperRequest)
	if responseError != nil {
		return 0, nil, responseError
	}
	defer scraperResponse.Body.Close()

	responseBody, readError := io.ReadAll(scraperResponse.Body)
	if readError != nil {
		return scraperResponse.StatusCode, nil, readError
	}
	return scraperResponse.StatusCode, responseBody, nil
}
