package handler

import "errors"

// реализует интерфейс URLShortener для тестов
type mockURLShortener struct {
	CreateMockFunc  func(string) (string, error)
	ResolveMockFunc func(string) (string, error)
}

func (m *mockURLShortener) Create(url string) (string, error) {
	if m.CreateMockFunc != nil {
		return m.CreateMockFunc(url)
	}
	return "", errors.New("not implemented")
}

func (m *mockURLShortener) Resolve(id string) (string, error) {
	if m.ResolveMockFunc != nil {
		return m.ResolveMockFunc(id)
	}
	return "", errors.New("not implemented")
}
