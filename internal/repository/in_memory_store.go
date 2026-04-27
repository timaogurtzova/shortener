package repository

import (
	"context"
	"sync"

	"github.com/timaogurtzova/shortener/internal/model"
)

// InMemoryStore потокобезопасное хранилище URL в памяти.
type InMemoryStore struct {
	mu            sync.RWMutex
	store         map[string]string
	shortIDsByURL map[string]string
	userShortIDs  map[string][]string
	userShortSet  map[string]map[string]struct{}
}

// NewInMemoryStore создаёт новое in-memory хранилище.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		store:         make(map[string]string),
		shortIDsByURL: make(map[string]string),
		userShortIDs:  make(map[string][]string),
		userShortSet:  make(map[string]map[string]struct{}),
	}
}

// Store сохраняет URL по ID.
func (s *InMemoryStore) Store(_ context.Context, id, url, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.store[id]; exists {
		return ErrIDAlreadyExists
	}
	if shortID, exists := s.shortIDsByURL[url]; exists {
		s.associateUserWithShortID(userID, shortID)
		return &URLConflictError{ShortID: shortID}
	}

	s.store[id] = url
	s.shortIDsByURL[url] = id
	s.associateUserWithShortID(userID, id)
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
		s.associateUserWithShortID(record.UserID, record.ID)
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

// FindByUserID возвращает все URL, ассоциированные с пользователем.
func (s *InMemoryStore) FindByUserID(_ context.Context, userID string) ([]model.UserURL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	shortIDs := s.userShortIDs[userID]
	result := make([]model.UserURL, 0, len(shortIDs))

	for _, shortID := range shortIDs {
		originalURL, exists := s.store[shortID]
		if !exists {
			continue
		}

		result = append(result, model.UserURL{
			ShortID:     shortID,
			OriginalURL: originalURL,
		})
	}

	return result, nil
}

func (s *InMemoryStore) associateUserWithShortID(userID, shortID string) {
	if userID == "" || shortID == "" {
		return
	}

	if _, exists := s.userShortSet[userID]; !exists {
		s.userShortSet[userID] = make(map[string]struct{})
	}

	if _, exists := s.userShortSet[userID][shortID]; exists {
		return
	}

	s.userShortSet[userID][shortID] = struct{}{}
	s.userShortIDs[userID] = append(s.userShortIDs[userID], shortID)
}
