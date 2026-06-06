package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"coin-alert/internal/domain"
	"coin-alert/internal/repository"
	"coin-alert/internal/service"
)

var errNotAuthenticated = errors.New("not authenticated")

// AuthHandler exposes the authentication endpoints and session-cookie handling. It is
// self-contained and registers itself onto a mux, so it does not depend on the legacy Server.
type AuthHandler struct {
	AuthService    *service.AuthService
	SessionService *service.SessionService
	CookieName     string
	SecureCookies  bool
}

func NewAuthHandler(authService *service.AuthService, sessionService *service.SessionService, secureCookies bool) *AuthHandler {
	return &AuthHandler{
		AuthService:    authService,
		SessionService: sessionService,
		CookieName:     "coin_hub_session",
		SecureCookies:  secureCookies,
	}
}

func (handler *AuthHandler) RegisterRoutes(router *http.ServeMux) {
	router.HandleFunc("/auth/signup", handler.handleSignup)
	router.HandleFunc("/auth/login", handler.handleLogin)
	router.HandleFunc("/auth/logout", handler.handleLogout)
	router.HandleFunc("/auth/me", handler.handleCurrentUser)
}

type signupRequestPayload struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type loginRequestPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponsePayload struct {
	Identifier  int64  `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

func (handler *AuthHandler) handleSignup(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var payload signupRequestPayload
	if decodeError := json.NewDecoder(request.Body).Decode(&payload); decodeError != nil {
		writeJSONError(responseWriter, http.StatusBadRequest, "Invalid request body.")
		return
	}

	registrationContext, cancel := context.WithTimeout(request.Context(), 8*time.Second)
	defer cancel()

	createdUser, registrationError := handler.AuthService.Register(registrationContext, payload.Email, payload.Password, payload.DisplayName)
	if registrationError != nil {
		handler.writeRegistrationError(responseWriter, registrationError)
		return
	}

	handler.issueSessionAndRespond(responseWriter, request, createdUser)
}

func (handler *AuthHandler) handleLogin(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var payload loginRequestPayload
	if decodeError := json.NewDecoder(request.Body).Decode(&payload); decodeError != nil {
		writeJSONError(responseWriter, http.StatusBadRequest, "Invalid request body.")
		return
	}

	authenticationContext, cancel := context.WithTimeout(request.Context(), 8*time.Second)
	defer cancel()

	authenticatedUser, authenticationError := handler.AuthService.Authenticate(authenticationContext, payload.Email, payload.Password)
	if authenticationError != nil {
		if errors.Is(authenticationError, service.ErrInvalidCredentials) || errors.Is(authenticationError, service.ErrAccountDisabled) {
			writeJSONError(responseWriter, http.StatusUnauthorized, authenticationError.Error())
			return
		}
		log.Printf("Login failed unexpectedly: %v", authenticationError)
		writeJSONError(responseWriter, http.StatusInternalServerError, "Could not sign in.")
		return
	}

	handler.issueSessionAndRespond(responseWriter, request, authenticatedUser)
}

func (handler *AuthHandler) handleLogout(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if sessionCookie, cookieError := request.Cookie(handler.CookieName); cookieError == nil {
		revokeContext, cancel := context.WithTimeout(request.Context(), 5*time.Second)
		defer cancel()
		if revokeError := handler.SessionService.RevokeSession(revokeContext, sessionCookie.Value); revokeError != nil {
			log.Printf("Could not revoke session on logout: %v", revokeError)
		}
	}

	handler.clearSessionCookie(responseWriter)
	writeJSON(responseWriter, http.StatusOK, map[string]string{"message": "Signed out."})
}

func (handler *AuthHandler) handleCurrentUser(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	userIdentifier, authenticationError := handler.ResolveAuthenticatedUserIdentifier(request)
	if authenticationError != nil {
		writeJSONError(responseWriter, http.StatusUnauthorized, "Not authenticated.")
		return
	}

	lookupContext, cancel := context.WithTimeout(request.Context(), 5*time.Second)
	defer cancel()
	currentUser, lookupError := handler.AuthService.GetUserByIdentifier(lookupContext, userIdentifier)
	if lookupError != nil {
		writeJSONError(responseWriter, http.StatusUnauthorized, "Not authenticated.")
		return
	}

	writeJSON(responseWriter, http.StatusOK, toUserResponse(currentUser))
}

// ResolveAuthenticatedUserIdentifier reads and validates the session cookie. It is exported so
// future protected handlers (and middleware) can reuse it.
func (handler *AuthHandler) ResolveAuthenticatedUserIdentifier(request *http.Request) (int64, error) {
	sessionCookie, cookieError := request.Cookie(handler.CookieName)
	if cookieError != nil {
		return 0, errNotAuthenticated
	}

	resolveContext, cancel := context.WithTimeout(request.Context(), 5*time.Second)
	defer cancel()
	userIdentifier, resolveError := handler.SessionService.ResolveUserIdentifier(resolveContext, sessionCookie.Value)
	if resolveError != nil {
		return 0, errNotAuthenticated
	}
	return userIdentifier, nil
}

func (handler *AuthHandler) issueSessionAndRespond(responseWriter http.ResponseWriter, request *http.Request, user *domain.User) {
	sessionContext, cancel := context.WithTimeout(request.Context(), 5*time.Second)
	defer cancel()

	rawToken, expiresAt, issueError := handler.SessionService.IssueSession(sessionContext, user.Identifier, request.UserAgent(), clientIPAddress(request))
	if issueError != nil {
		log.Printf("Could not issue session for user %d: %v", user.Identifier, issueError)
		writeJSONError(responseWriter, http.StatusInternalServerError, "Could not start a session.")
		return
	}

	handler.setSessionCookie(responseWriter, rawToken, expiresAt)
	writeJSON(responseWriter, http.StatusOK, toUserResponse(user))
}

func (handler *AuthHandler) setSessionCookie(responseWriter http.ResponseWriter, rawToken string, expiresAt time.Time) {
	http.SetCookie(responseWriter, &http.Cookie{
		Name:     handler.CookieName,
		Value:    rawToken,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   handler.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

func (handler *AuthHandler) clearSessionCookie(responseWriter http.ResponseWriter) {
	http.SetCookie(responseWriter, &http.Cookie{
		Name:     handler.CookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   handler.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

func (handler *AuthHandler) writeRegistrationError(responseWriter http.ResponseWriter, registrationError error) {
	switch {
	case errors.Is(registrationError, service.ErrInvalidEmail), errors.Is(registrationError, service.ErrWeakPassword):
		writeJSONError(responseWriter, http.StatusBadRequest, registrationError.Error())
	case errors.Is(registrationError, repository.ErrEmailAlreadyRegistered):
		writeJSONError(responseWriter, http.StatusConflict, "That email is already registered.")
	default:
		log.Printf("Registration failed unexpectedly: %v", registrationError)
		writeJSONError(responseWriter, http.StatusInternalServerError, "Could not create the account.")
	}
}

func toUserResponse(user *domain.User) userResponsePayload {
	return userResponsePayload{
		Identifier:  user.Identifier,
		Email:       user.Email,
		DisplayName: user.DisplayName,
	}
}

func writeJSON(responseWriter http.ResponseWriter, statusCode int, payload interface{}) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	if encodeError := json.NewEncoder(responseWriter).Encode(payload); encodeError != nil {
		log.Printf("Could not encode JSON response: %v", encodeError)
	}
}

func writeJSONError(responseWriter http.ResponseWriter, statusCode int, message string) {
	writeJSON(responseWriter, statusCode, map[string]string{"error": message})
}

func clientIPAddress(request *http.Request) string {
	forwardedFor := request.Header.Get("X-Forwarded-For")
	if forwardedFor != "" {
		return strings.TrimSpace(strings.Split(forwardedFor, ",")[0])
	}
	host, _, splitError := net.SplitHostPort(request.RemoteAddr)
	if splitError != nil {
		return request.RemoteAddr
	}
	return host
}
