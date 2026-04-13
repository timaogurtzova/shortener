package handler_test

import (
	"context"
	"errors"
)

// реализует интерфейс URLShortener для тестов
type mockURLShortener struct {
	CreateMockFunc      func(context.Context, string) (string, error)
	CreateBatchMockFunc func(context.Context, []string) ([]string, error)
	ResolveMockFunc     func(context.Context, string) (string, error)
}

func (m *mockURLShortener) Create(ctx context.Context, url string) (string, error) {
	if m.CreateMockFunc != nil {
		return m.CreateMockFunc(ctx, url)
	}
	return "", errors.New("not implemented")
}

func (m *mockURLShortener) CreateBatch(ctx context.Context, urls []string) ([]string, error) {
	if m.CreateBatchMockFunc != nil {
		return m.CreateBatchMockFunc(ctx, urls)
	}
	return nil, errors.New("not implemented")
}

func (m *mockURLShortener) Resolve(ctx context.Context, id string) (string, error) {
	if m.ResolveMockFunc != nil {
		return m.ResolveMockFunc(ctx, id)
	}
	return "", errors.New("not implemented")
}
