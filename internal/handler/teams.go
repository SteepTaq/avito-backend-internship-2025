package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/SteepTaq/avito-backend-internship-2025/internal/models"
)

func (h *Handler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var team models.Team
	if err := json.NewDecoder(r.Body).Decode(&team); err != nil {
		h.logger.Info("Failed to decode request body",
			"error", err,
			"handler", "CreateTeam",
			"request_id", middleware.GetReqID(r.Context()))
		h.sendError(w, http.StatusBadRequest, models.ErrorCodeValidationError, models.ErrMsgInvalidRequestBody)
		return
	}

	if err := validateTeam(&team); err != nil {
		h.logger.Warn("Validation failed in handler",
			"error", err,
			"handler", "CreateTeam",
			"request_id", middleware.GetReqID(r.Context()))
		h.handleServiceError(w, r, err, "CreateTeam",
			"team_name", team.TeamName,
			"members_count", len(team.Members))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), RequestTimeout)
	defer cancel()

	createdTeam, err := h.service.CreateTeam(ctx, &team)
	if err != nil {
		h.handleServiceError(w, r, err, "CreateTeam",
			"team_name", team.TeamName,
			"members_count", len(team.Members))
		return
	}

	h.logger.Info("Team created successfully",
		"team_name", createdTeam.TeamName,
		"members_count", len(createdTeam.Members),
		"request_id", middleware.GetReqID(r.Context()))

	h.sendJSON(w, http.StatusCreated, map[string]*models.Team{"team": createdTeam})
}

func (h *Handler) GetTeam(w http.ResponseWriter, r *http.Request) {
	teamName := r.URL.Query().Get("team_name")
	teamName = strings.TrimSpace(teamName)

	if err := validateTeamName(teamName); err != nil {
		h.logger.Warn("Validation failed in handler",
			"error", err,
			"handler", "GetTeam",
			"request_id", middleware.GetReqID(r.Context()))
		h.sendError(w, http.StatusBadRequest, models.ErrorCodeValidationError, models.ErrMsgTeamNameRequired)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), RequestTimeout)
	defer cancel()

	team, err := h.service.GetTeam(ctx, teamName)
	if err != nil {
		h.handleServiceError(w, r, err, "GetTeam", "team_name", teamName)
		return
	}

	h.sendJSON(w, http.StatusOK, team)
}
