package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/timaogurtzova/shortener/internal/model"
	"github.com/timaogurtzova/shortener/internal/repository"
	"github.com/timaogurtzova/shortener/internal/service"
)

func TestShortenerService_Create(t *testing.T) {
	tests := []struct {
		name          string
		mockStoreFunc func(ctx context.Context, id, url, userID string) error
		wantID        string
		wantErr       bool
		wantConflict  bool
	}{
		{
			name: "успешное создание",
			mockStoreFunc: func(ctx context.Context, id, url, userID string) error {
				assert.Equal(t, "user-1", userID)
				return nil
			},
			wantErr:      false,
			wantConflict: false,
		},
		{
			name: "ошибка при сохранении",
			mockStoreFunc: func(ctx context.Context, id, url, userID string) error {
				return errors.New("store error")
			},
			wantErr:      true,
			wantConflict: false,
		},
		{
			name: "исходный url уже сокращён",
			mockStoreFunc: func(ctx context.Context, id, url, userID string) error {
				return &repository.URLConflictError{ShortID: "abc123"}
			},
			wantID:       "abc123",
			wantErr:      true,
			wantConflict: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockURLRepository{
				storeFunc: tt.mockStoreFunc,
			}
			svc := service.NewShortenerService(mockRepo)

			id, err := svc.Create(context.Background(), "https://example.com", "user-1")
			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantConflict {
					assert.ErrorIs(t, err, service.ErrURLAlreadyExists)
					assert.Equal(t, tt.wantID, id)
				}
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, id)
			}
		})
	}
}

func TestShortenerService_Resolve(t *testing.T) {
	tests := []struct {
		name     string
		mockLoad func(ctx context.Context, id string) (string, error)
		wantURL  string
		wantErr  bool
	}{
		{
			name: "успешное получение",
			mockLoad: func(ctx context.Context, id string) (string, error) {
				return "https://example.com", nil
			},
			wantURL: "https://example.com",
			wantErr: false,
		},
		{
			name: "id не найден",
			mockLoad: func(ctx context.Context, id string) (string, error) {
				return "", errors.New("not found")
			},
			wantURL: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockURLRepository{
				loadFunc: tt.mockLoad,
			}
			svc := service.NewShortenerService(mockRepo)

			url, err := svc.Resolve(context.Background(), "abc123")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantURL, url)
			}
		})
	}
}

func TestShortenerService_CreateBatch(t *testing.T) {
	tests := []struct {
		name               string
		urls               []string
		mockBatchStoreFunc func(ctx context.Context, records []repository.BatchRecord) error
		wantErr            bool
		wantLen            int
	}{
		{
			name: "успешное пакетное создание",
			urls: []string{"https://example.com", "https://practicum.yandex.ru"},
			mockBatchStoreFunc: func(ctx context.Context, records []repository.BatchRecord) error {
				return nil
			},
			wantErr: false,
			wantLen: 2,
		},
		{
			name: "ошибка пакетного сохранения",
			urls: []string{"https://example.com", "https://practicum.yandex.ru"},
			mockBatchStoreFunc: func(ctx context.Context, records []repository.BatchRecord) error {
				return errors.New("batch store error")
			},
			wantErr: true,
			wantLen: 0,
		},
		{
			name:    "пустой батч",
			urls:    nil,
			wantErr: true,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockURLRepository{
				batchStoreFunc: tt.mockBatchStoreFunc,
			}
			svc := service.NewShortenerService(mockRepo)

			ids, err := svc.CreateBatch(context.Background(), tt.urls, "user-1")
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Len(t, ids, tt.wantLen)
			for _, id := range ids {
				assert.NotEmpty(t, id)
			}
		})
	}
}

func TestShortenerService_FindByUserID(t *testing.T) {
	expected := []model.UserURL{
		{ShortID: "abc123", OriginalURL: "https://example.com"},
	}

	mockRepo := &mockURLRepository{
		findByUserFunc: func(ctx context.Context, userID string) ([]model.UserURL, error) {
			assert.Equal(t, "user-1", userID)
			return expected, nil
		},
	}

	svc := service.NewShortenerService(mockRepo)

	actual, err := svc.FindByUserID(context.Background(), "user-1")
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}
