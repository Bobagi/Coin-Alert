package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"coin-alert/internal/domain"
	"coin-alert/internal/repository"
	"coin-alert/internal/service"
)

// RobotsHandler serves the per-user trading-robot endpoints. A robot is one automated bot for a
// single coin; standard users may have one per environment, admins unlimited.
type RobotsHandler struct {
	sessionService *service.SessionService
	authService    *service.AuthService
	cookieName     string
	robotService   *service.RobotService
}

func NewRobotsHandler(sessionService *service.SessionService, authService *service.AuthService, cookieName string, robotService *service.RobotService) *RobotsHandler {
	return &RobotsHandler{
		sessionService: sessionService,
		authService:    authService,
		cookieName:     cookieName,
		robotService:   robotService,
	}
}

func (handler *RobotsHandler) RegisterRoutes(router *http.ServeMux) {
	router.HandleFunc("/api/v1/robots", handler.handleRobots)
	router.HandleFunc("/api/v1/robots/update", handler.handleUpdate)
	router.HandleFunc("/api/v1/robots/delete", handler.handleDelete)
}

// resolveUser returns the authenticated user (including the is_admin flag), or writes a 401.
func (handler *RobotsHandler) resolveUser(responseWriter http.ResponseWriter, request *http.Request) (*domain.User, bool) {
	sessionCookie, cookieError := request.Cookie(handler.cookieName)
	if cookieError != nil {
		writeJSONError(responseWriter, http.StatusUnauthorized, "Not authenticated.")
		return nil, false
	}
	resolveContext, cancel := context.WithTimeout(request.Context(), 5*time.Second)
	defer cancel()
	userIdentifier, resolveError := handler.sessionService.ResolveUserIdentifier(resolveContext, sessionCookie.Value)
	if resolveError != nil {
		writeJSONError(responseWriter, http.StatusUnauthorized, "Not authenticated.")
		return nil, false
	}
	currentUser, lookupError := handler.authService.GetUserByIdentifier(resolveContext, userIdentifier)
	if lookupError != nil || currentUser == nil {
		writeJSONError(responseWriter, http.StatusUnauthorized, "Not authenticated.")
		return nil, false
	}
	return currentUser, true
}

type robotPayload struct {
	ID                    int64    `json:"id"`
	Symbol                string   `json:"symbol"`
	Name                  string   `json:"name"`
	CapitalThreshold      float64  `json:"capital_threshold"`
	TargetProfitPercent   float64  `json:"target_profit_percent"`
	StopLossPercent       *float64 `json:"stop_loss_percent"`
	DailyPurchaseHourUTC  int      `json:"daily_purchase_hour_utc"`
	DailyPurchaseEnabled  bool     `json:"daily_purchase_enabled"`
	SellOrderValidityDays int      `json:"sell_order_validity_days"`
	IsEnabled             bool     `json:"is_enabled"`
}

type robotInputPayload struct {
	ID                    int64    `json:"id"`
	Symbol                string   `json:"symbol"`
	Name                  string   `json:"name"`
	CapitalThreshold      float64  `json:"capital_threshold"`
	TargetProfitPercent   float64  `json:"target_profit_percent"`
	StopLossPercent       *float64 `json:"stop_loss_percent"`
	DailyPurchaseHourUTC  int      `json:"daily_purchase_hour_utc"`
	DailyPurchaseEnabled  bool     `json:"daily_purchase_enabled"`
	SellOrderValidityDays int      `json:"sell_order_validity_days"`
	IsEnabled             bool     `json:"is_enabled"`
}

func (payload robotInputPayload) toServiceInput() service.RobotInput {
	return service.RobotInput{
		TradingPairSymbol:     payload.Symbol,
		Name:                  payload.Name,
		CapitalThreshold:      payload.CapitalThreshold,
		TargetProfitPercent:   payload.TargetProfitPercent,
		StopLossPercent:       payload.StopLossPercent,
		DailyPurchaseHourUTC:  payload.DailyPurchaseHourUTC,
		DailyPurchaseEnabled:  payload.DailyPurchaseEnabled,
		SellOrderValidityDays: payload.SellOrderValidityDays,
		IsEnabled:             payload.IsEnabled,
	}
}

