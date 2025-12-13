package service

type CredentialService struct {
    BinanceAPIKey    string
    BinanceAPISecret string
}

func NewCredentialService(initialAPIKey string, initialAPISecret string) *CredentialService {
    return &CredentialService{
        BinanceAPIKey:    initialAPIKey,
        BinanceAPISecret: initialAPISecret,
    }
}

func (service *CredentialService) UpdateBinanceCredentials(updatedAPIKey string, updatedAPISecret string) {
    service.BinanceAPIKey = updatedAPIKey
    service.BinanceAPISecret = updatedAPISecret
}

func (service *CredentialService) HasValidBinanceCredentials() bool {
    return service.isValueProvided(service.BinanceAPIKey) && service.isValueProvided(service.BinanceAPISecret)
}

func (service *CredentialService) GetMaskedBinanceAPIKey() string {
    if !service.isValueProvided(service.BinanceAPIKey) {
        return ""
    }

    if len(service.BinanceAPIKey) <= 4 {
        return "****"
    }

    trailingCharacters := service.BinanceAPIKey[len(service.BinanceAPIKey)-4:]
    return "****" + trailingCharacters
}

func (service *CredentialService) GetMaskedBinanceAPISecret() string {
    if !service.isValueProvided(service.BinanceAPISecret) {
        return ""
    }

    if len(service.BinanceAPISecret) <= 4 {
        return "****"
    }

    trailingCharacters := service.BinanceAPISecret[len(service.BinanceAPISecret)-4:]
    return "****" + trailingCharacters
}

func (service *CredentialService) isValueProvided(value string) bool {
    return value != "" && value != "your_password_here" && value != "changeme"
}
