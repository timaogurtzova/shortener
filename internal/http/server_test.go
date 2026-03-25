package httpserver_test

import (
	"bytes"
	"compress/gzip"
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
		name                        string
		method                      string
		path                        string
		wantBody                    string
		wantCode                    int
		wantCreateShortURLPlainText bool
		wantCreateShortURLJSON      bool
		wantRedirect                bool
	}{
		{
			name:                        "POST / -> createShortURLPlainText handler ok",
			method:                      http.MethodPost,
			path:                        "/",
			wantBody:                    "create",
			wantCode:                    http.StatusCreated,
			wantCreateShortURLPlainText: true,
			wantCreateShortURLJSON:      false,
			wantRedirect:                false,
		},
		{
			name:                        "GET / -> createShortURLPlainText bad method",
			method:                      http.MethodGet,
			path:                        "/",
			wantBody:                    "method not allowed",
			wantCode:                    http.StatusBadRequest,
			wantCreateShortURLPlainText: false,
			wantCreateShortURLJSON:      false,
			wantRedirect:                false,
		},
		{
			name:                        "POST /api/shorten -> createShortURLJSON handler ok",
			method:                      http.MethodPost,
			path:                        "/api/shorten",
			wantBody:                    "create-json",
			wantCode:                    http.StatusCreated,
			wantCreateShortURLPlainText: false,
			wantCreateShortURLJSON:      true,
			wantRedirect:                false,
		},
		{
			name:                        "GET /abc -> redirectHandler ok",
			method:                      http.MethodGet,
			path:                        "/abc",
			wantBody:                    "redirect",
			wantCode:                    http.StatusTemporaryRedirect,
			wantCreateShortURLPlainText: false,
			wantCreateShortURLJSON:      false,
			wantRedirect:                true,
		},
		{
			name:                        "POST /abc -> redirectHandler bad method",
			method:                      http.MethodPost,
			path:                        "/abc",
			wantBody:                    "method not allowed",
			wantCode:                    http.StatusBadRequest,
			wantCreateShortURLPlainText: false,
			wantCreateShortURLJSON:      false,
			wantRedirect:                false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Флаги для проверки вызова handler'ов
			createShortURLPlainTextCalled := false
			createShortURLJSONCalled := false
			redirectCalled := false

			createShortURLPlainTextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				createShortURLPlainTextCalled = true
				if r.Method != http.MethodPost {
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				}
				w.WriteHeader(http.StatusCreated)
				w.Write([]byte("create"))
			})

			createShortURLJSONHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				createShortURLJSONCalled = true
				if r.Method != http.MethodPost {
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				}
				w.WriteHeader(http.StatusCreated)
				w.Write([]byte("create-json"))
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
			router := httpserver.NewRouter(createShortURLPlainTextHandler, createShortURLJSONHandler, redirectHandler)

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
			assert.Equal(t, tt.wantCreateShortURLPlainText, createShortURLPlainTextCalled)
			assert.Equal(t, tt.wantCreateShortURLJSON, createShortURLJSONCalled)
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

	createShortURLPlainTextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("create"))
	})

	redirectHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTemporaryRedirect)
		w.Write([]byte("redirect"))
	})

	router := httpserver.NewRouter(createShortURLPlainTextHandler, createShortURLPlainTextHandler, redirectHandler)

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

	createShortURLPlainTextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("create"))
	})

	redirectHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTemporaryRedirect)
		w.Write([]byte("redirect"))
	})

	router := httpserver.NewRouter(createShortURLPlainTextHandler, createShortURLPlainTextHandler, redirectHandler)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("body"))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, buf.String(), `"status":200`)
}

func TestGzipRequestMiddlewareDecompressesRequestBody(t *testing.T) {
	var requestBody string

	createShortURLPlainTextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		requestBody = string(body)

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("create"))
	})

	createShortURLJSONHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"result":"ok"}`))
	})

	redirectHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTemporaryRedirect)
	})

	router := httpserver.NewRouter(createShortURLPlainTextHandler, createShortURLJSONHandler, redirectHandler)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(gzipData(t, "http://localhost:8080")))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Content-Encoding", "gzip")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "http://localhost:8080", requestBody)
}

func TestGzipRequestMiddlewareReturnsBadRequestForUnsupportedEncoding(t *testing.T) {
	createShortURLPlainTextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("create"))
	})

	createShortURLJSONHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"result":"ok"}`))
	})

	redirectHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTemporaryRedirect)
	})

	router := httpserver.NewRouter(createShortURLPlainTextHandler, createShortURLJSONHandler, redirectHandler)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("body"))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Content-Encoding", "deflate")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "bad request\n", rec.Body.String())
}

func TestGzipRequestMiddlewareReturnsBadRequestForBrokenGzip(t *testing.T) {
	createShortURLPlainTextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("create"))
	})

	createShortURLJSONHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"result":"ok"}`))
	})

	redirectHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTemporaryRedirect)
	})

	router := httpserver.NewRouter(createShortURLPlainTextHandler, createShortURLJSONHandler, redirectHandler)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not-a-gzip-stream"))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Content-Encoding", "gzip")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "bad request\n", rec.Body.String())
}

func TestGzipRequestMiddlewareReturnsBadRequestForMultipleEncodings(t *testing.T) {
	createShortURLPlainTextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("create"))
	})

	createShortURLJSONHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"result":"ok"}`))
	})

	redirectHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTemporaryRedirect)
	})

	router := httpserver.NewRouter(createShortURLPlainTextHandler, createShortURLJSONHandler, redirectHandler)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("body"))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Content-Encoding", "gzip, deflate")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "bad request\n", rec.Body.String())
}

func TestGzipResponseMiddlewareCompressesJSONResponse(t *testing.T) {
	createShortURLPlainTextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("create"))
	})

	createShortURLJSONHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"result":"http://localhost:8080/abc123"}`))
	})

	redirectHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTemporaryRedirect)
	})

	router := httpserver.NewRouter(createShortURLPlainTextHandler, createShortURLJSONHandler, redirectHandler)

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(`{"url":"https://practicum.yandex.ru"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	assert.Equal(t, "gzip", res.Header.Get("Content-Encoding"))
	assert.Contains(t, res.Header.Values("Vary"), "Accept-Encoding")
	assert.Equal(t, `{"result":"http://localhost:8080/abc123"}`, ungzipBody(t, res.Body))
}

func TestGzipResponseMiddlewareSkipsUnsupportedContentType(t *testing.T) {
	createShortURLPlainTextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("create"))
	})

	createShortURLJSONHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"result":"ok"}`))
	})

	redirectHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTemporaryRedirect)
	})

	router := httpserver.NewRouter(createShortURLPlainTextHandler, createShortURLJSONHandler, redirectHandler)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("body"))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	assert.NoError(t, err)

	assert.Empty(t, res.Header.Get("Content-Encoding"))
	assert.Equal(t, "create", string(body))
}

func gzipData(t *testing.T, data string) []byte {
	t.Helper()

	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	_, err := writer.Write([]byte(data))
	assert.NoError(t, err)
	assert.NoError(t, writer.Close())

	return buf.Bytes()
}

func ungzipBody(t *testing.T, body io.Reader) string {
	t.Helper()

	reader, err := gzip.NewReader(body)
	assert.NoError(t, err)
	defer reader.Close()

	data, err := io.ReadAll(reader)
	assert.NoError(t, err)

	return string(data)
}
