package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/base64"
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

// postLoginRedirectPath is where the browser lands after a successful Google sign-in.
const postLoginRedirectPath = "/"

var errNotAuthenticated = errors.New("not authenticated")

// AuthHandler exposes the authentication endpoints and session-cookie handling. It is
// self-contained and registers itself onto a mux, so it does not depend on the legacy Server.
type AuthHandler struct {
	AuthService         *service.AuthService
	SessionService      *service.SessionService
	GoogleOAuthService  *service.GoogleOAuthService // nil when Google sign-in is not configured
	AccountEmailService *service.AccountEmailService
	CookieName          string
	OAuthStateCookie    string
	SecureCookies       bool
	loginThrottle       *loginThrottle
}

func NewAuthHandler(authService *service.AuthService, sessionService *service.SessionService, googleOAuthService *service.GoogleOAuthService, accountEmailService *service.AccountEmailService, secureCookies bool) *AuthHandler {
	return &AuthHandler{
		AuthService:         authService,
		SessionService:      sessionService,
		GoogleOAuthService:  googleOAuthService,
		AccountEmailService: accountEmailService,
		CookieName:          "coin_hub_session",
		OAuthStateCookie:    "coin_hub_oauth_state",
		SecureCookies:       secureCookies,
		loginThrottle:       newLoginThrottle(),
	}
}

func (handler *AuthHandler) RegisterRoutes(router *http.ServeMux) {
	router.HandleFunc("/auth/signup", handler.handleSignup)
	router.HandleFunc("/auth/login", handler.handleLogin)
	router.HandleFunc("/auth/logout", handler.handleLogout)
	router.HandleFunc("/auth/me", handler.handleCurrentUser)
	router.HandleFunc("/auth/providers", handler.handleProviders)
	router.HandleFunc("/auth/google/login", handler.handleGoogleLogin)
	router.HandleFunc("/auth/google/callback", handler.handleGoogleCallback)
	router.HandleFunc("/auth/password/forgot", handler.handleForgotPassword)
	router.HandleFunc("/auth/password/reset", handler.handleResetPassword)
	router.HandleFunc("/auth/email/verify", handler.handleVerifyEmail)
	router.HandleFunc("/auth/email/resend", handler.handleResendVerification)
}

type signupRequestPayload struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	Locale      string `json:"locale"`
}

type loginRequestPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponsePayload struct {
	Identifier      int64  `json:"id"`
	Email           string `json:"email"`
	DisplayName     string `json:"display_name"`
	HasPassword     bool   `json:"has_password"`
	GoogleConnected bool   `json:"google_connected"`
	IsAdmin         bool   `json:"is_admin"`
	EmailVerified   bool   `json:"email_verified"`
	CreatedAt       string `json:"created_at"`
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

	// Best-effort: send the email-confirmation link. A failure must not block signup.
	if sendError := handler.AccountEmailService.SendVerificationEmail(registrationContext, createdUser.Identifier, createdUser.Email, resolveRequestLocale(request, payload.Locale)); sendError != nil {
		log.Printf("Could not send verification email for user %d: %v", createdUser.Identifier, sendError)
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

	// Per-account lockout: too many recent failures for this email get a 429 regardless of source IP,
	// blunting distributed credential-stuffing that slips past nginx's per-IP limit.
	if handler.loginThrottle.IsLocked(payload.Email) {
		writeJSONError(responseWriter, http.StatusTooManyRequests, "Too many sign-in attempts. Please wait a few minutes and try again.")
		return
	}

	authenticationContext, cancel := context.WithTimeout(request.Context(), 8*time.Second)
	defer cancel()

	authenticatedUser, authenticationError := handler.AuthService.Authenticate(authenticationContext, payload.Email, payload.Password)
	if authenticationError != nil {
		if errors.Is(authenticationError, service.ErrInvalidCredentials) || errors.Is(authenticationError, service.ErrAccountDisabled) {
			handler.loginThrottle.RegisterFailure(payload.Email)
			writeJSONError(responseWriter, http.StatusUnauthorized, authenticationError.Error())
			return
		}
		log.Printf("Login failed unexpectedly: %v", authenticationError)
		writeJSONError(responseWriter, http.StatusInternalServerError, "Could not sign in.")
		return
	}

	handler.loginThrottle.RegisterSuccess(payload.Email)
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
	if issueError := handler.issueSessionCookie(responseWriter, request, user); issueError != nil {
		writeJSONError(responseWriter, http.StatusInternalServerError, "Could not start a session.")
		return
	}
	writeJSON(responseWriter, http.StatusOK, toUserResponse(user))
}

// issueSessionCookie creates a session for the user and writes the session cookie. Callers decide
// how to respond afterwards (JSON for the email flow, a redirect for the OAuth callback).
func (handler *AuthHandler) issueSessionCookie(responseWriter http.ResponseWriter, request *http.Request, user *domain.User) error {
	sessionContext, cancel := context.WithTimeout(request.Context(), 5*time.Second)
	defer cancel()

	rawToken, expiresAt, issueError := handler.SessionService.IssueSession(sessionContext, user.Identifier, request.UserAgent(), clientIPAddress(request))
	if issueError != nil {
		log.Printf("Could not issue session for user %d: %v", user.Identifier, issueError)
		return issueError
	}
	handler.setSessionCookie(responseWriter, rawToken, expiresAt)
	return nil
}

// handleProviders reports which third-party sign-in methods are available, so the SPA only renders
// buttons it can actually use.
func (handler *AuthHandler) handleProviders(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(responseWriter, http.StatusOK, map[string]bool{
		"google": handler.GoogleOAuthService != nil,
		"email":  handler.AccountEmailService.EmailEnabled(),
	})
}

func (handler *AuthHandler) handleGoogleLogin(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if handler.GoogleOAuthService == nil {
		http.Redirect(responseWriter, request, postLoginRedirectPath+"?login_error=google_unavailable", http.StatusSeeOther)
		return
	}

	state, stateError := generateOAuthState()
	if stateError != nil {
		http.Redirect(responseWriter, request, postLoginRedirectPath+"?login_error=google", http.StatusSeeOther)
		return
	}
	handler.setStateCookie(responseWriter, state)
	http.Redirect(responseWriter, request, handler.GoogleOAuthService.AuthorizationURL(state), http.StatusSeeOther)
}

func (handler *AuthHandler) handleGoogleCallback(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	handler.clearStateCookie(responseWriter)

	if handler.GoogleOAuthService == nil {
		http.Redirect(responseWriter, request, postLoginRedirectPath+"?login_error=google_unavailable", http.StatusSeeOther)
		return
	}

	stateCookie, cookieError := request.Cookie(handler.OAuthStateCookie)
	queryState := request.URL.Query().Get("state")
	if cookieError != nil || queryState == "" || stateCookie.Value != queryState {
		http.Redirect(responseWriter, request, postLoginRedirectPath+"?login_error=google", http.StatusSeeOther)
		return
	}

	authorizationCode := request.URL.Query().Get("code")
	if authorizationCode == "" {
		http.Redirect(responseWriter, request, postLoginRedirectPath+"?login_error=google", http.StatusSeeOther)
		return
	}

	exchangeContext, cancel := context.WithTimeout(request.Context(), 12*time.Second)
	defer cancel()
	googleProfile, profileError := handler.GoogleOAuthService.ExchangeCodeForUserInfo(exchangeContext, authorizationCode)
	if profileError != nil {
		log.Printf("Google sign-in failed during code exchange: %v", profileError)
		http.Redirect(responseWriter, request, postLoginRedirectPath+"?login_error=google", http.StatusSeeOther)
		return
	}

	authenticatedUser, authenticationError := handler.AuthService.AuthenticateWithGoogle(exchangeContext, googleProfile)
	if authenticationError != nil {
		log.Printf("Google sign-in could not resolve an account: %v", authenticationError)
		http.Redirect(responseWriter, request, postLoginRedirectPath+"?login_error=google", http.StatusSeeOther)
		return
	}

	if issueError := handler.issueSessionCookie(responseWriter, request, authenticatedUser); issueError != nil {
		http.Redirect(responseWriter, request, postLoginRedirectPath+"?login_error=google", http.StatusSeeOther)
		return
	}
	http.Redirect(responseWriter, request, postLoginRedirectPath, http.StatusSeeOther)
}

// handleForgotPassword emails a password-reset link. It always responds 200 so it cannot be used to
// probe which emails have accounts.
func (handler *AuthHandler) handleForgotPassword(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		Email  string `json:"email"`
		Locale string `json:"locale"`
	}
	if decodeError := json.NewDecoder(request.Body).Decode(&payload); decodeError != nil {
		writeJSONError(responseWriter, http.StatusBadRequest, "Invalid request body.")
		return
	}
	operationContext, cancel := context.WithTimeout(request.Context(), 8*time.Second)
	defer cancel()
	if resetError := handler.AccountEmailService.RequestPasswordReset(operationContext, payload.Email, resolveRequestLocale(request, payload.Locale)); resetError != nil {
		log.Printf("Password reset request failed: %v", resetError)
	}
	writeJSON(responseWriter, http.StatusOK, map[string]string{"message": "If that email has an account, a reset link is on its way."})
}

