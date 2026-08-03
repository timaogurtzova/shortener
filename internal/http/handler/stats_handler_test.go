package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/timaogurtzova/shortener/internal/http/handler"
	"github.com/timaogurtzova/shortener/internal/model"
)

type mockStatsProvider struct {
	getStatsFunc func(ctx context.Context) (model.Stats, error)
}

func (m *mockStatsProvider) GetStats(ctx context.Context) (model.Stats, error) {
	if m.getStatsFunc != nil {
		return m.getStatsFunc(ctx)
	}

	return model.Stats{}, nil
}

func TestStatsHandlerReturnsStatsForTrustedIP(t *testing.T) {
	statsHandler, err := handler.NewStatsHandler(
		&mockStatsProvider{
			getStatsFunc: func(ctx context.Context) (model.Stats, error) {
				return model.Stats{URLs: 12, Users: 3}, nil
			},
		},
		"192.168.10.0/24",
	)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
	req.Header.Set("X-Real-IP", "192.168.10.42")
	rec := httptest.NewRecorder()

	statsHandler.GetStats(rec, req)

	res := rec.Result()
	defer res.Body.Close()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "application/json", res.Header.Get("Content-Type"))
	assert.JSONEq(t, `{"urls":12,"users":3}`, rec.Body.String())
}

func TestStatsHandlerAcceptsTrustedIPv6(t *testing.T) {
	statsHandler, err := handler.NewStatsHandler(&mockStatsProvider{}, "2001:db8::/32")
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
	req.Header.Set("X-Real-IP", "2001:db8::1234")
	rec := httptest.NewRecorder()

	statsHandler.GetStats(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestStatsHandlerForbidsUntrustedRequests(t *testing.T) {
	tests := []struct {
		name          string
		trustedSubnet string
		realIP        string
	}{
		{
			name:          "IP is outside trusted subnet",
			trustedSubnet: "192.168.10.0/24",
			realIP:        "192.168.11.1",
		},
		{
			name:          "X-Real-IP is missing",
			trustedSubnet: "192.168.10.0/24",
		},
		{
			name:          "X-Real-IP is malformed",
			trustedSubnet: "192.168.10.0/24",
			realIP:        "not-an-ip",
		},
		{
			name:   "trusted subnet is empty",
			realIP: "192.168.10.42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			statsHandler, err := handler.NewStatsHandler(
				&mockStatsProvider{
					getStatsFunc: func(ctx context.Context) (model.Stats, error) {
						called = true
						return model.Stats{}, nil
					},
				},
				tt.trustedSubnet,
			)
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
			if tt.realIP != "" {
				req.Header.Set("X-Real-IP", tt.realIP)
			}
			rec := httptest.NewRecorder()

			statsHandler.GetStats(rec, req)

			assert.Equal(t, http.StatusForbidden, rec.Code)
			assert.Equal(t, "forbidden\n", rec.Body.String())
			assert.False(t, called)
		})
	}
}

func TestNewStatsHandlerRejectsMalformedTrustedSubnet(t *testing.T) {
	statsHandler, err := handler.NewStatsHandler(&mockStatsProvider{}, "192.168.10.0")

	require.Error(t, err)
	assert.Nil(t, statsHandler)
	assert.Contains(t, err.Error(), "parse trusted subnet")
}

func TestStatsHandlerReturnsInternalServerError(t *testing.T) {
	wantErr := errors.New("stats failed")
	statsHandler, err := handler.NewStatsHandler(
		&mockStatsProvider{
			getStatsFunc: func(ctx context.Context) (model.Stats, error) {
				return model.Stats{}, wantErr
			},
		},
		"10.0.0.0/8",
	)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
	req.Header.Set("X-Real-IP", "10.1.2.3")
	rec := httptest.NewRecorder()

	statsHandler.GetStats(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "internal server error\n", rec.Body.String())
}