func (handler *RobotsHandler) handleRobots(responseWriter http.ResponseWriter, request *http.Request) {
	currentUser, authenticated := handler.resolveUser(responseWriter, request)
	if !authenticated {
		return
	}

	switch request.Method {
	case http.MethodGet:
		operationContext, cancel := context.WithTimeout(request.Context(), 6*time.Second)
		defer cancel()
		robots, listError := handler.robotService.ListRobots(operationContext, currentUser.Identifier)
		if listError != nil {
			writeJSONError(responseWriter, http.StatusInternalServerError, "Could not load robots.")
			return
		}
		writeJSON(responseWriter, http.StatusOK, map[string]interface{}{
			"robots":   toRobotPayloads(robots),
			"limit":    service.RobotLimitForAdmin(currentUser.IsAdmin), // 0 = unlimited
			"is_admin": currentUser.IsAdmin,
		})

	case http.MethodPost:
		var payload robotInputPayload
		if decodeError := json.NewDecoder(request.Body).Decode(&payload); decodeError != nil {
			writeJSONError(responseWriter, http.StatusBadRequest, "Invalid request body.")
			return
		}
		operationContext, cancel := context.WithTimeout(request.Context(), 6*time.Second)
		defer cancel()
		if !currentUser.IsEmailVerified() {
			writeJSONErrorCode(responseWriter, http.StatusForbidden, "Confirm your email before using this feature.", "email_unverified")
			return
		}
		robot, createError := handler.robotService.CreateRobot(operationContext, currentUser.Identifier, currentUser.IsAdmin, payload.toServiceInput())
		if createError != nil {
			handler.writeRobotError(responseWriter, createError)
			return
		}
		writeJSON(responseWriter, http.StatusOK, toRobotPayload(*robot))

	default:
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (handler *RobotsHandler) handleUpdate(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	currentUser, authenticated := handler.resolveUser(responseWriter, request)
	if !authenticated {
		return
	}

	var payload robotInputPayload
	if decodeError := json.NewDecoder(request.Body).Decode(&payload); decodeError != nil {
		writeJSONError(responseWriter, http.StatusBadRequest, "Invalid request body.")
		return
	}
	if payload.ID <= 0 {
		writeJSONError(responseWriter, http.StatusBadRequest, "A robot id is required.")
		return
	}

	operationContext, cancel := context.WithTimeout(request.Context(), 6*time.Second)
	defer cancel()
	if !currentUser.IsEmailVerified() {
		writeJSONErrorCode(responseWriter, http.StatusForbidden, "Confirm your email before using this feature.", "email_unverified")
		return
	}
	robot, updateError := handler.robotService.UpdateRobot(operationContext, currentUser.Identifier, payload.ID, payload.toServiceInput())
	if updateError != nil {
		handler.writeRobotError(responseWriter, updateError)
		return
	}
	writeJSON(responseWriter, http.StatusOK, toRobotPayload(*robot))
}

func (handler *RobotsHandler) handleDelete(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	currentUser, authenticated := handler.resolveUser(responseWriter, request)
	if !authenticated {
		return
	}

	var payload struct {
		ID int64 `json:"id"`
	}
	if decodeError := json.NewDecoder(request.Body).Decode(&payload); decodeError != nil {
		writeJSONError(responseWriter, http.StatusBadRequest, "Invalid request body.")
		return
	}
	if payload.ID <= 0 {
		writeJSONError(responseWriter, http.StatusBadRequest, "A robot id is required.")
		return
	}

	operationContext, cancel := context.WithTimeout(request.Context(), 6*time.Second)
	defer cancel()
	if deleteError := handler.robotService.DeleteRobot(operationContext, currentUser.Identifier, payload.ID); deleteError != nil {
		handler.writeRobotError(responseWriter, deleteError)
		return
	}
	writeJSON(responseWriter, http.StatusOK, map[string]string{"message": "Robot deleted."})
}

func (handler *RobotsHandler) writeRobotError(responseWriter http.ResponseWriter, robotError error) {
	switch {
	case errors.Is(robotError, service.ErrRobotLimitReached):
		writeJSONError(responseWriter, http.StatusForbidden, robotError.Error())
	case errors.Is(robotError, service.ErrRobotSymbolExists):
		writeJSONError(responseWriter, http.StatusConflict, robotError.Error())
	case errors.Is(robotError, repository.ErrRobotNotFound):
		writeJSONError(responseWriter, http.StatusNotFound, "Robot not found.")
	default:
		writeJSONError(responseWriter, http.StatusInternalServerError, "Could not save the robot.")
	}
}

func toRobotPayload(robot domain.TradingRobot) robotPayload {
	return robotPayload{
		ID:                    robot.Identifier,
		Symbol:                robot.TradingPairSymbol,
		Name:                  robot.Name,
		CapitalThreshold:      robot.CapitalThreshold,
		TargetProfitPercent:   robot.TargetProfitPercent,
		StopLossPercent:       robot.StopLossPercent,
		DailyPurchaseHourUTC:  robot.DailyPurchaseHourUTC,
		DailyPurchaseEnabled:  robot.DailyPurchaseEnabled,
		SellOrderValidityDays: robot.SellOrderValidityDays,
		IsEnabled:             robot.IsEnabled,
	}
}

func toRobotPayloads(robots []domain.TradingRobot) []robotPayload {
	payloads := make([]robotPayload, 0, len(robots))
	for _, robot := range robots {
		payloads = append(payloads, toRobotPayload(robot))
	}
	return payloads
}
