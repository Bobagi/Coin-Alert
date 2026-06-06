package domain

import "strings"

const (
        BinanceEnvironmentProduction = "PRODUCTION"
        BinanceEnvironmentTestnet    = "TESTNET"
)

type BinanceEnvironmentConfiguration struct {
        EnvironmentName string
        RESTBaseURL     string
        APIKey          string
        APISecret       string
}

func NormalizeBinanceEnvironment(environmentName string) string {
        upperEnvironment := strings.ToUpper(strings.TrimSpace(environmentName))
        if upperEnvironment == BinanceEnvironmentProduction {
                return BinanceEnvironmentProduction
        }
        return BinanceEnvironmentTestnet
}
