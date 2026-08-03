package handler

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/timaogurtzova/shortener/internal/service"
)

const realIPHeader = "X-Real-IP"

// StatsHandler обрабатывает запросы внутренней статистики.
type StatsHandler struct {
	statsProvider service.StatsProvider
	trustedSubnet *net.IPNet
}

// NewStatsHandler создаёт обработчик статистики и разбирает доверенную подсеть.
// Пустая подсеть запрещает доступ для всех запросов.
func NewStatsHandler(statsProvider service.StatsProvider, trustedSubnet string) (*StatsHandler, error) {
	trustedSubnet = strings.TrimSpace(trustedSubnet)
	if trustedSubnet == "" {
		return &StatsHandler{statsProvider: statsProvider}, nil
	}

	_, subnet, err := net.ParseCIDR(trustedSubnet)
	if err != nil {
		return nil, fmt.Errorf("parse trusted subnet %q: %w", trustedSubnet, err)
	}

	return &StatsHandler{
		statsProvider: statsProvider,
		trustedSubnet: subnet,
	}, nil
}

// GetStats обрабатывает GET /api/internal/stats.
func (h *StatsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	clientIP := net.ParseIP(strings.TrimSpace(r.Header.Get(realIPHeader)))
	if h.trustedSubnet == nil || clientIP == nil || !h.trustedSubnet.Contains(clientIP) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	if h.statsProvider == nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	stats, err := h.statsProvider.GetStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	responseBody, err := json.Marshal(statsResponse{
		URLs:  stats.URLs,
		Users: stats.Users,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(http.StatusOK)
	writeResponse(w, responseBody)
}
