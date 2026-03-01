package handler_test

import (
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

func TestCreateHandler(t *testing.T) {
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
					CreateMockFunc: func(url string) (string, error) {
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
					CreateMockFunc: func(url string) (string, error) {
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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var svc service.URLShortener
			if test.mock != nil {
				svc = test.mock()
			} else {
				// минимальный мок
				svc = &mockURLShortener{
					CreateMockFunc: func(url string) (string, error) { return "", nil },
				}
			}

			h := handler.NewCreateHandler(svc, "http://localhost:8080")

			req := httptest.NewRequest(test.method, "/", strings.NewReader(test.body))
			req.Header.Set("Content-Type", test.contentType)
			req.Host = "localhost:8080"
			rec := httptest.NewRecorder()

			h.Create(rec, req)

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
