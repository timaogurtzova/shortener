package repository

import (
	"errors"
	"sync"
)

// InMemoryStore потокобезопасное хранилище URL в памяти
type InMemoryStore struct {
	store sync.Map // id → url
}

// NewInMemoryStore создаёт новое in-memory хранилище
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{}
}

// Store сохраняет URL по ID
func (s *InMemoryStore) Store(id, url string) error {
	_, loaded := s.store.LoadOrStore(id, url)
	if loaded {
		return errors.New("id already exists")
	}
	return nil
}

// Load возвращает URL по ID
func (s *InMemoryStore) Load(id string) (string, error) {
	v, ok := s.store.Load(id)
	if !ok {
		return "", errors.New("not found")
	}
	return v.(string), nil
}
