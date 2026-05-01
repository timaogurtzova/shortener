package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/timaogurtzova/shortener/internal/auth"
	"github.com/timaogurtzova/shortener/internal/service"
)

// UserHandler обслуживает endpoints, завязанные на пользователя.
type UserHandler struct {
	service service.URLShortener
	baseURL string
	auth    userAuthenticator
}

func NewUserHandler(service service.URLShortener, baseURL string, authenticator userAuthenticator) *UserHandler {
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
	writeResponse(w, responseBody)
}

func (h *UserHandler) DeleteUserURLs(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), contentTypeJSON) {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBatchBodySize)
	defer r.Body.Close()

	var request []string
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}

	if len(request) == 0 {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}

	shortIDs := make([]string, 0, len(request))
	seen := make(map[string]struct{}, len(request))
	for _, shortID := range request {
		shortID = strings.TrimSpace(shortID)
		if shortID == "" {
			writeError(w, http.StatusBadRequest, "bad request")
			return
		}

		if _, exists := seen[shortID]; exists {
			continue
		}

		seen[shortID] = struct{}{}
		shortIDs = append(shortIDs, shortID)
	}

	userID, err := h.auth.UserIDForHistory(w, r)
	if err != nil {
		if errors.Is(err, auth.ErrUserIDMissing) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := h.service.DeleteUserURLs(r.Context(), userID, shortIDs); err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
