package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/timaogurtzova/shortener/internal/http/handler"
	"github.com/timaogurtzova/shortener/internal/model"
)

func TestUserHandlerGetUserURLs(t *testing.T) {
	tests := []struct {
		name       string
		cookieUser string
		mock       func() *mockURLShortener
		wantCode   int
		wantBody   string
	}{
		{
			name:       "успешно возвращает url пользователя",
			cookieUser: "user-1",
			mock: func() *mockURLShortener {
				return &mockURLShortener{
					FindByUserIDMockFunc: func(ctx context.Context, userID string) ([]model.UserURL, error) {
						assert.Equal(t, "user-1", userID)
						return []model.UserURL{
							{ShortID: "abc123", OriginalURL: "https://example.com"},
						}, nil
					},
				}
			},
			wantCode: http.StatusOK,
			wantBody: `[{"short_url":"http://localhost:8080/abc123","original_url":"https://example.com"}]`,
		},
		{
			name:       "нет url пользователя",
			cookieUser: "",
			mock: func() *mockURLShortener {
				return &mockURLShortener{
					FindByUserIDMockFunc: func(ctx context.Context, userID string) ([]model.UserURL, error) {
						assert.NotEmpty(t, userID)
						return nil, nil
					},
				}
			},
			wantCode: http.StatusNoContent,
			wantBody: "",
		},
		{
			name:       "cookie без user id",
			cookieUser: "__empty__",
			mock: func() *mockURLShortener {
				return &mockURLShortener{}
			},
			wantCode: http.StatusUnauthorized,
			wantBody: "unauthorized\n",
		},
		{
			name:       "ошибка сервиса",
			cookieUser: "user-1",
			mock: func() *mockURLShortener {
				return &mockURLShortener{
					FindByUserIDMockFunc: func(ctx context.Context, userID string) ([]model.UserURL, error) {
						return nil, errors.New("service error")
					},
				}
			},
			wantCode: http.StatusInternalServerError,
			wantBody: "internal server error\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authenticator := newTestAuthenticator(t)
			h := handler.NewUserHandler(test.mock(), "http://localhost:8080", authenticator)

			req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
			if test.cookieUser == "__empty__" {
				cookie, err := authenticator.NewCookie("")
				require.NoError(t, err)
				req.AddCookie(cookie)
			} else if test.cookieUser != "" {
				cookie, err := authenticator.NewCookie(test.cookieUser)
				require.NoError(t, err)
				req.AddCookie(cookie)
			}

			rec := httptest.NewRecorder()
			h.GetUserURLs(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			body, err := io.ReadAll(res.Body)
			require.NoError(t, err)

			assert.Equal(t, test.wantCode, res.StatusCode)

			if test.wantCode == http.StatusOK {
				var actual []map[string]string
				require.NoError(t, json.Unmarshal(body, &actual))

				var expected []map[string]string
				require.NoError(t, json.Unmarshal([]byte(test.wantBody), &expected))
				assert.Equal(t, expected, actual)
				assert.Equal(t, "application/json", res.Header.Get("Content-Type"))
				return
			}

			assert.Equal(t, test.wantBody, string(body))
		})
	}
}
