package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/timaogurtzova/shortener/internal/http/handler"
	"github.com/timaogurtzova/shortener/internal/service"
)

type mockHealthChecker struct {
	pingFunc func(ctx context.Context) error
}

func (m *mockHealthChecker) Ping(ctx context.Context) error {
	if m.pingFunc != nil {
		return m.pingFunc(ctx)
	}

	return nil
}

func TestPingHandler(t *testing.T) {
	tests := []struct {
		name     string
		checker  service.HealthChecker
		wantCode int
		wantBody string
	}{
		{
			name: "успешная проверка базы данных",
			checker: &mockHealthChecker{
				pingFunc: func(ctx context.Context) error {
					return nil
				},
			},
			wantCode: http.StatusOK,
			wantBody: "",
		},
		{
			name:     "база данных не настроена",
			checker:  nil,
			wantCode: http.StatusInternalServerError,
			wantBody: "internal server error\n",
		},
		{
			name: "ошибка при проверке базы данных",
			checker: &mockHealthChecker{
				pingFunc: func(ctx context.Context) error {
					return errors.New("ping failed")
				},
			},
			wantCode: http.StatusInternalServerError,
			wantBody: "internal server error\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pingHandler := handler.NewPingHandler(test.checker)
			req := httptest.NewRequest(http.MethodGet, "/ping", nil)
			rec := httptest.NewRecorder()

			pingHandler.Ping(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			assert.Equal(t, test.wantCode, res.StatusCode)
			assert.Equal(t, test.wantBody, rec.Body.String())
		})
	}
}
