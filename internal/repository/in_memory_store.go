package repository

import (
	"context"
	"sync"
)

// InMemoryStore потокобезопасное хранилище URL в памяти.
type InMemoryStore struct {
	mu            sync.RWMutex
	store         map[string]string
	shortIDsByURL map[string]string
}

// NewInMemoryStore создаёт новое in-memory хранилище.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		store:         make(map[string]string),
		shortIDsByURL: make(map[string]string),
	}
}

// Store сохраняет URL по ID.
func (s *InMemoryStore) Store(_ context.Context, id, url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.store[id]; exists {
		return ErrIDAlreadyExists
	}
	if shortID, exists := s.shortIDsByURL[url]; exists {
		return &URLConflictError{ShortID: shortID}
	}

	s.store[id] = url
	s.shortIDsByURL[url] = id
	return nil
}

// BatchStore сохраняет пакет URL за одну критическую секцию.
func (s *InMemoryStore) BatchStore(_ context.Context, records []BatchRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	seen := make(map[string]struct{}, len(records))
	seenOriginalURLs := make(map[string]string, len(records))
	for _, record := range records {
		if _, exists := seen[record.ID]; exists {
			return ErrIDAlreadyExists
		}
		if _, exists := s.store[record.ID]; exists {
			return ErrIDAlreadyExists
		}
		if shortID, exists := s.shortIDsByURL[record.OriginalURL]; exists {
			return &URLConflictError{ShortID: shortID}
		}
		if shortID, exists := seenOriginalURLs[record.OriginalURL]; exists {
			return &URLConflictError{ShortID: shortID}
		}
		seen[record.ID] = struct{}{}
		seenOriginalURLs[record.OriginalURL] = record.ID
	}

	for _, record := range records {
		s.store[record.ID] = record.OriginalURL
		s.shortIDsByURL[record.OriginalURL] = record.ID
	}

	return nil
}

// Load возвращает URL по ID.
func (s *InMemoryStore) Load(_ context.Context, id string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	v, ok := s.store[id]
	if !ok {
		return "", ErrNotFound
	}

	return v, nil
}
