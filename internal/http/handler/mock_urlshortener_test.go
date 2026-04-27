package handler_test

import (
	"context"
	"errors"

	"github.com/timaogurtzova/shortener/internal/model"
)

// реализует интерфейс URLShortener для тестов
type mockURLShortener struct {
	CreateMockFunc       func(context.Context, string, string) (string, error)
	CreateBatchMockFunc  func(context.Context, []string, string) ([]string, error)
	ResolveMockFunc      func(context.Context, string) (string, error)
	FindByUserIDMockFunc func(context.Context, string) ([]model.UserURL, error)
}

func (m *mockURLShortener) Create(ctx context.Context, url, userID string) (string, error) {
	if m.CreateMockFunc != nil {
		return m.CreateMockFunc(ctx, url, userID)
	}
	return "", errors.New("not implemented")
}

func (m *mockURLShortener) CreateBatch(ctx context.Context, urls []string, userID string) ([]string, error) {
	if m.CreateBatchMockFunc != nil {
		return m.CreateBatchMockFunc(ctx, urls, userID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockURLShortener) Resolve(ctx context.Context, id string) (string, error) {
	if m.ResolveMockFunc != nil {
		return m.ResolveMockFunc(ctx, id)
	}
	return "", errors.New("not implemented")
}

func (m *mockURLShortener) FindByUserID(ctx context.Context, userID string) ([]model.UserURL, error) {
	if m.FindByUserIDMockFunc != nil {
		return m.FindByUserIDMockFunc(ctx, userID)
	}
	return nil, errors.New("not implemented")
}
