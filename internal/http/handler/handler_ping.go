package handler

import (
	"net/http"

	"github.com/timaogurtzova/shortener/internal/service"
)

// PingHandler обрабатывает проверку доступности хранилища.
type PingHandler struct {
	healthChecker service.HealthChecker
}

// NewPingHandler создаёт обработчик health-check эндпоинта.
func NewPingHandler(healthChecker service.HealthChecker) *PingHandler {
	return &PingHandler{healthChecker: healthChecker}
}

// Ping обрабатывает GET /ping и возвращает статус доступности хранилища.
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
