package service

import (
        "context"
        "errors"
        "log"
        "strings"
        "time"

        "coin-alert/internal/domain"
        "coin-alert/internal/repository"
)

type CredentialService struct {
        BinanceAPIKey                    string
        BinanceAPISecret                 string
        ActiveEnvironmentConfiguration   domain.BinanceEnvironmentConfiguration
        credentialsValidated             bool
        credentialsSupplied              bool
        credentialRepository             repository.BinanceCredentialRepository
        credentialValidator              *BinanceCredentialValidator
        defaultValidationTimeout         time.Duration
        defaultEnvironmentConfiguration  domain.BinanceEnvironmentConfiguration
}

func NewCredentialService(repositoryInstance repository.BinanceCredentialRepository, validator *BinanceCredentialValidator, initialConfiguration domain.BinanceEnvironmentConfiguration) *CredentialService {
        return &CredentialService{
                BinanceAPIKey:                   initialConfiguration.APIKey,
                BinanceAPISecret:                initialConfiguration.APISecret,
                ActiveEnvironmentConfiguration:  initialConfiguration,
                credentialsValidated:            false,
                credentialsSupplied:             false,
                credentialRepository:            repositoryInstance,
                credentialValidator:             validator,
                defaultValidationTimeout:        8 * time.Second,
                defaultEnvironmentConfiguration: initialConfiguration,
        }
}

func (service *CredentialService) InitializeCredentials(initializationContext context.Context) {
        repositoryContext, cancel := context.WithTimeout(initializationContext, service.defaultValidationTimeout)
        defer cancel()

        storedCredentials, loadCredentialsError := service.credentialRepository.LoadActiveCredentials(repositoryContext)
        if loadCredentialsError != nil {
                log.Printf("Could not load saved credentials: %v", loadCredentialsError)
        }

        if storedCredentials != nil {
                service.credentialsSupplied = true
                activeEnvironment := service.buildEnvironmentConfigurationFromRecord(*storedCredentials)
                service.ActiveEnvironmentConfiguration = activeEnvironment
                service.credentialValidator.UpdateAPIBaseURL(activeEnvironment.RESTBaseURL)

                validationError := service.validateAndSet(repositoryContext, storedCredentials.APIKey, storedCredentials.APISecret)
                if validationError == nil {
                        return
                }
                log.Printf("Saved credentials are invalid: %v", validationError)
        }

        if strings.TrimSpace(service.BinanceAPIKey) == "" || strings.TrimSpace(service.BinanceAPISecret) == "" {
                service.credentialsValidated = false
                return
        }

        environmentName := domain.NormalizeBinanceEnvironment(service.defaultEnvironmentConfiguration.EnvironmentName)
        validationError := service.ValidateAndPersistCredentials(initializationContext, service.BinanceAPIKey, service.BinanceAPISecret, environmentName)
        if validationError != nil {
                log.Printf("Environment credentials are invalid: %v", validationError)
        }
}

func (service *CredentialService) ValidateAndPersistCredentials(operationContext context.Context, updatedAPIKey string, updatedAPISecret string, environmentName string) error {
        validationContext, cancel := context.WithTimeout(operationContext, service.defaultValidationTimeout)
        defer cancel()

        if service.credentialValidator == nil {
                return errors.New("Binance credential validator is not configured")
        }

        normalizedEnvironment := domain.NormalizeBinanceEnvironment(environmentName)
        environmentConfiguration := service.buildEnvironmentConfiguration(normalizedEnvironment, updatedAPIKey, updatedAPISecret)
        service.credentialValidator.UpdateAPIBaseURL(environmentConfiguration.RESTBaseURL)

        validationError := service.credentialValidator.ValidateCredentials(validationContext, updatedAPIKey, updatedAPISecret)
        if validationError != nil {
                service.credentialsValidated = false
                service.credentialsSupplied = true
                return validationError
        }

        repositoryError := service.credentialRepository.SaveCredentials(validationContext, domain.BinanceCredentialRecord{
                APIKey:          updatedAPIKey,
                APISecret:       updatedAPISecret,
                EnvironmentName: normalizedEnvironment,
                APIBaseURL:      environmentConfiguration.RESTBaseURL,
                IsActive:        true,
        })
        if repositoryError != nil {
                service.credentialsValidated = false
                service.credentialsSupplied = true
                return repositoryError
        }

        service.BinanceAPIKey = updatedAPIKey
        service.BinanceAPISecret = updatedAPISecret
        service.ActiveEnvironmentConfiguration = environmentConfiguration
        service.credentialsValidated = true
        service.credentialsSupplied = true
        return nil
}

