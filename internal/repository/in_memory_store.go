package repository

import (
	"sync"
)

// InMemoryStore потокобезопасное хранилище URL в памяти.
type InMemoryStore struct {
	mu    sync.RWMutex
	store map[string]string
}

// NewInMemoryStore создаёт новое in-memory хранилище.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		store: make(map[string]string),
	}
}

// Store сохраняет URL по ID.
func (s *InMemoryStore) Store(id, url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.store[id]; exists {
		return ErrIDAlreadyExists
	}

	s.store[id] = url
	return nil
}

// BatchStore сохраняет пакет URL за одну критическую секцию.
func (s *InMemoryStore) BatchStore(records []BatchRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		if _, exists := seen[record.ID]; exists {
			return ErrIDAlreadyExists
		}
		if _, exists := s.store[record.ID]; exists {
			return ErrIDAlreadyExists
		}
		seen[record.ID] = struct{}{}
	}

	for _, record := range records {
		s.store[record.ID] = record.OriginalURL
	}

	return nil
}

// Load возвращает URL по ID.
func (s *InMemoryStore) Load(id string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	v, ok := s.store[id]
	if !ok {
		return "", ErrNotFound
	}

	return v, nil
}
