package handler

import (
	"io"
	"net/http"
	"strings"

	"github.com/timaogurtzova/shortener/internal/service"
)

// CreateHandler адаптирует HTTP-запрос на создание короткого URL
type CreateHandler struct {
	Service *service.ShortenerService
	BaseURL string
}

// ServeHTTP реализует интерфейс http.Handler
func (h *CreateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Проверка метода
	if r.Method != http.MethodPost || r.URL.Path != "/" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Чтение и валидация тела запроса
	body, err := io.ReadAll(r.Body)
	if err != nil || len(strings.TrimSpace(string(body))) == 0 {
		http.Error(w, "bad request: empty body", http.StatusBadRequest)
		return
	}
	originalURL := strings.TrimSpace(string(body))

	// Вызов бизнес-логики
	id, err := h.Service.Create(originalURL)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Формирование ответа
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(h.BaseURL + "/" + id))
}
