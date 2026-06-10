package service

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"coin-alert/internal/email"
	"coin-alert/internal/repository"
)

const passwordResetTokenTTL = time.Hour
const emailVerificationTokenTTL = 24 * time.Hour

// AccountEmailService drives the email-based flows: password reset and email verification. Tokens are
// opaque random strings stored only as a hash; the raw value travels in the emailed link.
type AccountEmailService struct {
	userRepository    repository.UserRepository
	tokenRepository   repository.AuthTokenRepository
	sessionRepository repository.UserSessionRepository
	passwordService   *PasswordService
	emailSender       email.Sender
	baseURL           string
}

func NewAccountEmailService(userRepository repository.UserRepository, tokenRepository repository.AuthTokenRepository, sessionRepository repository.UserSessionRepository, passwordService *PasswordService, emailSender email.Sender, baseURL string) *AccountEmailService {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://coin.bobagi.space"
	}
	return &AccountEmailService{
		userRepository:    userRepository,
		tokenRepository:   tokenRepository,
		sessionRepository: sessionRepository,
		passwordService:   passwordService,
		emailSender:       emailSender,
		baseURL:           strings.TrimRight(baseURL, "/"),
	}
}

// EmailEnabled reports whether a real email transport is configured.
func (service *AccountEmailService) EmailEnabled() bool {
	return service.emailSender != nil && service.emailSender.Enabled()
}

// RequestPasswordReset emails a reset link if the address belongs to a password account. It never
// reveals whether the email exists (always succeeds from the caller's point of view).
func (service *AccountEmailService) RequestPasswordReset(operationContext context.Context, emailAddress string, locale string) error {
	normalizedEmail := strings.TrimSpace(emailAddress)
	foundUser, lookupError := service.userRepository.FindByEmail(operationContext, normalizedEmail)
	if errors.Is(lookupError, repository.ErrUserNotFound) {
		return nil
	}
	if lookupError != nil {
		return lookupError
	}
	if !foundUser.HasPassword() {
		// Google-only account: there is no password to reset. Stay silent so we don't leak the method.
		return nil
	}

	_ = service.tokenRepository.InvalidateUserTokens(operationContext, foundUser.Identifier, repository.AuthTokenPurposePasswordReset)
	rawToken, tokenError := generateSessionToken()
	if tokenError != nil {
		return tokenError
	}
	if createError := service.tokenRepository.CreateToken(operationContext, foundUser.Identifier, repository.AuthTokenPurposePasswordReset, hashSessionToken(rawToken), time.Now().Add(passwordResetTokenTTL)); createError != nil {
		return createError
	}

	resetLink := service.baseURL + "/#/reset?token=" + rawToken
	service.sendAsync(passwordResetEmail(locale, resetLink), foundUser.Email)
	return nil
}

// ConfirmPasswordReset validates the token and sets the new password. It also marks the email
// verified (clicking the link proves ownership) and revokes all existing sessions.
func (service *AccountEmailService) ConfirmPasswordReset(operationContext context.Context, rawToken string, newPassword string) error {
	token, lookupError := service.tokenRepository.FindValidByHash(operationContext, hashSessionToken(rawToken), repository.AuthTokenPurposePasswordReset)
	if lookupError != nil {
		return lookupError
	}
	if len(newPassword) < minimumPasswordLength || len(newPassword) > maximumPasswordLength {
		return ErrWeakPassword
	}

	passwordHash, hashError := service.passwordService.HashPassword(newPassword)
	if hashError != nil {
		return hashError
	}
	if updateError := service.userRepository.UpdatePasswordHash(operationContext, token.UserIdentifier, passwordHash); updateError != nil {
		return updateError
	}
	_ = service.tokenRepository.MarkUsed(operationContext, token.Identifier)
	_ = service.userRepository.MarkEmailVerified(operationContext, token.UserIdentifier)
	// A password reset should sign the account out everywhere.
	_ = service.sessionRepository.DeleteAllForUser(operationContext, token.UserIdentifier)
	return nil
}

// SendVerificationEmail issues a fresh verification link for the user.
func (service *AccountEmailService) SendVerificationEmail(operationContext context.Context, userIdentifier int64, emailAddress string, locale string) error {
	_ = service.tokenRepository.InvalidateUserTokens(operationContext, userIdentifier, repository.AuthTokenPurposeEmailVerification)
	rawToken, tokenError := generateSessionToken()
	if tokenError != nil {
		return tokenError
	}
	if createError := service.tokenRepository.CreateToken(operationContext, userIdentifier, repository.AuthTokenPurposeEmailVerification, hashSessionToken(rawToken), time.Now().Add(emailVerificationTokenTTL)); createError != nil {
		return createError
	}

	verifyLink := service.baseURL + "/#/verify?token=" + rawToken
	service.sendAsync(emailVerificationEmail(locale, verifyLink), emailAddress)
	return nil
}

// ConfirmEmailVerification validates the token and marks the user's email verified.
func (service *AccountEmailService) ConfirmEmailVerification(operationContext context.Context, rawToken string) error {
	token, lookupError := service.tokenRepository.FindValidByHash(operationContext, hashSessionToken(rawToken), repository.AuthTokenPurposeEmailVerification)
	if lookupError != nil {
		return lookupError
	}
	if markError := service.userRepository.MarkEmailVerified(operationContext, token.UserIdentifier); markError != nil {
		return markError
	}
	_ = service.tokenRepository.MarkUsed(operationContext, token.Identifier)
	return nil
}

// sendAsync delivers the email in the background so the HTTP request never blocks on SMTP.
func (service *AccountEmailService) sendAsync(message email.Message, recipient string) {
	message.To = recipient
	go func() {
		sendContext, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		if sendError := service.emailSender.Send(sendContext, message); sendError != nil {
			log.Printf("email send failed (to=%s subject=%q): %v", message.To, message.Subject, sendError)
		}
	}()
}
