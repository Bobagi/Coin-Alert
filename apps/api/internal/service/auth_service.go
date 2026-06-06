package service

import (
	"context"
	"errors"
	"log"
	"strings"

	"coin-alert/internal/domain"
	"coin-alert/internal/repository"
)

// Authentication errors surfaced to HTTP handlers.
var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidEmail       = errors.New("a valid email address is required")
	ErrWeakPassword       = errors.New("password must be between 8 and 72 characters")
	ErrAccountDisabled    = errors.New("this account is disabled")
)

// bcrypt silently truncates passwords beyond 72 bytes, so we reject them explicitly.
const minimumPasswordLength = 8
const maximumPasswordLength = 72

// AuthService registers and authenticates users.
type AuthService struct {
	userRepository            repository.UserRepository
	tradingSettingsRepository repository.UserTradingSettingsRepository
	passwordService           *PasswordService
	placeholderPasswordHash   string
}

func NewAuthService(userRepository repository.UserRepository, tradingSettingsRepository repository.UserTradingSettingsRepository, passwordService *PasswordService) *AuthService {
	// A real bcrypt hash compared against when an email is unknown, to keep authentication
	// timing roughly constant and avoid leaking which emails exist.
	placeholderPasswordHash, _ := passwordService.HashPassword("placeholder-password-for-constant-time-auth")
	return &AuthService{
		userRepository:            userRepository,
		tradingSettingsRepository: tradingSettingsRepository,
		passwordService:           passwordService,
		placeholderPasswordHash:   placeholderPasswordHash,
	}
}

func (service *AuthService) Register(registrationContext context.Context, email string, password string, displayName string) (*domain.User, error) {
	normalizedEmail := strings.TrimSpace(email)
	if !isPlausibleEmail(normalizedEmail) {
		return nil, ErrInvalidEmail
	}
	if len(password) < minimumPasswordLength || len(password) > maximumPasswordLength {
		return nil, ErrWeakPassword
	}

	passwordHash, hashError := service.passwordService.HashPassword(password)
	if hashError != nil {
		return nil, hashError
	}

	createdUser, creationError := service.userRepository.CreateUser(registrationContext, normalizedEmail, passwordHash, displayName)
	if creationError != nil {
		return nil, creationError
	}

	if _, defaultsError := service.tradingSettingsRepository.EnsureDefaults(registrationContext, createdUser.Identifier); defaultsError != nil {
		log.Printf("Could not seed default trading settings for user %d: %v", createdUser.Identifier, defaultsError)
	}

	return createdUser, nil
}

func (service *AuthService) Authenticate(authenticationContext context.Context, email string, password string) (*domain.User, error) {
	foundUser, lookupError := service.userRepository.FindByEmail(authenticationContext, email)
	if errors.Is(lookupError, repository.ErrUserNotFound) {
		service.passwordService.VerifyPassword(service.placeholderPasswordHash, password)
		return nil, ErrInvalidCredentials
	}
	if lookupError != nil {
		return nil, lookupError
	}

	if !service.passwordService.VerifyPassword(foundUser.PasswordHash, password) {
		return nil, ErrInvalidCredentials
	}
	if !foundUser.IsActive {
		return nil, ErrAccountDisabled
	}
	return foundUser, nil
}

func (service *AuthService) GetUserByIdentifier(lookupContext context.Context, userIdentifier int64) (*domain.User, error) {
	return service.userRepository.FindByIdentifier(lookupContext, userIdentifier)
}

func isPlausibleEmail(candidate string) bool {
	if len(candidate) < 3 || len(candidate) > 255 {
		return false
	}
	atIndex := strings.IndexByte(candidate, '@')
	if atIndex <= 0 || atIndex == len(candidate)-1 {
		return false
	}
	if strings.ContainsAny(candidate, " \t\r\n") {
		return false
	}
	return strings.Contains(candidate[atIndex+1:], ".")
}
