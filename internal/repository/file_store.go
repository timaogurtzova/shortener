package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/timaogurtzova/shortener/internal/model"
)

type storedURLRecord struct {
	UUID        string `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
	DeletedFlag bool   `json:"is_deleted,omitempty"`
}

type storedUserURLRecord struct {
	UserID   string `json:"user_id,omitempty"`
	ShortURL string `json:"short_url"`
}

type legacyStoredURLRecord struct {
	UUID        string `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
	UserID      string `json:"user_id,omitempty"`
	DeletedFlag bool   `json:"is_deleted,omitempty"`
}

type storedFileData struct {
	URLs     []storedURLRecord     `json:"urls"`
	UserURLs []storedUserURLRecord `json:"user_urls,omitempty"`
}

// FileStore хранит сокращённые URL в файле и восстанавливает данные при старте.
type FileStore struct {
	mu            sync.RWMutex
	path          string
	urls          map[string]string
	deletedShorts map[string]struct{}
	shortIDsByURL map[string]string
	userShortIDs  map[string][]string
	userShortSet  map[string]map[string]struct{}
	records       []storedURLRecord
	userRecords   []storedUserURLRecord
	nextUUID      int
}

// NewFileStore создаёт файловое хранилище и загружает ранее сохранённые записи.
func NewFileStore(path string) (*FileStore, error) {
	store := &FileStore{
		path:          path,
		urls:          make(map[string]string),
		deletedShorts: make(map[string]struct{}),
		shortIDsByURL: make(map[string]string),
		userShortIDs:  make(map[string][]string),
		userShortSet:  make(map[string]map[string]struct{}),
		nextUUID:      1,
	}

	if err := store.load(); err != nil {
		return nil, err
	}

	return store, nil
}

// Store сохраняет исходный URL по короткому идентификатору в файловом хранилище.
func (s *FileStore) Store(_ context.Context, id, url, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.urls[id]; exists {
		return ErrIDAlreadyExists
	}
	if shortID, exists := s.shortIDsByURL[url]; exists {
		return s.persistUserAssociation(userID, shortID)
	}

	record := storedURLRecord{
		UUID:        strconv.Itoa(s.nextUUID),
		ShortURL:    id,
		OriginalURL: url,
	}

	s.urls[id] = url
	s.shortIDsByURL[url] = id
	s.records = append(s.records, record)
	s.nextUUID++
	userAssociationAdded := s.addUserAssociation(userID, id)

	if err := s.persist(); err != nil {
		delete(s.urls, id)
		delete(s.shortIDsByURL, url)
		if userAssociationAdded {
			s.removeUserAssociation(userID, id)
		}
		s.records = s.records[:len(s.records)-1]
		s.nextUUID--
		return err
	}

	return nil
}

func (s *FileStore) persistUserAssociation(userID, shortID string) error {
	added := s.addUserAssociation(userID, shortID)
	if !added {
		return &URLConflictError{ShortID: shortID}
	}

	if err := s.persist(); err != nil {
		s.removeUserAssociation(userID, shortID)
		return err
	}

	return &URLConflictError{ShortID: shortID}
}

