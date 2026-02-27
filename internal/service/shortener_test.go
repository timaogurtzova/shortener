package service

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
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
			svc := NewShortenerService(mockRepo)

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
			svc := NewShortenerService(mockRepo)

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