func (handler *AuthHandler) handleResetPassword(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if decodeError := json.NewDecoder(request.Body).Decode(&payload); decodeError != nil {
		writeJSONError(responseWriter, http.StatusBadRequest, "Invalid request body.")
		return
	}
	if payload.Token == "" {
		writeJSONError(responseWriter, http.StatusBadRequest, "A reset token is required.")
		return
	}
	operationContext, cancel := context.WithTimeout(request.Context(), 8*time.Second)
	defer cancel()
	resetError := handler.AccountEmailService.ConfirmPasswordReset(operationContext, payload.Token, payload.NewPassword)
	switch {
	case resetError == nil:
		writeJSON(responseWriter, http.StatusOK, map[string]string{"message": "Password updated. You can sign in now."})
	case errors.Is(resetError, repository.ErrAuthTokenInvalid):
		writeJSONError(responseWriter, http.StatusBadRequest, "This reset link is invalid or has expired. Request a new one.")
	case errors.Is(resetError, service.ErrWeakPassword):
		writeJSONError(responseWriter, http.StatusBadRequest, service.ErrWeakPassword.Error())
	default:
		log.Printf("Password reset failed: %v", resetError)
		writeJSONError(responseWriter, http.StatusInternalServerError, "Could not reset your password.")
	}
}

func (handler *AuthHandler) handleVerifyEmail(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		Token string `json:"token"`
	}
	if decodeError := json.NewDecoder(request.Body).Decode(&payload); decodeError != nil {
		writeJSONError(responseWriter, http.StatusBadRequest, "Invalid request body.")
		return
	}
	if payload.Token == "" {
		writeJSONError(responseWriter, http.StatusBadRequest, "A verification token is required.")
		return
	}
	operationContext, cancel := context.WithTimeout(request.Context(), 8*time.Second)
	defer cancel()
	verifyError := handler.AccountEmailService.ConfirmEmailVerification(operationContext, payload.Token)
	switch {
	case verifyError == nil:
		writeJSON(responseWriter, http.StatusOK, map[string]string{"message": "Email confirmed."})
	case errors.Is(verifyError, repository.ErrAuthTokenInvalid):
		writeJSONError(responseWriter, http.StatusBadRequest, "This confirmation link is invalid or has expired.")
	default:
		log.Printf("Email verification failed: %v", verifyError)
		writeJSONError(responseWriter, http.StatusInternalServerError, "Could not confirm your email.")
	}
}

func (handler *AuthHandler) handleResendVerification(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	userIdentifier, authenticationError := handler.ResolveAuthenticatedUserIdentifier(request)
	if authenticationError != nil {
		writeJSONError(responseWriter, http.StatusUnauthorized, "Not authenticated.")
		return
	}
	operationContext, cancel := context.WithTimeout(request.Context(), 8*time.Second)
	defer cancel()
	currentUser, lookupError := handler.AuthService.GetUserByIdentifier(operationContext, userIdentifier)
	if lookupError != nil {
		writeJSONError(responseWriter, http.StatusUnauthorized, "Not authenticated.")
		return
	}
	if currentUser.IsEmailVerified() {
		writeJSON(responseWriter, http.StatusOK, map[string]string{"message": "Your email is already confirmed."})
		return
	}
	if sendError := handler.AccountEmailService.SendVerificationEmail(operationContext, userIdentifier, currentUser.Email, resolveRequestLocale(request, "")); sendError != nil {
		log.Printf("Could not resend verification email for user %d: %v", userIdentifier, sendError)
	}
	writeJSON(responseWriter, http.StatusOK, map[string]string{"message": "Verification email sent."})
}

