package service

import (
	"context"
	"errors"
	"strings"

	"coin-alert/internal/domain"
	"coin-alert/internal/repository"
)

// ErrRobotLimitReached is returned when a non-admin tries to exceed their robot allowance.
var ErrRobotLimitReached = errors.New("you have reached your robot limit — more robots are a paid upgrade")

// ErrRobotSymbolExists is returned when a robot already exists for that coin in the environment.
var ErrRobotSymbolExists = errors.New("you already have a robot for this coin in this environment")

// StandardUserRobotLimitPerEnvironment is how many robots a non-admin user may have per Binance
// environment. Admins are unlimited (monetization hook: extra robots become a paid upgrade later).
const StandardUserRobotLimitPerEnvironment = 1

// RobotService manages a user's trading robots, scoped to their active Binance environment, and
// enforces the per-plan robot limit on creation.
type RobotService struct {
	repository        repository.TradingRobotRepository
	credentialService *UserCredentialService
}

func NewRobotService(repositoryInstance repository.TradingRobotRepository, credentialService *UserCredentialService) *RobotService {
	return &RobotService{repository: repositoryInstance, credentialService: credentialService}
}

// RobotInput carries the editable robot fields coming from the API.
type RobotInput struct {
	TradingPairSymbol     string
	Name                  string
	CapitalThreshold      float64
	TargetProfitPercent   float64
	StopLossPercent       *float64
	DailyPurchaseHourUTC  int
	DailyPurchaseEnabled  bool
	SellOrderValidityDays int
	IsEnabled             bool
}

// RobotLimitForAdmin returns the per-environment robot limit for a user; 0 means unlimited.
func RobotLimitForAdmin(isAdmin bool) int {
	if isAdmin {
		return 0
	}
	return StandardUserRobotLimitPerEnvironment
}

func (service *RobotService) ListRobots(operationContext context.Context, userIdentifier int64) ([]domain.TradingRobot, error) {
	environment := service.credentialService.ActiveEnvironmentName(operationContext, userIdentifier)
	return service.repository.ListRobotsForUser(operationContext, userIdentifier, environment)
}

func (service *RobotService) CreateRobot(operationContext context.Context, userIdentifier int64, isAdmin bool, input RobotInput) (*domain.TradingRobot, error) {
	environment := service.credentialService.ActiveEnvironmentName(operationContext, userIdentifier)
	if !isAdmin {
		robotCount, countError := service.repository.CountRobotsForUser(operationContext, userIdentifier, environment)
		if countError != nil {
			return nil, countError
		}
		if robotCount >= StandardUserRobotLimitPerEnvironment {
			return nil, ErrRobotLimitReached
		}
	}

	robot := normalizeRobot(input, environment)
	robotIdentifier, createError := service.repository.CreateRobotForUser(operationContext, userIdentifier, robot)
	if createError != nil {
		if errors.Is(createError, repository.ErrRobotSymbolExists) {
			return nil, ErrRobotSymbolExists
		}
		return nil, createError
	}
	robot.Identifier = robotIdentifier
	return &robot, nil
}

func (service *RobotService) UpdateRobot(operationContext context.Context, userIdentifier int64, robotIdentifier int64, input RobotInput) (*domain.TradingRobot, error) {
	existing, lookupError := service.repository.GetRobotForUser(operationContext, userIdentifier, robotIdentifier)
	if lookupError != nil {
		return nil, lookupError
	}

	robot := normalizeRobot(input, existing.BinanceEnvironment)
	robot.Identifier = robotIdentifier
	// The coin is immutable after creation (it is part of the robot's identity within the environment).
	robot.TradingPairSymbol = existing.TradingPairSymbol
	if updateError := service.repository.UpdateRobotForUser(operationContext, userIdentifier, robot); updateError != nil {
		return nil, updateError
	}
	return &robot, nil
}

func (service *RobotService) DeleteRobot(operationContext context.Context, userIdentifier int64, robotIdentifier int64) error {
	return service.repository.DeleteRobotForUser(operationContext, userIdentifier, robotIdentifier)
}

func normalizeRobot(input RobotInput, environment string) domain.TradingRobot {
	symbol := strings.ToUpper(strings.TrimSpace(input.TradingPairSymbol))
	if symbol == "" {
		symbol = "BTCUSDT"
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = symbol
	}
	targetProfitPercent := input.TargetProfitPercent
	if targetProfitPercent <= 0 {
		targetProfitPercent = 1.0
	}
	dailyHour := input.DailyPurchaseHourUTC
	if dailyHour < 0 || dailyHour > 23 {
		dailyHour = 4
	}
	validityDays := input.SellOrderValidityDays
	if validityDays < 0 {
		validityDays = 0
	}
	if validityDays > 365 {
		validityDays = 365
	}
	capital := input.CapitalThreshold
	if capital < 0 {
		capital = 0
	}
	var stopLossPercent *float64
	if input.StopLossPercent != nil && *input.StopLossPercent > 0 {
		value := *input.StopLossPercent
		stopLossPercent = &value
	}

	return domain.TradingRobot{
		BinanceEnvironment:    environment,
		TradingPairSymbol:     symbol,
		Name:                  name,
		CapitalThreshold:      capital,
		TargetProfitPercent:   targetProfitPercent,
		StopLossPercent:       stopLossPercent,
		DailyPurchaseHourUTC:  dailyHour,
		DailyPurchaseEnabled:  input.DailyPurchaseEnabled,
		SellOrderValidityDays: validityDays,
		IsEnabled:             input.IsEnabled,
	}
}
