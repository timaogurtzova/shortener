package handler

import (
	"net/http"

	"github.com/timaogurtzova/shortener/internal/service"
)

// RedirectHandler обрабатывает GET /{id} запрос на редирект по короткому URL
type RedirectHandler struct {
	Service service.URLShortener
}

func (h *RedirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Проверка метода
	if r.Method != http.MethodGet {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Валидация пути
	id := r.URL.Path[1:]
	if id == "" {
		http.Error(w, "bad request: missing id", http.StatusBadRequest)
		return
	}

	// Вызов бизнес-логики
	originalURL, err := h.Service.Resolve(id)
	if err != nil {
		http.Error(w, "bad request: id not found", http.StatusBadRequest)
		return
	}

	// Формирование HTTP-ответа
	w.Header().Set("Location", originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}
