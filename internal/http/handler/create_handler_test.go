package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/timaogurtzova/shortener/internal/http/handler"
	"github.com/timaogurtzova/shortener/internal/service"
)

func TestCreateShortURLPlainTextHandler(t *testing.T) {
	type want struct {
		code        int
		response    string
		contentType string
	}

	tests := []struct {
		name        string
		method      string
		contentType string
		body        string
		mock        func() *mockURLShortener
		want        want
	}{
		{
			name:        "успешное создание",
			method:      http.MethodPost,
			body:        "http://localhost:8080",
			contentType: "text/plain",
			mock: func() *mockURLShortener {
				return &mockURLShortener{
					CreateMockFunc: func(ctx context.Context, url string) (string, error) {
						return "abc123", nil
					},
				}
			},
			want: want{
				code:        http.StatusCreated,
				response:    "http://localhost:8080/abc123",
				contentType: "text/plain",
			},
		},
		{
			name:        "неправильный Content-Type",
			method:      http.MethodPost,
			body:        "http://localhost:8080",
			mock:        nil,
			contentType: "error",
			want: want{
				code:        http.StatusBadRequest,
				response:    "bad request\n",
				contentType: "text/plain; charset=utf-8",
			},
		},
		{
			name:        "пустое тело запроса",
			method:      http.MethodPost,
			body:        "   ",
			contentType: "text/plain",
			mock:        nil,
			want: want{
				code:        http.StatusBadRequest,
				response:    "bad request: empty body\n",
				contentType: "text/plain; charset=utf-8",
			},
		},
		{
			name:        "ошибка сервиса",
			method:      http.MethodPost,
			body:        "http://localhost:8080",
			contentType: "text/plain",
			mock: func() *mockURLShortener {
				return &mockURLShortener{
					CreateMockFunc: func(ctx context.Context, url string) (string, error) {
						return "", errors.New("service error")
					},
				}
			},
			want: want{
				code:        http.StatusInternalServerError,
				response:    "internal server error\n",
				contentType: "text/plain; charset=utf-8",
			},
		},
		{
			name:        "url уже существует",
			method:      http.MethodPost,
			body:        "http://localhost:8080",
			contentType: "text/plain",
			mock: func() *mockURLShortener {
				return &mockURLShortener{
					CreateMockFunc: func(ctx context.Context, url string) (string, error) {
						return "abc123", service.ErrURLAlreadyExists
					},
				}
			},
			want: want{
				code:        http.StatusConflict,
				response:    "http://localhost:8080/abc123",
				contentType: "text/plain",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var svc service.URLShortener
			if test.mock != nil {
				svc = test.mock()
			} else {
				// минимальный мок
				svc = &mockURLShortener{
					CreateMockFunc: func(ctx context.Context, url string) (string, error) { return "", nil },
				}
			}

			h := handler.NewCreateHandler(svc, "http://localhost:8080")

			req := httptest.NewRequest(test.method, "/", strings.NewReader(test.body))
			req.Header.Set("Content-Type", test.contentType)
			req.Host = "localhost:8080"
			rec := httptest.NewRecorder()

			h.CreateShortURLPlainText(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			// Проверяем код
			assert.Equal(t, test.want.code, res.StatusCode)

			// Проверяем тело
			body, err := io.ReadAll(res.Body)
			require.NoError(t, err)
			assert.Equal(t, test.want.response, string(body))

			// Проверяем Content-Type
			assert.Equal(t, test.want.contentType, res.Header.Get("Content-Type"))
		})
	}
}

func TestCreateShortURLJSONHandler(t *testing.T) {
	type want struct {
		code        int
		response    string
		contentType string
	}

	tests := []struct {
		name        string
		method      string
		contentType string
		body        string
		mock        func() *mockURLShortener
		want        want
	}{
		{
			name:        "успешное создание через json",
			method:      http.MethodPost,
			body:        `{"url":"https://practicum.yandex.ru"}`,
			contentType: "application/json",
			mock: func() *mockURLShortener {
				return &mockURLShortener{
					CreateMockFunc: func(ctx context.Context, url string) (string, error) {
						return "abc123", nil
					},
				}
			},
			want: want{
				code:        http.StatusCreated,
				response:    `{"result":"http://localhost:8080/abc123"}`,
				contentType: "application/json",
			},
		},
		{
			name:        "неправильный Content-Type",
			method:      http.MethodPost,
			body:        `{"url":"https://practicum.yandex.ru"}`,
			contentType: "text/plain",
			want: want{
				code:        http.StatusBadRequest,
				response:    "bad request\n",
				contentType: "text/plain; charset=utf-8",
			},
		},
		{
			name:        "пустой url в json",
			method:      http.MethodPost,
			body:        `{"url":"   "}`,
			contentType: "application/json",
			want: want{
				code:        http.StatusBadRequest,
				response:    "bad request: empty body\n",
				contentType: "text/plain; charset=utf-8",
			},
		},
		{
			name:        "невалидный json",
			method:      http.MethodPost,
			body:        `{"url":`,
			contentType: "application/json",
			want: want{
				code:        http.StatusBadRequest,
				response:    "bad request\n",
				contentType: "text/plain; charset=utf-8",
			},
		},
		{
			name:        "ошибка сервиса",
			method:      http.MethodPost,
			body:        `{"url":"https://practicum.yandex.ru"}`,
			contentType: "application/json",
			mock: func() *mockURLShortener {
				return &mockURLShortener{
					CreateMockFunc: func(ctx context.Context, url string) (string, error) {
						return "", errors.New("service error")
					},
				}
			},
			want: want{
				code:        http.StatusInternalServerError,
				response:    "internal server error\n",
				contentType: "text/plain; charset=utf-8",
			},
		},
		{
			name:        "url уже существует",
			method:      http.MethodPost,
			body:        `{"url":"https://practicum.yandex.ru"}`,
			contentType: "application/json",
			mock: func() *mockURLShortener {
				return &mockURLShortener{
					CreateMockFunc: func(ctx context.Context, url string) (string, error) {
						return "abc123", service.ErrURLAlreadyExists
					},
				}
			},
			want: want{
				code:        http.StatusConflict,
				response:    `{"result":"http://localhost:8080/abc123"}`,
				contentType: "application/json",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var svc service.URLShortener
			if test.mock != nil {
				svc = test.mock()
			} else {
				svc = &mockURLShortener{
					CreateMockFunc: func(ctx context.Context, url string) (string, error) { return "", nil },
				}
			}

			h := handler.NewCreateHandler(svc, "http://localhost:8080")

			req := httptest.NewRequest(test.method, "/api/shorten", strings.NewReader(test.body))
			req.Header.Set("Content-Type", test.contentType)
			rec := httptest.NewRecorder()

			h.CreateShortURLJSON(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			assert.Equal(t, test.want.code, res.StatusCode)

			body, err := io.ReadAll(res.Body)
			require.NoError(t, err)

			if test.want.contentType == "application/json" {
				var actual map[string]string
				require.NoError(t, json.Unmarshal(body, &actual))

				var expected map[string]string
				require.NoError(t, json.Unmarshal([]byte(test.want.response), &expected))
				assert.Equal(t, expected, actual)
			} else {
				assert.Equal(t, test.want.response, string(body))
			}

			assert.Equal(t, test.want.contentType, res.Header.Get("Content-Type"))
		})
	}
}

func TestCreateShortURLBatchJSONHandler(t *testing.T) {
	type want struct {
		code        int
		response    string
		contentType string
	}

	tests := []struct {
		name        string
		contentType string
		body        string
		mock        func() *mockURLShortener
		want        want
	}{
		{
			name:        "успешное пакетное создание через json",
			body:        `[{"correlation_id":"1","original_url":"https://example.com"},{"correlation_id":"2","original_url":"https://practicum.yandex.ru"}]`,
			contentType: "application/json",
			mock: func() *mockURLShortener {
				return &mockURLShortener{
					CreateBatchMockFunc: func(ctx context.Context, urls []string) ([]string, error) {
						return []string{"abc123", "def456"}, nil
					},
				}
			},
			want: want{
				code:        http.StatusCreated,
				response:    `[{"correlation_id":"1","short_url":"http://localhost:8080/abc123"},{"correlation_id":"2","short_url":"http://localhost:8080/def456"}]`,
				contentType: "application/json",
			},
		},
		{
			name:        "неправильный Content-Type",
			body:        `[{"correlation_id":"1","original_url":"https://example.com"}]`,
			contentType: "text/plain",
			want: want{
				code:        http.StatusBadRequest,
				response:    "bad request\n",
				contentType: "text/plain; charset=utf-8",
			},
		},
		{
			name:        "пустой батч",
			body:        `[]`,
			contentType: "application/json",
			want: want{
				code:        http.StatusBadRequest,
				response:    "bad request\n",
				contentType: "text/plain; charset=utf-8",
			},
		},
		{
			name:        "пустой correlation_id",
			body:        `[{"correlation_id":" ","original_url":"https://example.com"}]`,
			contentType: "application/json",
			want: want{
				code:        http.StatusBadRequest,
				response:    "bad request\n",
				contentType: "text/plain; charset=utf-8",
			},
		},
		{
			name:        "ошибка сервиса",
			body:        `[{"correlation_id":"1","original_url":"https://example.com"}]`,
			contentType: "application/json",
			mock: func() *mockURLShortener {
				return &mockURLShortener{
					CreateBatchMockFunc: func(ctx context.Context, urls []string) ([]string, error) {
						return nil, errors.New("service error")
					},
				}
			},
			want: want{
				code:        http.StatusInternalServerError,
				response:    "internal server error\n",
				contentType: "text/plain; charset=utf-8",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var svc service.URLShortener
			if test.mock != nil {
				svc = test.mock()
			} else {
				svc = &mockURLShortener{
					CreateBatchMockFunc: func(ctx context.Context, urls []string) ([]string, error) { return nil, nil },
				}
			}

			h := handler.NewCreateHandler(svc, "http://localhost:8080")

			req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", strings.NewReader(test.body))
			req.Header.Set("Content-Type", test.contentType)
			rec := httptest.NewRecorder()

			h.CreateShortURLBatchJSON(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			assert.Equal(t, test.want.code, res.StatusCode)

			body, err := io.ReadAll(res.Body)
			require.NoError(t, err)

			if test.want.contentType == "application/json" {
				var actual []map[string]string
				require.NoError(t, json.Unmarshal(body, &actual))

				var expected []map[string]string
				require.NoError(t, json.Unmarshal([]byte(test.want.response), &expected))
				assert.Equal(t, expected, actual)
			} else {
				assert.Equal(t, test.want.response, string(body))
			}

			assert.Equal(t, test.want.contentType, res.Header.Get("Content-Type"))
		})
	}
}
