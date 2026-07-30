package service_test

import (
	"context"
	"errors"

	"github.com/timaogurtzova/shortener/internal/model"
	"github.com/timaogurtzova/shortener/internal/repository"
)

// mockURLRepository реализует repository.URLRepository для тестов
type mockURLRepository struct {
	storeFunc       func(ctx context.Context, id, url, userID string) error
	batchStoreFunc  func(ctx context.Context, records []repository.BatchRecord) error
	loadFunc        func(ctx context.Context, id string) (string, error)
	findByUserFunc  func(ctx context.Context, userID string) ([]model.UserURL, error)
	markDeletedFunc func(ctx context.Context, userID string, shortIDs []string) error
	getStatsFunc    func(ctx context.Context) (model.Stats, error)
}

func (m *mockURLRepository) Store(ctx context.Context, id, url, userID string) error {
	if m.storeFunc != nil {
		return m.storeFunc(ctx, id, url, userID)
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

func (m *mockURLRepository) FindByUserID(ctx context.Context, userID string) ([]model.UserURL, error) {
	if m.findByUserFunc != nil {
		return m.findByUserFunc(ctx, userID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockURLRepository) MarkDeleted(ctx context.Context, userID string, shortIDs []string) error {
	if m.markDeletedFunc != nil {
		return m.markDeletedFunc(ctx, userID, shortIDs)
	}
	return nil
}

func (m *mockURLRepository) GetStats(ctx context.Context) (model.Stats, error) {
	if m.getStatsFunc != nil {
		return m.getStatsFunc(ctx)
	}
	return model.Stats{}, errors.New("not implemented")
}
