package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/timaogurtzova/shortener/internal/http/handler"
	"github.com/timaogurtzova/shortener/internal/service"
)

func TestRedirectHandler(t *testing.T) {
	type want struct {
		code        int
		location    string
		body        string
		contentType string
	}

	tests := []struct {
		name   string
		method string
		path   string
		mock   func() *mockURLShortener
		want   want
	}{
		{
			name:   "успешный редирект",
			method: http.MethodGet,
			path:   "/abc123",
			mock: func() *mockURLShortener {
				return &mockURLShortener{
					ResolveMockFunc: func(ctx context.Context, id string) (string, error) {
						return "http://localhost", nil
					},
				}
			},
			want: want{
				code:        http.StatusTemporaryRedirect,
				location:    "http://localhost",
				body:        "",
				contentType: "",
			},
		},
		{
			name:   "пустой id",
			method: http.MethodGet,
			path:   "/",
			mock:   nil,
			want: want{
				code:        http.StatusBadRequest,
				location:    "",
				body:        "bad request: missing id\n",
				contentType: "text/plain; charset=utf-8",
			},
		},
		{
			name:   "id не найден",
			method: http.MethodGet,
			path:   "/unknown",
			mock: func() *mockURLShortener {
				return &mockURLShortener{
					ResolveMockFunc: func(ctx context.Context, id string) (string, error) {
						return "", errors.New("not found")
					},
				}
			},
			want: want{
				code:        http.StatusBadRequest,
				location:    "",
				body:        "bad request: id not found\n",
				contentType: "text/plain; charset=utf-8",
			},
		},
		{
			name:   "id удалён",
			method: http.MethodGet,
			path:   "/deleted",
			mock: func() *mockURLShortener {
				return &mockURLShortener{
					ResolveMockFunc: func(ctx context.Context, id string) (string, error) {
						return "", service.ErrURLDeleted
					},
				}
			},
			want: want{
				code:        http.StatusGone,
				location:    "",
				body:        "gone\n",
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
					ResolveMockFunc: func(ctx context.Context, id string) (string, error) { return "", nil },
				}
			}

			h := handler.NewRedirectHandler(svc)
			req := httptest.NewRequest(test.method, test.path, nil)
			req.Host = "localhost:8080"

			// --- Добавляем RouteContext только для GET с id ---
			if test.method == http.MethodGet && test.path != "/" {
				rctx := chi.NewRouteContext()
				id := test.path[1:]
				rctx.URLParams.Add("id", id)
				req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
			}

			rec := httptest.NewRecorder()

			h.Redirect(rec, req)
			res := rec.Result()
			defer res.Body.Close()

			// Проверяем код
			assert.Equal(t, test.want.code, res.StatusCode)

			// Проверяем Location
			assert.Equal(t, test.want.location, res.Header.Get("Location"))

			// Проверяем Content-Type
			assert.Equal(t, test.want.contentType, res.Header.Get("Content-Type"))

			// Проверяем тело
			assert.Equal(t, test.want.body, rec.Body.String())
		})
	}
}
