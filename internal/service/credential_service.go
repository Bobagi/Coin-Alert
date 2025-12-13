package service

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"coin-alert/internal/repository"
)

type CredentialService struct {
	BinanceAPIKey            string
	BinanceAPISecret         string
	credentialsValidated     bool
	credentialsSupplied      bool
	credentialRepository     repository.BinanceCredentialRepository
	credentialValidator      *BinanceCredentialValidator
	defaultValidationTimeout time.Duration
}

func NewCredentialService(repositoryInstance repository.BinanceCredentialRepository, validator *BinanceCredentialValidator, initialAPIKey string, initialAPISecret string) *CredentialService {
	return &CredentialService{
		BinanceAPIKey:            initialAPIKey,
		BinanceAPISecret:         initialAPISecret,
		credentialsValidated:     false,
		credentialsSupplied:      false,
		credentialRepository:     repositoryInstance,
		credentialValidator:      validator,
		defaultValidationTimeout: 8 * time.Second,
	}
}

func (service *CredentialService) InitializeCredentials(initializationContext context.Context) {
	repositoryContext, cancel := context.WithTimeout(initializationContext, service.defaultValidationTimeout)
	defer cancel()

	storedAPIKey, storedAPISecret, loadCredentialsError := service.credentialRepository.LoadLatestCredentials(repositoryContext)
	if loadCredentialsError != nil {
		log.Printf("Could not load saved credentials: %v", loadCredentialsError)
	}

	if strings.TrimSpace(storedAPIKey) != "" && strings.TrimSpace(storedAPISecret) != "" {
		service.credentialsSupplied = true
		validationError := service.validateAndSet(repositoryContext, storedAPIKey, storedAPISecret)
		if validationError == nil {
			return
		}
		log.Printf("Saved credentials are invalid: %v", validationError)
	}

	if strings.TrimSpace(service.BinanceAPIKey) == "" || strings.TrimSpace(service.BinanceAPISecret) == "" {
		service.credentialsValidated = false
		return
	}

	service.credentialsSupplied = true
	validationError := service.ValidateAndPersistCredentials(initializationContext, service.BinanceAPIKey, service.BinanceAPISecret)
	if validationError != nil {
		log.Printf("Environment credentials are invalid: %v", validationError)
	}
}

func (service *CredentialService) ValidateAndPersistCredentials(operationContext context.Context, updatedAPIKey string, updatedAPISecret string) error {
	validationContext, cancel := context.WithTimeout(operationContext, service.defaultValidationTimeout)
	defer cancel()

	if service.credentialValidator == nil {
		return errors.New("Binance credential validator is not configured")
	}

	validationError := service.credentialValidator.ValidateCredentials(validationContext, updatedAPIKey, updatedAPISecret)
	if validationError != nil {
		service.credentialsValidated = false
		service.credentialsSupplied = true
		return validationError
	}

	repositoryError := service.credentialRepository.SaveCredentials(validationContext, updatedAPIKey, updatedAPISecret)
	if repositoryError != nil {
		service.credentialsValidated = false
		service.credentialsSupplied = true
		return repositoryError
	}

	service.BinanceAPIKey = updatedAPIKey
	service.BinanceAPISecret = updatedAPISecret
	service.credentialsValidated = true
	service.credentialsSupplied = true
	return nil
}

func (service *CredentialService) HasValidBinanceCredentials() bool {
	return service.credentialsValidated
}

func (service *CredentialService) HasSuppliedBinanceCredentials() bool {
	if service.credentialsSupplied {
		return true
	}

	return strings.TrimSpace(service.BinanceAPIKey) != "" && strings.TrimSpace(service.BinanceAPISecret) != ""
}

func (service *CredentialService) GetMaskedBinanceAPIKey() string {
	if !service.credentialsValidated {
		return ""
	}

	if len(service.BinanceAPIKey) <= 4 {
		return "****"
	}

	trailingCharacters := service.BinanceAPIKey[len(service.BinanceAPIKey)-4:]
	return "****" + trailingCharacters
}

func (service *CredentialService) GetMaskedBinanceAPISecret() string {
	if !service.credentialsValidated {
		return ""
	}

	if len(service.BinanceAPISecret) <= 4 {
		return "****"
	}

	trailingCharacters := service.BinanceAPISecret[len(service.BinanceAPISecret)-4:]
	return "****" + trailingCharacters
}

func (service *CredentialService) validateAndSet(validationContext context.Context, apiKey string, apiSecret string) error {
	validationError := service.credentialValidator.ValidateCredentials(validationContext, apiKey, apiSecret)
	if validationError != nil {
		service.credentialsValidated = false
		return validationError
	}

	service.BinanceAPIKey = apiKey
	service.BinanceAPISecret = apiSecret
	service.credentialsValidated = true
	return nil
}
