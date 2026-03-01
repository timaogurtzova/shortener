package handler

import (
	"io"
	"net/http"
	"strings"

	"github.com/timaogurtzova/shortener/internal/service"
)

const (
	maxBodySize = 2048
	contentType = "text/plain"
)

type CreateHandler struct {
	Service service.URLShortener
}

// ServeHTTP реализует интерфейс http.Handler
func (h *CreateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Проверяем Content-Type (допускаем charset)
	if !strings.HasPrefix(r.Header.Get("Content-Type"), contentType) {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}

	// Ограничиваем размер тела запроса
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}

	originalURL := strings.TrimSpace(string(body))
	if originalURL == "" {
		writeError(w, http.StatusBadRequest, "bad request: empty body")
		return
	}

	// Вызов бизнес-логики
	id, err := h.Service.Create(originalURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Формирование ответа
	baseURL := buildBaseURL(r)
	shortURL := baseURL + "/" + id
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(shortURL))
}

func buildBaseURL(r *http.Request) string {
	scheme := "http"

	// Если сервер работает по HTTPS
	if r.TLS != nil {
		scheme = "https"
	}

	// Если за reverse proxy (nginx, traefik)
	if forwardedProto := r.Header.Get("X-Forwarded-Proto"); forwardedProto != "" {
		scheme = forwardedProto
	}

	return scheme + "://" + r.Host
}

func writeError(w http.ResponseWriter, status int, msg string) {
	http.Error(w, msg, status)
}
