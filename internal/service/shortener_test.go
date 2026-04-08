package service_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/timaogurtzova/shortener/internal/repository"
	"github.com/timaogurtzova/shortener/internal/service"
)

func TestShortenerService_Create(t *testing.T) {
	tests := []struct {
		name          string
		mockStoreFunc func(id, url string) error
		wantErr       bool
	}{
		{
			name: "успешное создание",
			mockStoreFunc: func(id, url string) error {
				return nil
			},
			wantErr: false,
		},
		{
			name: "ошибка при сохранении",
			mockStoreFunc: func(id, url string) error {
				return errors.New("store error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockURLRepository{
				storeFunc: tt.mockStoreFunc,
			}
			svc := service.NewShortenerService(mockRepo)

			id, err := svc.Create("https://example.com")
			if tt.wantErr {
				assert.Error(t, err)
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
		mockLoad func(id string) (string, error)
		wantURL  string
		wantErr  bool
	}{
		{
			name: "успешное получение",
			mockLoad: func(id string) (string, error) {
				return "https://example.com", nil
			},
			wantURL: "https://example.com",
			wantErr: false,
		},
		{
			name: "id не найден",
			mockLoad: func(id string) (string, error) {
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

			url, err := svc.Resolve("abc123")
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
		mockBatchStoreFunc func(records []repository.BatchRecord) error
		wantErr            bool
		wantLen            int
	}{
		{
			name: "успешное пакетное создание",
			urls: []string{"https://example.com", "https://practicum.yandex.ru"},
			mockBatchStoreFunc: func(records []repository.BatchRecord) error {
				return nil
			},
			wantErr: false,
			wantLen: 2,
		},
		{
			name: "ошибка пакетного сохранения",
			urls: []string{"https://example.com", "https://practicum.yandex.ru"},
			mockBatchStoreFunc: func(records []repository.BatchRecord) error {
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

			ids, err := svc.CreateBatch(tt.urls)
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
