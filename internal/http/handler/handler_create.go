package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/timaogurtzova/shortener/internal/audit"
	"github.com/timaogurtzova/shortener/internal/service"
)

const (
	maxBodySize      = 2048
	maxBatchBodySize = 65536
	contentTypeText  = "text/plain"
	contentTypeJSON  = "application/json"
)

type CreateHandler struct {
	service service.URLShortener
	baseURL string
	auth    userAuthenticator
	auditor auditNotifier
}

func NewCreateHandler(service service.URLShortener, baseURL string, authenticator userAuthenticator) *CreateHandler {
	return &CreateHandler{service: service, baseURL: baseURL, auth: authenticator}
}

func (h *CreateHandler) SetAuditPublisher(publisher auditNotifier) {
	h.auditor = publisher
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

	shortURL, err := h.createShortURL(w, r, originalURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Формирование ответа
	w.Header().Set("Content-Type", contentTypeText)
	w.WriteHeader(shortURL.statusCode)
	writeResponse(w, []byte(shortURL.value))
	if shortURL.statusCode == http.StatusCreated {
		publishAuditEvent(r.Context(), h.auditor, audit.ActionShorten, shortURL.userID, originalURL)
	}
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

	shortURL, err := h.createShortURL(w, r, originalURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	responseBody, err := json.Marshal(shortenResponse{Result: shortURL.value})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(shortURL.statusCode)
	writeResponse(w, responseBody)
	if shortURL.statusCode == http.StatusCreated {
		publishAuditEvent(r.Context(), h.auditor, audit.ActionShorten, shortURL.userID, originalURL)
	}
}

func (h *CreateHandler) CreateShortURLBatchJSON(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), contentTypeJSON) {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBatchBodySize)
	defer r.Body.Close()

	var request []batchShortenRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}

	if len(request) == 0 {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}

	originalURLs := make([]string, len(request))
	response := make([]batchShortenResponse, len(request))

	for i, item := range request {
		correlationID := strings.TrimSpace(item.CorrelationID)
		originalURL := strings.TrimSpace(item.OriginalURL)
		if correlationID == "" || originalURL == "" {
			writeError(w, http.StatusBadRequest, "bad request")
			return
		}

		originalURLs[i] = originalURL
		response[i].CorrelationID = correlationID
	}

	userID, err := h.auth.EnsureUserID(w, r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	shortIDs, err := h.service.CreateBatch(r.Context(), originalURLs, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if len(shortIDs) != len(request) {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	for i, shortID := range shortIDs {
		response[i].ShortURL = h.baseURL + "/" + shortID
	}

	responseBody, err := json.Marshal(response)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(http.StatusCreated)
	writeResponse(w, responseBody)
}

type createShortURLResult struct {
	value      string
	statusCode int
	userID     string
}

func (h *CreateHandler) createShortURL(w http.ResponseWriter, r *http.Request, originalURL string) (createShortURLResult, error) {
	userID, err := h.auth.EnsureUserID(w, r)
	if err != nil {
		return createShortURLResult{}, err
	}

	shortID, err := h.service.Create(r.Context(), originalURL, userID)
	if err != nil {
		if errors.Is(err, service.ErrURLAlreadyExists) && shortID != "" {
			return createShortURLResult{
				value:      h.baseURL + "/" + shortID,
				statusCode: http.StatusConflict,
				userID:     userID,
			}, nil
		}

		return createShortURLResult{}, err
	}

	return createShortURLResult{
		value:      h.baseURL + "/" + shortID,
		statusCode: http.StatusCreated,
		userID:     userID,
	}, nil
}

func writeError(w http.ResponseWriter, status int, msg string) {
	http.Error(w, msg, status)
}

func writeResponse(w http.ResponseWriter, body []byte) {
	if _, err := w.Write(body); err != nil {
		log.Error().Err(err).Msg("failed to write http response")
	}
}
