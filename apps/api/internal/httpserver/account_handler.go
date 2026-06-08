package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"coin-alert/internal/service"
)

// AccountHandler serves the session-protected account-management endpoints: editing the profile,
// setting/changing the password, and permanently deleting the account.
type AccountHandler struct {
	authService    *service.AuthService
	sessionService *service.SessionService
	cookieName     string
	secureCookies  bool
}

func NewAccountHandler(authService *service.AuthService, sessionService *service.SessionService, cookieName string, secureCookies bool) *AccountHandler {
	return &AccountHandler{
		authService:    authService,
		sessionService: sessionService,
		cookieName:     cookieName,
		secureCookies:  secureCookies,
	}
}

func (handler *AccountHandler) RegisterRoutes(router *http.ServeMux) {
	router.HandleFunc("/api/v1/account/profile", handler.handleProfile)
	router.HandleFunc("/api/v1/account/password", handler.handlePassword)
	router.HandleFunc("/api/v1/account", handler.handleDeleteAccount)
}

func (handler *AccountHandler) requireUser(responseWriter http.ResponseWriter, request *http.Request) (int64, bool) {
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

func (handler *AccountHandler) handleProfile(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPut {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	userIdentifier, authenticated := handler.requireUser(responseWriter, request)
	if !authenticated {
		return
	}

	var payload struct {
		DisplayName string `json:"display_name"`
	}
	if decodeError := json.NewDecoder(request.Body).Decode(&payload); decodeError != nil {
		writeJSONError(responseWriter, http.StatusBadRequest, "Invalid request body.")
		return
	}
	if len(strings.TrimSpace(payload.DisplayName)) > 120 {
		writeJSONError(responseWriter, http.StatusBadRequest, "Name is too long (max 120 characters).")
		return
	}

	operationContext, cancel := context.WithTimeout(request.Context(), 5*time.Second)
	defer cancel()
	updatedUser, updateError := handler.authService.UpdateDisplayName(operationContext, userIdentifier, payload.DisplayName)
	if updateError != nil {
		writeJSONError(responseWriter, http.StatusInternalServerError, "Could not update your profile.")
		return
	}
	writeJSON(responseWriter, http.StatusOK, toUserResponse(updatedUser))
}

func (handler *AccountHandler) handlePassword(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	userIdentifier, authenticated := handler.requireUser(responseWriter, request)
	if !authenticated {
		return
	}

	var payload struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if decodeError := json.NewDecoder(request.Body).Decode(&payload); decodeError != nil {
		writeJSONError(responseWriter, http.StatusBadRequest, "Invalid request body.")
		return
	}

	operationContext, cancel := context.WithTimeout(request.Context(), 8*time.Second)
	defer cancel()
	changeError := handler.authService.SetOrChangePassword(operationContext, userIdentifier, payload.CurrentPassword, payload.NewPassword)
	if changeError != nil {
		switch {
		case errors.Is(changeError, service.ErrIncorrectPassword):
			writeJSONError(responseWriter, http.StatusBadRequest, "Your current password is incorrect.")
		case errors.Is(changeError, service.ErrWeakPassword):
			writeJSONError(responseWriter, http.StatusBadRequest, service.ErrWeakPassword.Error())
		default:
			writeJSONError(responseWriter, http.StatusInternalServerError, "Could not update your password.")
		}
		return
	}
	writeJSON(responseWriter, http.StatusOK, map[string]string{"message": "Password updated."})
}

func (handler *AccountHandler) handleDeleteAccount(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodDelete {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	userIdentifier, authenticated := handler.requireUser(responseWriter, request)
	if !authenticated {
		return
	}

	var payload struct {
		Password string `json:"password"`
		Confirm  bool   `json:"confirm"`
	}
	if decodeError := json.NewDecoder(request.Body).Decode(&payload); decodeError != nil {
		writeJSONError(responseWriter, http.StatusBadRequest, "Invalid request body.")
		return
	}
	if !payload.Confirm {
		writeJSONError(responseWriter, http.StatusBadRequest, "Account deletion must be explicitly confirmed.")
		return
	}

	operationContext, cancel := context.WithTimeout(request.Context(), 8*time.Second)
	defer cancel()
	deletionError := handler.authService.DeleteAccount(operationContext, userIdentifier, payload.Password)
	if deletionError != nil {
		if errors.Is(deletionError, service.ErrIncorrectPassword) {
			writeJSONError(responseWriter, http.StatusBadRequest, "Your password is incorrect.")
			return
		}
		writeJSONError(responseWriter, http.StatusInternalServerError, "Could not delete your account.")
		return
	}

	// Best-effort revoke of the current session, then clear the cookie so the browser is signed out.
	if sessionCookie, cookieError := request.Cookie(handler.cookieName); cookieError == nil {
		revokeContext, revokeCancel := context.WithTimeout(request.Context(), 5*time.Second)
		defer revokeCancel()
		_ = handler.sessionService.RevokeSession(revokeContext, sessionCookie.Value)
	}
	handler.clearSessionCookie(responseWriter)
	writeJSON(responseWriter, http.StatusOK, map[string]string{"message": "Account deleted."})
}

func (handler *AccountHandler) clearSessionCookie(responseWriter http.ResponseWriter) {
	http.SetCookie(responseWriter, &http.Cookie{
		Name:     handler.cookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   handler.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}
