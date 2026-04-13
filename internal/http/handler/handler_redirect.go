package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/timaogurtzova/shortener/internal/service"
)

// RedirectHandler обрабатывает GET /{id} запрос на редирект по короткому URL
type RedirectHandler struct {
	service service.URLShortener
}

func NewRedirectHandler(service service.URLShortener) *RedirectHandler {
	return &RedirectHandler{service: service}
}

func (h *RedirectHandler) Redirect(w http.ResponseWriter, r *http.Request) {
	// Валидация пути
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "bad request: missing id", http.StatusBadRequest)
		return
	}

	// Вызов бизнес-логики
	originalURL, err := h.service.Resolve(r.Context(), id)
	if err != nil {
		http.Error(w, "bad request: id not found", http.StatusBadRequest)
		return
	}

	// Формирование HTTP-ответа
	w.Header().Set("Location", originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}
