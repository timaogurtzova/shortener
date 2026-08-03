package service_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/timaogurtzova/shortener/internal/service"
)

func TestNormalizeOriginalURL(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		wantURL string
		wantErr error
	}{
		{
			name:    "normalizes surrounding spaces",
			rawURL:  "  https://example.com/path  ",
			wantURL: "https://example.com/path",
		},
		{
			name:    "rejects empty URL",
			rawURL:  " \t\n ",
			wantErr: service.ErrURLInvalid,
		},
		{
			name:    "accepts maximum URL length",
			rawURL:  strings.Repeat("a", service.MaxOriginalURLLength),
			wantURL: strings.Repeat("a", service.MaxOriginalURLLength),
		},
		{
			name:    "rejects URL over maximum length",
			rawURL:  strings.Repeat("a", service.MaxOriginalURLLength+1),
			wantErr: service.ErrURLTooLong,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			normalizedURL, err := service.NormalizeOriginalURL(test.rawURL)

			if test.wantErr != nil {
				require.ErrorIs(t, err, test.wantErr)
				assert.Empty(t, normalizedURL)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.wantURL, normalizedURL)
		})
	}
}

func TestShortenerServiceValidatesURLBeforeRepositoryCall(t *testing.T) {
	repositoryCalled := false
	repo := &mockURLRepository{
		storeFunc: func(context.Context, string, string, string) error {
			repositoryCalled = true
			return nil
		},
	}
	shortener := service.NewShortenerService(context.Background(), repo)
	t.Cleanup(func() {
		require.NoError(t, shortener.Close(context.Background()))
	})

	_, err := shortener.Create(
		context.Background(),
		strings.Repeat("a", service.MaxOriginalURLLength+1),
		"user-1",
	)

	require.ErrorIs(t, err, service.ErrURLTooLong)
	assert.False(t, repositoryCalled)
}
