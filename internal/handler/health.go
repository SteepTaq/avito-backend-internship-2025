package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.service.Ping(ctx); err != nil {
		reqID := middleware.GetReqID(r.Context())
		h.logger.Info("Health check failed: database connection error",
			"error", err,
			"request_id", reqID)
		h.sendJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "unhealthy",
			"error":  "database connection failed",
		})
		return
	}

	h.sendJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
