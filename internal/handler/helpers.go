package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	apperrors "github.com/SteepTaq/avito-backend-internship-2025/internal/errors"
	"github.com/SteepTaq/avito-backend-internship-2025/internal/models"
)

// таймаут для операций с БД
const RequestTimeout = 10 * time.Second

// handleServiceError обрабатывает ошибки сервиса и отправляет соответствующий HTTP ответ
func (h *Handler) handleServiceError(w http.ResponseWriter, r *http.Request, err error, handlerName string, contextFields ...interface{}) {
	reqID := middleware.GetReqID(r.Context())
	baseFields := []interface{}{
		"handler", handlerName,
		"request_id", reqID,
		"method", r.Method,
		"path", r.URL.Path,
	}
	baseFields = append(baseFields, contextFields...)

	if errors.Is(err, apperrors.ErrTeamNameRequired) || errors.Is(err, apperrors.ErrTeamMustHaveMembers) ||
		errors.Is(err, apperrors.ErrStringTooLong) || errors.Is(err, apperrors.ErrUsernameRequired) ||
		errors.Is(err, apperrors.ErrDuplicateUserInTeam) {
		h.logger.Warn("Validation error", append(baseFields, "error", err.Error())...)
		h.sendError(w, http.StatusBadRequest, models.ErrorCodeValidationError, err.Error())
		return
	}
	if errors.Is(err, apperrors.ErrPRIDRequired) || errors.Is(err, apperrors.ErrPRNameRequired) || errors.Is(err, apperrors.ErrAuthorIDRequired) {
		h.logger.Warn("Validation error", append(baseFields, "error", err.Error())...)
		h.sendError(w, http.StatusBadRequest, models.ErrorCodeValidationError, err.Error())
		return
	}

	if errors.Is(err, apperrors.ErrTeamExists) {
		h.sendError(w, http.StatusBadRequest, models.ErrorCodeTeamExists, models.ErrMsgTeamNameAlreadyExists)
		return
	}
	if errors.Is(err, apperrors.ErrTeamNotFound) {
		h.sendError(w, http.StatusNotFound, models.ErrorCodeNotFound, models.ErrMsgResourceNotFound)
		return
	}
	if errors.Is(err, apperrors.ErrUserNotFound) {
		h.sendError(w, http.StatusNotFound, models.ErrorCodeNotFound, models.ErrMsgResourceNotFound)
		return
	}
	if errors.Is(err, apperrors.ErrPRExists) {
		h.sendError(w, http.StatusConflict, models.ErrorCodePRExists, models.ErrMsgPRIDAlreadyExists)
		return
	}
	if errors.Is(err, apperrors.ErrPRNotFound) {
		h.sendError(w, http.StatusNotFound, models.ErrorCodeNotFound, models.ErrMsgResourceNotFound)
		return
	}
	if errors.Is(err, apperrors.ErrPRMerged) {
		h.sendError(w, http.StatusConflict, models.ErrorCodePRMerged, models.ErrMsgCannotReassignMerged)
		return
	}
	if errors.Is(err, apperrors.ErrPRInvalidStatus) {
		h.sendError(w, http.StatusConflict, models.ErrorCodePRMerged, models.ErrMsgPRInvalidStatus)
		return
	}
	if errors.Is(err, apperrors.ErrReviewerNotAssigned) {
		h.sendError(w, http.StatusConflict, models.ErrorCodeNotAssigned, models.ErrMsgReviewerNotAssigned)
		return
	}
	if errors.Is(err, apperrors.ErrNoCandidate) {
		h.sendError(w, http.StatusConflict, models.ErrorCodeNoCandidate, models.ErrMsgNoCandidate)
		return
	}

	h.sendError(w, http.StatusInternalServerError, models.ErrorCodeInternalServerError, models.ErrMsgInternalServerError)
}

func (h *Handler) sendJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("Failed to encode JSON response after WriteHeader was called",
			"error", err,
			"status_code", statusCode,
			"error_type", "json_encode_failure")
	}
}
func (h *Handler) sendError(w http.ResponseWriter, statusCode int, code models.ErrorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(models.NewErrorResponse(code, message)); err != nil {
		h.logger.Error("Failed to encode error response after WriteHeader was called",
			"error", err,
			"status_code", statusCode,
			"error_code", code,
			"error_type", "json_encode_failure")
	}
}
