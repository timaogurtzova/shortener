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
	service service.URLShortener
	baseURL string
}

func NewCreateHandler(service service.URLShortener, baseURL string) *CreateHandler {
	return &CreateHandler{service: service, baseURL: baseURL}
}

func (h *CreateHandler) Create(w http.ResponseWriter, r *http.Request) {
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
	id, err := h.service.Create(originalURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Формирование ответа
	shortURL := h.baseURL + "/" + id
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(shortURL))
}

func writeError(w http.ResponseWriter, status int, msg string) {
	http.Error(w, msg, status)
}
