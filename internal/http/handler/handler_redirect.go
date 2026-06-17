package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"github.com/timaogurtzova/shortener/internal/audit"
	"github.com/timaogurtzova/shortener/internal/service"
)

// RedirectHandler обрабатывает GET /{id} запрос на редирект по короткому URL.
type RedirectHandler struct {
	service        service.URLShortener
	auditor        auditNotifier
	userIDResolver userIDResolver
}

// NewRedirectHandler создаёт обработчик перехода по короткому URL.
func NewRedirectHandler(service service.URLShortener) *RedirectHandler {
	return &RedirectHandler{service: service}
}

// SetAuditPublisher подключает издатель событий аудита к обработчику редиректа.
func (h *RedirectHandler) SetAuditPublisher(publisher auditNotifier) {
	h.auditor = publisher
}

// SetUserIDResolver подключает чтение user ID для событий аудита редиректа.
func (h *RedirectHandler) SetUserIDResolver(resolver userIDResolver) {
	h.userIDResolver = resolver
}

// Redirect обрабатывает переход по короткому идентификатору и возвращает HTTP-редирект.
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
		if errors.Is(err, service.ErrURLDeleted) {
			http.Error(w, "gone", http.StatusGone)
			return
		}

		http.Error(w, "bad request: id not found", http.StatusBadRequest)
		return
	}

	// Формирование HTTP-ответа
	w.Header().Set("Location", originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
	publishAuditEvent(r.Context(), h.auditor, audit.ActionFollow, h.userID(r), originalURL)
}

func (h *RedirectHandler) userID(r *http.Request) string {
	if h.userIDResolver == nil {
		return ""
	}

	userID, ok, err := h.userIDResolver.UserID(r)
	if err != nil {
		log.Error().Err(err).Msg("failed to resolve user id for audit")
		return ""
	}
	if !ok {
		return ""
	}

	return userID
}
