package service_test

import (
	"errors"

	"github.com/timaogurtzova/shortener/internal/repository"
)

// mockURLRepository реализует repository.URLRepository для тестов
type mockURLRepository struct {
	storeFunc      func(id, url string) error
	batchStoreFunc func(records []repository.BatchRecord) error
	loadFunc       func(id string) (string, error)
}

func (m *mockURLRepository) Store(id, url string) error {
	if m.storeFunc != nil {
		return m.storeFunc(id, url)
	}
	return nil
}

func (m *mockURLRepository) BatchStore(records []repository.BatchRecord) error {
	if m.batchStoreFunc != nil {
		return m.batchStoreFunc(records)
	}
	return nil
}

func (m *mockURLRepository) Load(id string) (string, error) {
	if m.loadFunc != nil {
		return m.loadFunc(id)
	}
	return "", errors.New("not implemented")
}
