package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/timaogurtzova/shortener/internal/auth"
	"github.com/timaogurtzova/shortener/internal/service"
)

// UserHandler обслуживает endpoints, завязанные на пользователя.
type UserHandler struct {
	service service.URLShortener
	baseURL string
	auth    *auth.Authenticator
}

func NewUserHandler(service service.URLShortener, baseURL string, authenticator *auth.Authenticator) *UserHandler {
	return &UserHandler{service: service, baseURL: baseURL, auth: authenticator}
}

func (h *UserHandler) GetUserURLs(w http.ResponseWriter, r *http.Request) {
	userID, err := h.auth.UserIDForHistory(w, r)
	if err != nil {
		if errors.Is(err, auth.ErrUserIDMissing) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	userURLs, err := h.service.FindByUserID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if len(userURLs) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	response := make([]userURLResponse, len(userURLs))
	for i, item := range userURLs {
		response[i] = userURLResponse{
			ShortURL:    h.baseURL + "/" + item.ShortID,
			OriginalURL: item.OriginalURL,
		}
	}

	responseBody, err := json.Marshal(response)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(http.StatusOK)
	w.Write(responseBody)
}
