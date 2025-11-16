package handler

import (
	"context"
	"net/http"
)

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), RequestTimeout)
	defer cancel()

	stats, err := h.service.GetStats(ctx)
	if err != nil {
		h.handleServiceError(w, r, err, "GetStats")
		return
	}

	h.sendJSON(w, http.StatusOK, stats)
}