func (service *CredentialService) ActivateEnvironment(operationContext context.Context, environmentName string) error {
        activationContext, cancel := context.WithTimeout(operationContext, service.defaultValidationTimeout)
        defer cancel()

        normalizedEnvironment := domain.NormalizeBinanceEnvironment(environmentName)
        storedRecord, loadError := service.credentialRepository.LoadLatestCredentialsByEnvironment(activationContext, normalizedEnvironment)
        if loadError != nil {
                return loadError
        }
        if storedRecord == nil {
                return errors.New("No credentials stored for the selected Binance environment")
        }

        candidateConfiguration := service.buildEnvironmentConfigurationFromRecord(*storedRecord)
        service.credentialValidator.UpdateAPIBaseURL(candidateConfiguration.RESTBaseURL)
        validationError := service.credentialValidator.ValidateCredentials(activationContext, storedRecord.APIKey, storedRecord.APISecret)
        if validationError != nil {
                service.credentialsValidated = false
                return validationError
        }

        activationError := service.credentialRepository.ActivateEnvironment(activationContext, normalizedEnvironment)
        if activationError != nil {
                return activationError
        }

        service.ActiveEnvironmentConfiguration = candidateConfiguration
        service.BinanceAPIKey = storedRecord.APIKey
        service.BinanceAPISecret = storedRecord.APISecret
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
        if !service.HasSuppliedBinanceCredentials() {
                return ""
        }

        return "********"
}

func (service *CredentialService) GetActiveEnvironmentConfiguration() domain.BinanceEnvironmentConfiguration {
        return service.ActiveEnvironmentConfiguration
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

func (service *CredentialService) RevalidateStoredCredentials(operationContext context.Context) error {
        if strings.TrimSpace(service.BinanceAPIKey) == "" || strings.TrimSpace(service.BinanceAPISecret) == "" {
                service.credentialsValidated = false
                service.credentialsSupplied = false
                return errors.New("Binance credentials are missing. Please provide both API Key and Secret Key before revalidating.")
        }

        validationContext, cancel := context.WithTimeout(operationContext, service.defaultValidationTimeout)
        defer cancel()

        service.credentialValidator.UpdateAPIBaseURL(service.ActiveEnvironmentConfiguration.RESTBaseURL)
        validationError := service.credentialValidator.ValidateCredentials(validationContext, service.BinanceAPIKey, service.BinanceAPISecret)
        if validationError != nil {
                service.credentialsValidated = false
                service.credentialsSupplied = true
                return validationError
        }

        service.credentialsValidated = true
        service.credentialsSupplied = true
        return nil
}

func (service *CredentialService) buildEnvironmentConfiguration(environmentName string, apiKey string, apiSecret string) domain.BinanceEnvironmentConfiguration {
        baseURL := service.defaultEnvironmentConfiguration.RESTBaseURL
        if domain.NormalizeBinanceEnvironment(environmentName) == domain.BinanceEnvironmentProduction {
                baseURL = "https://api.binance.com"
        } else {
                baseURL = "https://testnet.binance.vision"
        }

        return domain.BinanceEnvironmentConfiguration{
                EnvironmentName: domain.NormalizeBinanceEnvironment(environmentName),
                RESTBaseURL:     baseURL,
                APIKey:          apiKey,
                APISecret:       apiSecret,
        }
}

func (service *CredentialService) buildEnvironmentConfigurationFromRecord(record domain.BinanceCredentialRecord) domain.BinanceEnvironmentConfiguration {
        environment := domain.NormalizeBinanceEnvironment(record.EnvironmentName)
        baseURL := record.APIBaseURL
        if strings.TrimSpace(baseURL) == "" {
                baseURL = service.buildEnvironmentConfiguration(environment, record.APIKey, record.APISecret).RESTBaseURL
        }

        return domain.BinanceEnvironmentConfiguration{
                EnvironmentName: environment,
                RESTBaseURL:     baseURL,
                APIKey:          record.APIKey,
                APISecret:       record.APISecret,
        }
}
