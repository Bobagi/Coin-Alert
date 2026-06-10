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
	authService         *service.AuthService
	cookieName          string
	portfolioRepository repository.UserPortfolioRepository
	scraperClient       *service.PortfolioScraperClient
}

func NewPortfolioHandler(sessionService *service.SessionService, authService *service.AuthService, cookieName string, portfolioRepository repository.UserPortfolioRepository, scraperClient *service.PortfolioScraperClient) *PortfolioHandler {
	return &PortfolioHandler{
		sessionService:      sessionService,
		authService:         authService,
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

// requireAdminUser resolves the session and then enforces that the account is an admin. The whole
// B3/Investidor10 feature is admin-only, so every portfolio endpoint goes through this.
func (handler *PortfolioHandler) requireAdminUser(responseWriter http.ResponseWriter, request *http.Request) (int64, bool) {
	userIdentifier, authenticated := handler.requireUser(responseWriter, request)
	if !authenticated {
		return 0, false
	}
	lookupContext, cancel := context.WithTimeout(request.Context(), 5*time.Second)
	defer cancel()
	currentUser, lookupError := handler.authService.GetUserByIdentifier(lookupContext, userIdentifier)
	if lookupError != nil || currentUser == nil {
		writeJSONError(responseWriter, http.StatusUnauthorized, "Not authenticated.")
		return 0, false
	}
	if !currentUser.IsAdmin {
		writeJSONError(responseWriter, http.StatusForbidden, "The B3 portfolio is available to admins only.")
		return 0, false
	}
	return userIdentifier, true
}

func (handler *PortfolioHandler) handleSource(responseWriter http.ResponseWriter, request *http.Request) {
	userIdentifier, authenticated := handler.requireAdminUser(responseWriter, request)
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
		trimmedWalletURL := strings.TrimSpace(payload.WalletURL)
		// An empty value clears the source; otherwise it must point at Investidor10. This stops the
		// scraper from being pointed at internal/cloud-metadata hosts (SSRF) via the wallet URL.
		if trimmedWalletURL != "" && !isAllowedWalletURL(trimmedWalletURL) {
			writeJSONError(responseWriter, http.StatusBadRequest, "Enter a valid Investidor10 wallet URL (e.g. https://investidor10.com.br/carteiras/...).")
			return
		}
		operationContext, cancel := context.WithTimeout(request.Context(), 5*time.Second)
		defer cancel()
		if saveError := handler.portfolioRepository.UpsertWalletURL(operationContext, userIdentifier, trimmedWalletURL); saveError != nil {
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
	userIdentifier, authenticated := handler.requireAdminUser(responseWriter, request)
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
	// Defense in depth: reject anything that is not an Investidor10 URL before the scraper fetches it.
	if !isAllowedWalletURL(walletURL) {
		writeJSONError(responseWriter, http.StatusBadRequest, "Your saved wallet URL is not a valid Investidor10 address. Please update it.")
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

// investidor10Host is the only host the portfolio scraper is allowed to be pointed at.
const investidor10Host = "investidor10.com.br"

// isAllowedWalletURL reports whether rawWalletURL is a plain http(s) URL on investidor10.com.br
// (or a subdomain). The scraper fetches this URL server-side, so restricting it to the expected
// host prevents the wallet URL from being abused for SSRF against internal or metadata endpoints.
func isAllowedWalletURL(rawWalletURL string) bool {
	parsedURL, parseError := url.Parse(strings.TrimSpace(rawWalletURL))
	if parseError != nil {
		return false
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return false
	}
	host := strings.ToLower(parsedURL.Hostname())
	return host == investidor10Host || strings.HasSuffix(host, "."+investidor10Host)
}
