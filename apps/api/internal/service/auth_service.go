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
	ErrInvalidCredentials    = errors.New("invalid email or password")
	ErrInvalidEmail          = errors.New("a valid email address is required")
	ErrWeakPassword          = errors.New("password must be between 8 and 72 characters")
	ErrAccountDisabled       = errors.New("this account is disabled")
	ErrIncorrectPassword     = errors.New("current password is incorrect")
	ErrGoogleEmailUnverified = errors.New("your Google account email is not verified")
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

	if _, defaultsError := service.tradingSettingsRepository.EnsureDefaults(registrationContext, createdUser.Identifier, domain.BinanceEnvironmentTestnet); defaultsError != nil {
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

// AuthenticateWithGoogle resolves the local account behind a verified Google profile, creating or
// linking one as needed:
//   - a previously linked account is returned directly;
//   - an existing account with the same (verified) email gets the Google identity linked to it;
//   - otherwise a new passwordless account is provisioned and seeded with default settings.
func (service *AuthService) AuthenticateWithGoogle(authenticationContext context.Context, googleProfile *GoogleUserInfo) (*domain.User, error) {
	if !googleProfile.EmailVerified {
		return nil, ErrGoogleEmailUnverified
	}
	normalizedEmail := strings.TrimSpace(googleProfile.Email)

	existingBySubject, subjectLookupError := service.userRepository.FindByGoogleSubject(authenticationContext, googleProfile.Subject)
	if subjectLookupError == nil {
		if !existingBySubject.IsActive {
			return nil, ErrAccountDisabled
		}
		return existingBySubject, nil
	}
	if !errors.Is(subjectLookupError, repository.ErrUserNotFound) {
		return nil, subjectLookupError
	}

	existingByEmail, emailLookupError := service.userRepository.FindByEmail(authenticationContext, normalizedEmail)
	if emailLookupError == nil {
		if !existingByEmail.IsActive {
			return nil, ErrAccountDisabled
		}
		if linkError := service.userRepository.LinkGoogleSubject(authenticationContext, existingByEmail.Identifier, googleProfile.Subject); linkError != nil {
			return nil, linkError
		}
		existingByEmail.GoogleSubject = googleProfile.Subject
		return existingByEmail, nil
	}
	if !errors.Is(emailLookupError, repository.ErrUserNotFound) {
		return nil, emailLookupError
	}

	createdUser, creationError := service.userRepository.CreateGoogleUser(authenticationContext, normalizedEmail, googleProfile.Subject, googleProfile.Name)
	if creationError != nil {
		return nil, creationError
	}
	if _, defaultsError := service.tradingSettingsRepository.EnsureDefaults(authenticationContext, createdUser.Identifier, domain.BinanceEnvironmentTestnet); defaultsError != nil {
		log.Printf("Could not seed default trading settings for user %d: %v", createdUser.Identifier, defaultsError)
	}
	return createdUser, nil
}

// UpdateDisplayName changes the account's display name.
func (service *AuthService) UpdateDisplayName(updateContext context.Context, userIdentifier int64, displayName string) (*domain.User, error) {
	return service.userRepository.UpdateDisplayName(updateContext, userIdentifier, strings.TrimSpace(displayName))
}

// SetOrChangePassword sets a password for a passwordless (Google) account, or changes it for an
// account that already has one (in which case the current password must be supplied and match).
func (service *AuthService) SetOrChangePassword(updateContext context.Context, userIdentifier int64, currentPassword string, newPassword string) error {
	existingUser, lookupError := service.userRepository.FindByIdentifier(updateContext, userIdentifier)
	if lookupError != nil {
		return lookupError
	}
	if existingUser.HasPassword() && !service.passwordService.VerifyPassword(existingUser.PasswordHash, currentPassword) {
		return ErrIncorrectPassword
	}
	if len(newPassword) < minimumPasswordLength || len(newPassword) > maximumPasswordLength {
		return ErrWeakPassword
	}

	passwordHash, hashError := service.passwordService.HashPassword(newPassword)
	if hashError != nil {
		return hashError
	}
	return service.userRepository.UpdatePasswordHash(updateContext, userIdentifier, passwordHash)
}

// DeleteAccount permanently removes the account and (via ON DELETE CASCADE) all of its data. For
// accounts with a password, the password must be supplied and match.
func (service *AuthService) DeleteAccount(deletionContext context.Context, userIdentifier int64, password string) error {
	existingUser, lookupError := service.userRepository.FindByIdentifier(deletionContext, userIdentifier)
	if lookupError != nil {
		return lookupError
	}
	if existingUser.HasPassword() && !service.passwordService.VerifyPassword(existingUser.PasswordHash, password) {
		return ErrIncorrectPassword
	}
	return service.userRepository.DeleteUser(deletionContext, userIdentifier)
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
