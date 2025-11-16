package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/SteepTaq/avito-backend-internship-2025/internal/models"
)

func (h *Handler) CreatePR(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var req models.CreatePRRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Info("Failed to decode request body",
			"error", err,
			"handler", "CreatePR",
			"request_id", middleware.GetReqID(r.Context()))
		h.sendError(w, http.StatusBadRequest, models.ErrorCodeValidationError, models.ErrMsgInvalidRequestBody)
		return
	}

	if err := validateCreatePRRequest(&req); err != nil {
		h.logger.Warn("Validation failed in handler",
			"error", err,
			"handler", "CreatePR",
			"request_id", middleware.GetReqID(r.Context()))
		h.handleServiceError(w, r, err, "CreatePR",
			"pull_request_id", req.PullRequestID,
			"author_id", req.AuthorID)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), RequestTimeout)
	defer cancel()

	pr, err := h.service.CreatePR(ctx, req.PullRequestID, req.PullRequestName, req.AuthorID)
	if err != nil {
		h.handleServiceError(w, r, err, "CreatePR",
			"pull_request_id", req.PullRequestID,
			"author_id", req.AuthorID)
		return
	}

	h.sendJSON(w, http.StatusCreated, map[string]*models.PullRequest{"pr": pr})
}

func (h *Handler) MergePR(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var req models.MergePRRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Info("Failed to decode request body",
			"error", err,
			"handler", "MergePR",
			"request_id", middleware.GetReqID(r.Context()))
		h.sendError(w, http.StatusBadRequest, models.ErrorCodeValidationError, models.ErrMsgInvalidRequestBody)
		return
	}

	if err := validateMergePRRequest(&req); err != nil {
		h.logger.Warn("Validation failed in handler",
			"error", err,
			"handler", "MergePR",
			"request_id", middleware.GetReqID(r.Context()))
		h.handleServiceError(w, r, err, "MergePR", "pull_request_id", req.PullRequestID)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), RequestTimeout)
	defer cancel()

	pr, err := h.service.MergePR(ctx, req.PullRequestID)
	if err != nil {
		h.handleServiceError(w, r, err, "MergePR", "pull_request_id", req.PullRequestID)
		return
	}

	h.sendJSON(w, http.StatusOK, map[string]*models.PullRequest{"pr": pr})
}

func (h *Handler) ReassignReviewer(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var req models.ReassignReviewerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Info("Failed to decode request body",
			"error", err,
			"handler", "ReassignReviewer",
			"request_id", middleware.GetReqID(r.Context()))
		h.sendError(w, http.StatusBadRequest, models.ErrorCodeValidationError, models.ErrMsgInvalidRequestBody)
		return
	}

	if err := validateReassignReviewerRequest(&req); err != nil {
		h.logger.Warn("Validation failed in handler",
			"error", err,
			"handler", "ReassignReviewer",
			"request_id", middleware.GetReqID(r.Context()))
		h.handleServiceError(w, r, err, "ReassignReviewer",
			"pull_request_id", req.PullRequestID,
			"old_user_id", req.OldUserID)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), RequestTimeout)
	defer cancel()

	pr, newReviewerID, err := h.service.ReassignReviewer(ctx, req.PullRequestID, req.OldUserID)
	if err != nil {
		h.handleServiceError(w, r, err, "ReassignReviewer",
			"pull_request_id", req.PullRequestID,
			"old_user_id", req.OldUserID)
		return
	}

	h.sendJSON(w, http.StatusOK, map[string]interface{}{
		"pr":          pr,
		"replaced_by": newReviewerID,
	})
}
