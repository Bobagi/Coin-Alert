package service

import (
	"context"
	"errors"

	"coin-alert/internal/domain"
	"coin-alert/internal/repository"
	"coin-alert/internal/security"
)

// ErrCredentialEncryptionUnavailable is returned when the server has no encryption key configured.
var ErrCredentialEncryptionUnavailable = errors.New("credential encryption is not configured on the server")

// CredentialStatus is a non-sensitive summary of a user's stored Binance credentials.
type CredentialStatus struct {
	HasActiveCredential    bool
	ActiveEnvironment      string
	MaskedAPIKey           string
	ConfiguredEnvironments []string
}

// UserCredentialService validates, encrypts, stores, and retrieves per-user Binance credentials.
type UserCredentialService struct {
	repository        repository.UserBinanceCredentialRepository
	cipher            *security.SecretCipher
	testnetBaseURL    string
	productionBaseURL string
}

func NewUserCredentialService(repositoryInstance repository.UserBinanceCredentialRepository, cipher *security.SecretCipher, testnetBaseURL string, productionBaseURL string) *UserCredentialService {
	return &UserCredentialService{
		repository:        repositoryInstance,
		cipher:            cipher,
		testnetBaseURL:    testnetBaseURL,
		productionBaseURL: productionBaseURL,
	}
}

func (service *UserCredentialService) baseURLForEnvironment(environmentName string) string {
	if domain.NormalizeBinanceEnvironment(environmentName) == domain.BinanceEnvironmentProduction {
		return service.productionBaseURL
	}
	return service.testnetBaseURL
}

// SaveAndValidate validates the keys against Binance, then stores them encrypted as the active credential.
func (service *UserCredentialService) SaveAndValidate(operationContext context.Context, userIdentifier int64, apiKey string, apiSecret string, environmentName string) error {
	if service.cipher == nil {
		return ErrCredentialEncryptionUnavailable
	}

	normalizedEnvironment := domain.NormalizeBinanceEnvironment(environmentName)
	baseURL := service.baseURLForEnvironment(normalizedEnvironment)

	// A fresh validator per call keeps this safe under concurrent requests.
	validator := NewBinanceCredentialValidator(baseURL)
	if validationError := validator.ValidateCredentials(operationContext, apiKey, apiSecret); validationError != nil {
		return validationError
	}

	encryptedAPIKey, keyEncryptionError := service.cipher.EncryptString(apiKey)
	if keyEncryptionError != nil {
		return keyEncryptionError
	}
	encryptedAPISecret, secretEncryptionError := service.cipher.EncryptString(apiSecret)
	if secretEncryptionError != nil {
		return secretEncryptionError
	}

	return service.repository.SaveCredentialForUser(operationContext, userIdentifier, domain.BinanceCredentialRecord{
		APIKey:          encryptedAPIKey,
		APISecret:       encryptedAPISecret,
		EnvironmentName: normalizedEnvironment,
		APIBaseURL:      baseURL,
		IsActive:        true,
	})
}

// LoadActiveEnvironmentConfiguration returns the decrypted active credential ready for the Binance
// clients, or (nil, nil) when the user has none stored.
func (service *UserCredentialService) LoadActiveEnvironmentConfiguration(operationContext context.Context, userIdentifier int64) (*domain.BinanceEnvironmentConfiguration, error) {
	record, loadError := service.repository.LoadActiveCredentialForUser(operationContext, userIdentifier)
	if loadError != nil {
		return nil, loadError
	}
	if record == nil {
		return nil, nil
	}
	if service.cipher == nil {
		return nil, ErrCredentialEncryptionUnavailable
	}

	apiKey, keyDecryptionError := service.cipher.DecryptString(record.APIKey)
	if keyDecryptionError != nil {
		return nil, keyDecryptionError
	}
	apiSecret, secretDecryptionError := service.cipher.DecryptString(record.APISecret)
	if secretDecryptionError != nil {
		return nil, secretDecryptionError
	}

	return &domain.BinanceEnvironmentConfiguration{
		EnvironmentName: record.EnvironmentName,
		RESTBaseURL:     record.APIBaseURL,
		APIKey:          apiKey,
		APISecret:       apiSecret,
	}, nil
}

// ActivateEnvironment switches the user's active environment to one they already have keys for.
func (service *UserCredentialService) ActivateEnvironment(operationContext context.Context, userIdentifier int64, environmentName string) error {
	normalizedEnvironment := domain.NormalizeBinanceEnvironment(environmentName)
	existing, loadError := service.repository.LoadLatestCredentialForUserByEnvironment(operationContext, userIdentifier, normalizedEnvironment)
	if loadError != nil {
		return loadError
	}
	if existing == nil {
		return errors.New("no Binance credentials are stored for the selected environment")
	}
	return service.repository.ActivateEnvironmentForUser(operationContext, userIdentifier, normalizedEnvironment)
}

// ActiveEnvironmentName returns the user's active Binance environment, defaulting to TESTNET when the
// user has not connected any credentials yet. Used to scope settings/operations to one environment.
func (service *UserCredentialService) ActiveEnvironmentName(operationContext context.Context, userIdentifier int64) string {
	record, loadError := service.repository.LoadActiveCredentialForUser(operationContext, userIdentifier)
	if loadError == nil && record != nil {
		return record.EnvironmentName
	}
	return domain.BinanceEnvironmentTestnet
}

// GetStatus returns a non-sensitive summary suitable for the dashboard.
func (service *UserCredentialService) GetStatus(operationContext context.Context, userIdentifier int64) (CredentialStatus, error) {
	configuredEnvironments, listError := service.repository.ListConfiguredEnvironmentsForUser(operationContext, userIdentifier)
	if listError != nil {
		return CredentialStatus{}, listError
	}

	status := CredentialStatus{ConfiguredEnvironments: configuredEnvironments}

	record, loadError := service.repository.LoadActiveCredentialForUser(operationContext, userIdentifier)
	if loadError != nil {
		return CredentialStatus{}, loadError
	}
	if record == nil {
		return status, nil
	}

	status.HasActiveCredential = true
	status.ActiveEnvironment = record.EnvironmentName
	status.MaskedAPIKey = service.maskAPIKey(record.APIKey)
	return status, nil
}

func (service *UserCredentialService) maskAPIKey(encryptedAPIKey string) string {
	if service.cipher == nil {
		return "****"
	}
	apiKey, decryptionError := service.cipher.DecryptString(encryptedAPIKey)
	if decryptionError != nil || len(apiKey) < 4 {
		return "****"
	}
	return "****" + apiKey[len(apiKey)-4:]
}
