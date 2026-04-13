package handler

import (
	"net/http"

	"github.com/timaogurtzova/shortener/internal/service"
)

type PingHandler struct {
	healthChecker service.HealthChecker
}

func NewPingHandler(healthChecker service.HealthChecker) *PingHandler {
	return &PingHandler{healthChecker: healthChecker}
}

func (h *PingHandler) Ping(w http.ResponseWriter, r *http.Request) {
	if h.healthChecker == nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := h.healthChecker.Ping(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.WriteHeader(http.StatusOK)
}
