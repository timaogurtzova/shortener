package httpserver_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/timaogurtzova/shortener/internal/http"
)

func TestServerRouting(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		path         string
		wantBody     string
		wantCode     int
		wantCreate   bool
		wantRedirect bool
	}{
		{
			name:         "POST / -> createHandler ok",
			method:       http.MethodPost,
			path:         "/",
			wantBody:     "create",
			wantCode:     http.StatusCreated,
			wantCreate:   true,
			wantRedirect: false,
		},
		{
			name:         "GET / -> createHandler bad method",
			method:       http.MethodGet,
			path:         "/",
			wantBody:     "method not allowed",
			wantCode:     http.StatusBadRequest,
			wantCreate:   false,
			wantRedirect: false,
		},
		{
			name:         "GET /abc -> redirectHandler ok",
			method:       http.MethodGet,
			path:         "/abc",
			wantBody:     "redirect",
			wantCode:     http.StatusTemporaryRedirect,
			wantCreate:   false,
			wantRedirect: true,
		},
		{
			name:         "POST /abc -> redirectHandler bad method",
			method:       http.MethodPost,
			path:         "/abc",
			wantBody:     "method not allowed",
			wantCode:     http.StatusBadRequest,
			wantCreate:   false,
			wantRedirect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Флаги для проверки вызова handler'ов
			createCalled := false
			redirectCalled := false

			// Мок CreateHandler
			createHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				createCalled = true
				if r.Method != http.MethodPost {
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				}
				w.WriteHeader(http.StatusCreated)
				w.Write([]byte("create"))
			})

			// Мок RedirectHandler
			redirectHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				redirectCalled = true
				if r.Method != http.MethodGet {
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				}
				w.WriteHeader(http.StatusTemporaryRedirect)
				w.Write([]byte("redirect"))
			})

			// Создаём handler через NewRouter (production-стиль)
			router := httpserver.NewRouter(createHandler, redirectHandler)

			// Создаём рекордер и запрос
			var bodyReader *strings.Reader
			if tt.method == http.MethodPost {
				bodyReader = strings.NewReader("body")
			} else {
				bodyReader = strings.NewReader("")
			}
			req := httptest.NewRequest(tt.method, tt.path, bodyReader)
			rec := httptest.NewRecorder()

			// ServeHTTP через handler
			router.ServeHTTP(rec, req)

			resp := rec.Result()
			defer resp.Body.Close()

			respBody, _ := io.ReadAll(resp.Body)

			// Проверка статуса и тела ответа
			assert.Equal(t, tt.wantCode, resp.StatusCode)
			if tt.wantCode >= 400 {
				assert.Equal(t, tt.wantBody+"\n", string(respBody))
			} else {
				assert.Equal(t, tt.wantBody, string(respBody))
			}

			// Проверка, какой handler был вызван
			assert.Equal(t, tt.wantCreate, createCalled)
			assert.Equal(t, tt.wantRedirect, redirectCalled)
		})
	}
}

func TestLoggingMiddlewareLogsRequestAndResponseData(t *testing.T) {
	var buf bytes.Buffer
	oldLogger := log.Logger
	log.Logger = zerolog.New(&buf).Level(zerolog.InfoLevel)
	t.Cleanup(func() {
		log.Logger = oldLogger
	})

	createHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("create"))
	})

	redirectHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTemporaryRedirect)
		w.Write([]byte("redirect"))
	})

	router := httpserver.NewRouter(createHandler, redirectHandler)

	req := httptest.NewRequest(http.MethodPost, "/?trace=1", strings.NewReader("body"))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	logOutput := buf.String()
	assert.Contains(t, logOutput, `"level":"info"`)
	assert.Contains(t, logOutput, `"uri":"/?trace=1"`)
	assert.Contains(t, logOutput, `"method":"POST"`)
	assert.Contains(t, logOutput, `"duration":"`)
	assert.Contains(t, logOutput, `"status":201`)
	assert.Contains(t, logOutput, `"size":6`)
}

func TestLoggingMiddlewareLogsImplicitStatusCode(t *testing.T) {
	var buf bytes.Buffer
	oldLogger := log.Logger
	log.Logger = zerolog.New(&buf).Level(zerolog.InfoLevel)
	t.Cleanup(func() {
		log.Logger = oldLogger
	})

	createHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("create"))
	})

	redirectHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTemporaryRedirect)
		w.Write([]byte("redirect"))
	})

	router := httpserver.NewRouter(createHandler, redirectHandler)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("body"))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, buf.String(), `"status":200`)
}
