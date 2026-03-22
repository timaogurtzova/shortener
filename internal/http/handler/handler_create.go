package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/timaogurtzova/shortener/internal/service"
)

const (
	maxBodySize     = 2048
	contentTypeText = "text/plain"
	contentTypeJSON = "application/json"
)

type CreateHandler struct {
	service service.URLShortener
	baseURL string
}

func NewCreateHandler(service service.URLShortener, baseURL string) *CreateHandler {
	return &CreateHandler{service: service, baseURL: baseURL}
}

func (h *CreateHandler) CreateShortURLPlainText(w http.ResponseWriter, r *http.Request) {
	// Проверяем Content-Type (допускаем charset)
	if !strings.HasPrefix(r.Header.Get("Content-Type"), contentTypeText) {
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

	shortURL, err := h.createShortURL(originalURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Формирование ответа
	w.Header().Set("Content-Type", contentTypeText)
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(shortURL))
}

func (h *CreateHandler) CreateShortURLJSON(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), contentTypeJSON) {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	defer r.Body.Close()

	var request shortenRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}

	originalURL := strings.TrimSpace(request.URL)
	if originalURL == "" {
		writeError(w, http.StatusBadRequest, "bad request: empty body")
		return
	}

	shortURL, err := h.createShortURL(originalURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	responseBody, err := json.Marshal(shortenResponse{Result: shortURL})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(http.StatusCreated)
	w.Write(responseBody)
}

func (h *CreateHandler) createShortURL(originalURL string) (string, error) {
	shortID, err := h.service.Create(originalURL)
	if err != nil {
		return "", err
	}

	return h.baseURL + "/" + shortID, nil
}

func writeError(w http.ResponseWriter, status int, msg string) {
	http.Error(w, msg, status)
}