// BatchStore сохраняет пакет URL в файловом хранилище.
func (s *FileStore) BatchStore(_ context.Context, records []BatchRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	seen := make(map[string]struct{}, len(records))
	seenOriginalURLs := make(map[string]string, len(records))
	for _, record := range records {
		if _, exists := seen[record.ID]; exists {
			return ErrIDAlreadyExists
		}
		if _, exists := s.urls[record.ID]; exists {
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

	initialRecordsLen := len(s.records)
	initialNextUUID := s.nextUUID

	insertedRecords := make([]BatchRecord, 0, len(records))
	for _, record := range records {
		storedRecord := storedURLRecord{
			UUID:        strconv.Itoa(s.nextUUID),
			ShortURL:    record.ID,
			OriginalURL: record.OriginalURL,
		}

		s.urls[record.ID] = record.OriginalURL
		s.shortIDsByURL[record.OriginalURL] = record.ID
		s.records = append(s.records, storedRecord)
		s.nextUUID++
		s.addUserAssociation(record.UserID, record.ID)
		insertedRecords = append(insertedRecords, record)
	}

	if err := s.persist(); err != nil {
		for _, record := range insertedRecords {
			delete(s.urls, record.ID)
			delete(s.shortIDsByURL, record.OriginalURL)
			s.removeUserAssociation(record.UserID, record.ID)
		}
		s.records = s.records[:initialRecordsLen]
		s.nextUUID = initialNextUUID
		return err
	}

	return nil
}

// Load возвращает исходный URL из файлового хранилища по короткому идентификатору.
func (s *FileStore) Load(_ context.Context, id string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	url, ok := s.urls[id]
	if !ok {
		return "", ErrNotFound
	}
	if s.isDeleted(id) {
		return "", ErrDeleted
	}

	return url, nil
}

// FindByUserID возвращает все URL, ассоциированные с пользователем.
func (s *FileStore) FindByUserID(_ context.Context, userID string) ([]model.UserURL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	shortIDs := s.userShortIDs[userID]
	result := make([]model.UserURL, 0, len(shortIDs))

	for _, shortID := range shortIDs {
		originalURL, exists := s.urls[shortID]
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

// GetStats возвращает количество сокращённых URL и уникальных пользователей.
func (s *FileStore) GetStats(_ context.Context) (model.Stats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return model.Stats{
		URLs:  len(s.urls),
		Users: len(s.userShortSet),
	}, nil
}

// MarkDeleted помечает принадлежащие пользователю короткие URL как удалённые.
func (s *FileStore) MarkDeleted(_ context.Context, userID string, shortIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ownedShortIDs := s.userShortSet[userID]
	if len(ownedShortIDs) == 0 {
		return nil
	}

	changed := make([]string, 0, len(shortIDs))
	for _, shortID := range shortIDs {
		if _, exists := ownedShortIDs[shortID]; !exists {
			continue
		}
		if _, exists := s.urls[shortID]; !exists {
			continue
		}
		if s.isDeleted(shortID) {
			continue
		}

		s.setDeleted(shortID, true)
		changed = append(changed, shortID)
	}

	if len(changed) == 0 {
		return nil
	}

	if err := s.persist(); err != nil {
		for _, shortID := range changed {
			s.setDeleted(shortID, false)
		}

		return err
	}

	return nil
}

func (s *FileStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	trimmedData := strings.TrimSpace(string(data))
	if trimmedData == "" {
		return nil
	}

	if strings.HasPrefix(trimmedData, "{") {
		var fileData storedFileData
		if err := json.Unmarshal(data, &fileData); err != nil {
			return err
		}

		return s.loadNormalized(fileData)
	}

	var records []legacyStoredURLRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return err
	}

	return s.loadLegacy(records)
}

func (s *FileStore) persist() error {
	dir := filepath.Dir(s.path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	tmpFile, err := os.CreateTemp(dir, filepath.Base(s.path)+".tmp-*")
	if err != nil {
		return err
	}

	tmpName := tmpFile.Name()
	defer os.Remove(tmpName)

	encoder := json.NewEncoder(tmpFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(storedFileData{
		URLs:     s.records,
		UserURLs: s.userRecords,
	}); err != nil {
		_ = tmpFile.Close()
		return err
	}

	if err := tmpFile.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, s.path)
}

func (s *FileStore) addUserAssociation(userID, shortID string) bool {
	if userID == "" || shortID == "" {
		return false
	}

	if _, exists := s.userShortSet[userID]; !exists {
		s.userShortSet[userID] = make(map[string]struct{})
	}

	if _, exists := s.userShortSet[userID][shortID]; exists {
		return false
	}

	s.userShortSet[userID][shortID] = struct{}{}
	s.userShortIDs[userID] = append(s.userShortIDs[userID], shortID)
	s.userRecords = append(s.userRecords, storedUserURLRecord{
		UserID:   userID,
		ShortURL: shortID,
	})
	return true
}

func (s *FileStore) removeUserAssociation(userID, shortID string) {
	if userID == "" || shortID == "" {
		return
	}

	userShortSet, exists := s.userShortSet[userID]
	if !exists {
		return
	}

	if _, exists := userShortSet[shortID]; !exists {
		return
	}

	delete(userShortSet, shortID)
	shortIDs := s.userShortIDs[userID]
	for i, existingShortID := range shortIDs {
		if existingShortID != shortID {
			continue
		}

		s.userShortIDs[userID] = append(shortIDs[:i], shortIDs[i+1:]...)
		break
	}

	if len(userShortSet) == 0 {
		delete(s.userShortSet, userID)
		delete(s.userShortIDs, userID)
	}

	for i := len(s.userRecords) - 1; i >= 0; i-- {
		record := s.userRecords[i]
		if record.UserID != userID || record.ShortURL != shortID {
			continue
		}

		s.userRecords = append(s.userRecords[:i], s.userRecords[i+1:]...)
		break
	}
}

func nextUUID(records []storedURLRecord) int {
	maxUUID := len(records)

	for _, record := range records {
		uuid, err := strconv.Atoi(record.UUID)
		if err != nil {
			continue
		}
		if uuid > maxUUID {
			maxUUID = uuid
		}
	}

	return maxUUID + 1
}

func nextUUIDFromLegacy(records []legacyStoredURLRecord) int {
	maxUUID := len(records)

	for _, record := range records {
		uuid, err := strconv.Atoi(record.UUID)
		if err != nil {
			continue
		}
		if uuid > maxUUID {
			maxUUID = uuid
		}
	}

	return maxUUID + 1
}

func (s *FileStore) loadNormalized(data storedFileData) error {
	for _, record := range data.URLs {
		if err := s.loadURLRecord(record); err != nil {
			return err
		}
	}

	for _, record := range data.UserURLs {
		if _, exists := s.urls[record.ShortURL]; !exists {
			return fmt.Errorf("user association references unknown short url: %s", record.ShortURL)
		}

		s.addUserAssociation(record.UserID, record.ShortURL)
	}

	s.records = append(s.records, data.URLs...)
	s.nextUUID = nextUUID(data.URLs)
	return nil
}

func (s *FileStore) loadLegacy(records []legacyStoredURLRecord) error {
	normalizedRecords := make([]storedURLRecord, 0, len(records))

	for _, record := range records {
		if _, exists := s.urls[record.ShortURL]; !exists {
			normalizedRecord := storedURLRecord{
				UUID:        record.UUID,
				ShortURL:    record.ShortURL,
				OriginalURL: record.OriginalURL,
			}
			if err := s.loadURLRecord(normalizedRecord); err != nil {
				return err
			}

			normalizedRecords = append(normalizedRecords, normalizedRecord)
		} else if s.urls[record.ShortURL] != record.OriginalURL {
			return fmt.Errorf("%w: %s", ErrIDAlreadyExists, record.ShortURL)
		}

		s.addUserAssociation(record.UserID, record.ShortURL)
	}

	s.records = append(s.records, normalizedRecords...)
	s.nextUUID = nextUUIDFromLegacy(records)
	return nil
}

func (s *FileStore) loadURLRecord(record storedURLRecord) error {
	if existingOriginalURL, exists := s.urls[record.ShortURL]; exists {
		if existingOriginalURL != record.OriginalURL {
			return fmt.Errorf("%w: %s", ErrIDAlreadyExists, record.ShortURL)
		}
		return nil
	}

	if existingShortID, exists := s.shortIDsByURL[record.OriginalURL]; exists && existingShortID != record.ShortURL {
		return &URLConflictError{ShortID: existingShortID}
	}

	s.urls[record.ShortURL] = record.OriginalURL
	if _, exists := s.shortIDsByURL[record.OriginalURL]; !exists {
		s.shortIDsByURL[record.OriginalURL] = record.ShortURL
	}
	if record.DeletedFlag {
		s.deletedShorts[record.ShortURL] = struct{}{}
	}

	return nil
}

func (s *FileStore) isDeleted(shortID string) bool {
	_, exists := s.deletedShorts[shortID]
	return exists
}

func (s *FileStore) setDeleted(shortID string, deleted bool) {
	if deleted {
		s.deletedShorts[shortID] = struct{}{}
	} else {
		delete(s.deletedShorts, shortID)
	}

	for i := range s.records {
		if s.records[i].ShortURL != shortID {
			continue
		}

		s.records[i].DeletedFlag = deleted
		return
	}
}
