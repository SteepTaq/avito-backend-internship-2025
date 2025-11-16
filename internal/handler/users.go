package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/SteepTaq/avito-backend-internship-2025/internal/models"
)

func (h *Handler) SetUserActive(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var req models.SetUserActiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Info("Failed to decode request body",
			"error", err,
			"handler", "SetUserActive",
			"request_id", middleware.GetReqID(r.Context()))
		h.sendError(w, http.StatusBadRequest, models.ErrorCodeValidationError, models.ErrMsgInvalidRequestBody)
		return
	}

	if err := validateSetUserActiveRequest(&req); err != nil {
		h.logger.Warn("Validation failed in handler",
			"error", err,
			"handler", "SetUserActive",
			"request_id", middleware.GetReqID(r.Context()))
		h.handleServiceError(w, r, err, "SetUserActive",
			"user_id", req.UserID,
			"is_active", req.IsActive)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), RequestTimeout)
	defer cancel()

	user, err := h.service.SetUserActive(ctx, req.UserID, req.IsActive)
	if err != nil {
		h.handleServiceError(w, r, err, "SetUserActive",
			"user_id", req.UserID,
			"is_active", req.IsActive)
		return
	}

	h.sendJSON(w, http.StatusOK, map[string]*models.User{"user": user})
}

func (h *Handler) GetPRsByReviewer(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	userID = strings.TrimSpace(userID)

	if err := validateUserID(userID); err != nil {
		h.logger.Warn("Validation failed in handler",
			"error", err,
			"handler", "GetPRsByReviewer",
			"request_id", middleware.GetReqID(r.Context()))
		h.sendError(w, http.StatusBadRequest, models.ErrorCodeValidationError, models.ErrMsgUserIDRequired)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), RequestTimeout)
	defer cancel()

	prs, err := h.service.GetPRsByReviewer(ctx, userID)
	if err != nil {
		h.handleServiceError(w, r, err, "GetPRsByReviewer", "user_id", userID)
		return
	}

	h.sendJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":       userID,
		"pull_requests": prs,
	})
}

func (h *Handler) DeactivateTeamUsers(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var req models.DeactivateTeamUsersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Info("Failed to decode request body",
			"error", err,
			"handler", "DeactivateTeamUsers",
			"request_id", middleware.GetReqID(r.Context()))
		h.sendError(w, http.StatusBadRequest, models.ErrorCodeValidationError, models.ErrMsgInvalidRequestBody)
		return
	}

	if err := validateDeactivateTeamUsersRequest(&req); err != nil {
		h.logger.Warn("Validation failed in handler",
			"error", err,
			"handler", "DeactivateTeamUsers",
			"request_id", middleware.GetReqID(r.Context()))
		h.handleServiceError(w, r, err, "DeactivateTeamUsers",
			"team_name", req.TeamName,
			"user_ids_count", len(req.UserIDs))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), RequestTimeout)
	defer cancel()

	result, err := h.service.DeactivateTeamUsers(ctx, req.TeamName, req.UserIDs)
	if err != nil {
		h.handleServiceError(w, r, err, "DeactivateTeamUsers",
			"team_name", req.TeamName,
			"user_ids_count", len(req.UserIDs))
		return
	}

	h.sendJSON(w, http.StatusOK, result)
}
