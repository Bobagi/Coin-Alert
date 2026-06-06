package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"coin-alert/internal/repository"
	"coin-alert/internal/service"
)

// PortfolioHandler serves the B3 portfolio endpoints, backed by the investidor10 scraper.
type PortfolioHandler struct {
	sessionService      *service.SessionService
	cookieName          string
	portfolioRepository repository.UserPortfolioRepository
	scraperClient       *service.PortfolioScraperClient
}

func NewPortfolioHandler(sessionService *service.SessionService, cookieName string, portfolioRepository repository.UserPortfolioRepository, scraperClient *service.PortfolioScraperClient) *PortfolioHandler {
	return &PortfolioHandler{
		sessionService:      sessionService,
		cookieName:          cookieName,
		portfolioRepository: portfolioRepository,
		scraperClient:       scraperClient,
	}
}

func (handler *PortfolioHandler) RegisterRoutes(router *http.ServeMux) {
	router.HandleFunc("/api/v1/portfolio/source", handler.handleSource)
	router.HandleFunc("/api/v1/portfolio/assets", handler.handleAssets)
	router.HandleFunc("/api/v1/portfolio/dividends", handler.handleDividends)
}

func (handler *PortfolioHandler) requireUser(responseWriter http.ResponseWriter, request *http.Request) (int64, bool) {
	sessionCookie, cookieError := request.Cookie(handler.cookieName)
	if cookieError != nil {
		writeJSONError(responseWriter, http.StatusUnauthorized, "Not authenticated.")
		return 0, false
	}
	resolveContext, cancel := context.WithTimeout(request.Context(), 5*time.Second)
	defer cancel()
	userIdentifier, resolveError := handler.sessionService.ResolveUserIdentifier(resolveContext, sessionCookie.Value)
	if resolveError != nil {
		writeJSONError(responseWriter, http.StatusUnauthorized, "Not authenticated.")
		return 0, false
	}
	return userIdentifier, true
}

func (handler *PortfolioHandler) handleSource(responseWriter http.ResponseWriter, request *http.Request) {
	userIdentifier, authenticated := handler.requireUser(responseWriter, request)
	if !authenticated {
		return
	}

	switch request.Method {
	case http.MethodGet:
		operationContext, cancel := context.WithTimeout(request.Context(), 5*time.Second)
		defer cancel()
		walletURL, lookupError := handler.portfolioRepository.GetWalletURL(operationContext, userIdentifier)
		if lookupError != nil {
			writeJSONError(responseWriter, http.StatusInternalServerError, "Could not load portfolio source.")
			return
		}
		writeJSON(responseWriter, http.StatusOK, map[string]string{"wallet_url": walletURL})

	case http.MethodPut:
		var payload struct {
			WalletURL string `json:"wallet_url"`
		}
		if decodeError := json.NewDecoder(request.Body).Decode(&payload); decodeError != nil {
			writeJSONError(responseWriter, http.StatusBadRequest, "Invalid request body.")
			return
		}
		operationContext, cancel := context.WithTimeout(request.Context(), 5*time.Second)
		defer cancel()
		if saveError := handler.portfolioRepository.UpsertWalletURL(operationContext, userIdentifier, strings.TrimSpace(payload.WalletURL)); saveError != nil {
			writeJSONError(responseWriter, http.StatusInternalServerError, "Could not save portfolio source.")
			return
		}
		writeJSON(responseWriter, http.StatusOK, map[string]string{"message": "Saved."})

	default:
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (handler *PortfolioHandler) handleAssets(responseWriter http.ResponseWriter, request *http.Request) {
	handler.proxyScrape(responseWriter, request, "/assets", nil)
}

func (handler *PortfolioHandler) handleDividends(responseWriter http.ResponseWriter, request *http.Request) {
	handler.proxyScrape(responseWriter, request, "/data-com", url.Values{"async": []string{"false"}})
}

// proxyScrape resolves the user's wallet URL, calls the scraper, and passes its JSON through.
func (handler *PortfolioHandler) proxyScrape(responseWriter http.ResponseWriter, request *http.Request, scraperPath string, extraQuery url.Values) {
	if request.Method != http.MethodGet {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	userIdentifier, authenticated := handler.requireUser(responseWriter, request)
	if !authenticated {
		return
	}

	lookupContext, lookupCancel := context.WithTimeout(request.Context(), 5*time.Second)
	walletURL, lookupError := handler.portfolioRepository.GetWalletURL(lookupContext, userIdentifier)
	lookupCancel()
	if lookupError != nil {
		writeJSONError(responseWriter, http.StatusInternalServerError, "Could not load portfolio source.")
		return
	}
	if strings.TrimSpace(walletURL) == "" {
		writeJSONError(responseWriter, http.StatusBadRequest, "Set your Investidor10 wallet URL first.")
		return
	}

	query := url.Values{"wallet_url": []string{walletURL}}
	for key, values := range extraQuery {
		for _, value := range values {
			query.Add(key, value)
		}
	}

	scrapeContext, scrapeCancel := context.WithTimeout(request.Context(), 175*time.Second)
	defer scrapeCancel()
	statusCode, responseBody, scrapeError := handler.scraperClient.FetchRaw(scrapeContext, scraperPath, query)
	if scrapeError != nil {
		writeJSONError(responseWriter, http.StatusBadGateway, "The portfolio scraper is unavailable right now.")
		return
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	_, _ = responseWriter.Write(responseBody)
}
