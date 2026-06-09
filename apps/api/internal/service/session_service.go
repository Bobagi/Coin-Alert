package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log"
	"time"

	"coin-alert/internal/domain"
	"coin-alert/internal/repository"
)

// ErrSessionNotFound is returned when a session token does not map to an active session.
var ErrSessionNotFound = errors.New("session not found or expired")

// SessionService issues and resolves opaque, server-side sessions. The raw token is returned
// only once (to be placed in a secure cookie); the database stores only its SHA-256 hash.
type SessionService struct {
	sessionRepository repository.UserSessionRepository
	sessionLifetime   time.Duration
}

func NewSessionService(sessionRepository repository.UserSessionRepository, sessionLifetime time.Duration) *SessionService {
	if sessionLifetime <= 0 {
		sessionLifetime = 720 * time.Hour
	}
	return &SessionService{sessionRepository: sessionRepository, sessionLifetime: sessionLifetime}
}

// IssueSession creates a session and returns the raw token plus its expiry.
func (service *SessionService) IssueSession(issueContext context.Context, userIdentifier int64, userAgent string, ipAddress string) (string, time.Time, error) {
	rawToken, tokenError := generateSessionToken()
	if tokenError != nil {
		return "", time.Time{}, tokenError
	}

	expiresAt := time.Now().Add(service.sessionLifetime)
	session := domain.UserSession{
		UserIdentifier:   userIdentifier,
		SessionTokenHash: hashSessionToken(rawToken),
		ExpiresAt:        expiresAt,
		UserAgent:        userAgent,
		IPAddress:        ipAddress,
	}

	creationError := service.sessionRepository.CreateSession(issueContext, session)
	if creationError != nil {
		return "", time.Time{}, creationError
	}
	return rawToken, expiresAt, nil
}

// ResolveUserIdentifier maps a raw session token to its owning user identifier.
func (service *SessionService) ResolveUserIdentifier(resolveContext context.Context, rawToken string) (int64, error) {
	if rawToken == "" {
		return 0, ErrSessionNotFound
	}
	session, lookupError := service.sessionRepository.FindActiveByTokenHash(resolveContext, hashSessionToken(rawToken))
	if lookupError != nil {
		return 0, lookupError
	}
	if session == nil {
		return 0, ErrSessionNotFound
	}
	return session.UserIdentifier, nil
}

// RevokeSession deletes the session backing a raw token, if any.
func (service *SessionService) RevokeSession(revokeContext context.Context, rawToken string) error {
	if rawToken == "" {
		return nil
	}
	return service.sessionRepository.DeleteByTokenHash(revokeContext, hashSessionToken(rawToken))
}

// StartExpiredSessionCleanup runs a background loop that periodically deletes sessions past their
// expiry, so the user_sessions table does not grow without bound. It returns immediately; the loop
// stops when the supplied context is cancelled.
func (service *SessionService) StartExpiredSessionCleanup(loopContext context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Hour
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		service.purgeExpiredSessionsOnce(loopContext)
		for {
			select {
			case <-loopContext.Done():
				return
			case <-ticker.C:
				service.purgeExpiredSessionsOnce(loopContext)
			}
		}
	}()
}

func (service *SessionService) purgeExpiredSessionsOnce(parentContext context.Context) {
	purgeContext, cancel := context.WithTimeout(parentContext, 30*time.Second)
	defer cancel()
	deletedCount, deletionError := service.sessionRepository.DeleteExpiredSessions(purgeContext)
	if deletionError != nil {
		log.Printf("session cleanup: could not delete expired sessions: %v", deletionError)
		return
	}
	if deletedCount > 0 {
		log.Printf("session cleanup: removed %d expired session(s)", deletedCount)
	}
}

func generateSessionToken() (string, error) {
	randomBytes := make([]byte, 32)
	if _, randomError := rand.Read(randomBytes); randomError != nil {
		return "", randomError
	}
	return base64.RawURLEncoding.EncodeToString(randomBytes), nil
}

func hashSessionToken(rawToken string) string {
	digest := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(digest[:])
}
