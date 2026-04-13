package service_test

import (
	"context"
	"errors"

	"github.com/timaogurtzova/shortener/internal/repository"
)

// mockURLRepository реализует repository.URLRepository для тестов
type mockURLRepository struct {
	storeFunc      func(ctx context.Context, id, url string) error
	batchStoreFunc func(ctx context.Context, records []repository.BatchRecord) error
	loadFunc       func(ctx context.Context, id string) (string, error)
}

func (m *mockURLRepository) Store(ctx context.Context, id, url string) error {
	if m.storeFunc != nil {
		return m.storeFunc(ctx, id, url)
	}
	return nil
}

func (m *mockURLRepository) BatchStore(ctx context.Context, records []repository.BatchRecord) error {
	if m.batchStoreFunc != nil {
		return m.batchStoreFunc(ctx, records)
	}
	return nil
}

func (m *mockURLRepository) Load(ctx context.Context, id string) (string, error) {
	if m.loadFunc != nil {
		return m.loadFunc(ctx, id)
	}
	return "", errors.New("not implemented")
}