// resolveRequestLocale picks the email language: the payload's locale if supported, otherwise the
// browser's Accept-Language, otherwise pt-BR.
func resolveRequestLocale(request *http.Request, payloadLocale string) string {
	if isSupportedLocale(payloadLocale) {
		return payloadLocale
	}
	acceptLanguage := strings.ToLower(request.Header.Get("Accept-Language"))
	for _, candidate := range []string{"pt", "es", "en"} {
		if strings.HasPrefix(acceptLanguage, candidate) {
			return candidate
		}
	}
	return "pt"
}

func isSupportedLocale(locale string) bool {
	return locale == "pt" || locale == "en" || locale == "es"
}

func (handler *AuthHandler) setStateCookie(responseWriter http.ResponseWriter, state string) {
	http.SetCookie(responseWriter, &http.Cookie{
		Name:     handler.OAuthStateCookie,
		Value:    state,
		Path:     "/auth",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   handler.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

func (handler *AuthHandler) clearStateCookie(responseWriter http.ResponseWriter) {
	http.SetCookie(responseWriter, &http.Cookie{
		Name:     handler.OAuthStateCookie,
		Value:    "",
		Path:     "/auth",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   handler.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

func generateOAuthState() (string, error) {
	randomBytes := make([]byte, 32)
	if _, randomError := rand.Read(randomBytes); randomError != nil {
		return "", randomError
	}
	return base64.RawURLEncoding.EncodeToString(randomBytes), nil
}

func (handler *AuthHandler) setSessionCookie(responseWriter http.ResponseWriter, rawToken string, expiresAt time.Time) {
	http.SetCookie(responseWriter, &http.Cookie{
		Name:     handler.CookieName,
		Value:    rawToken,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   handler.SecureCookies,
		// Strict: the session cookie is never attached to cross-site requests, so it cannot be
		// ridden by CSRF. All API calls are same-origin XHR from the SPA, so this does not affect
		// normal use; the OAuth state cookie stays Lax because it must survive Google's redirect.
		SameSite: http.SameSiteStrictMode,
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
		SameSite: http.SameSiteStrictMode,
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
		Identifier:      user.Identifier,
		Email:           user.Email,
		DisplayName:     user.DisplayName,
		HasPassword:     user.HasPassword(),
		GoogleConnected: user.HasGoogleLinked(),
		IsAdmin:         user.IsAdmin,
		EmailVerified:   user.IsEmailVerified(),
		CreatedAt:       user.CreatedAt.Format(time.RFC3339),
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

// writeJSONErrorCode is writeJSONError plus a machine-readable code the SPA can branch on (e.g. to
// show a specific dialog) without parsing the human message.
func writeJSONErrorCode(responseWriter http.ResponseWriter, statusCode int, message string, code string) {
	writeJSON(responseWriter, statusCode, map[string]string{"error": message, "code": code})
}

// enforceEmailVerified loads the user and writes 403 if the email is not confirmed yet. Used to block
// sensitive actions (connecting Binance, trading, robots) until the account confirms its email.
func enforceEmailVerified(operationContext context.Context, responseWriter http.ResponseWriter, authService *service.AuthService, userIdentifier int64) bool {
	currentUser, lookupError := authService.GetUserByIdentifier(operationContext, userIdentifier)
	if lookupError != nil || currentUser == nil {
		writeJSONError(responseWriter, http.StatusUnauthorized, "Not authenticated.")
		return false
	}
	if !currentUser.IsEmailVerified() {
		writeJSONErrorCode(responseWriter, http.StatusForbidden, "Confirm your email before using this feature.", "email_unverified")
		return false
	}
	return true
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
