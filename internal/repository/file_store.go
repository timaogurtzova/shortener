package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type storedURLRecord struct {
	UUID        string `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

type FileStore struct {
	mu            sync.RWMutex
	path          string
	urls          map[string]string
	shortIDsByURL map[string]string
	records       []storedURLRecord
	nextUUID      int
}

func NewFileStore(path string) (*FileStore, error) {
	store := &FileStore{
		path:          path,
		urls:          make(map[string]string),
		shortIDsByURL: make(map[string]string),
		nextUUID:      1,
	}

	if err := store.load(); err != nil {
		return nil, err
	}

	return store, nil
}

func (s *FileStore) Store(id, url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.urls[id]; exists {
		return ErrIDAlreadyExists
	}
	if shortID, exists := s.shortIDsByURL[url]; exists {
		return &URLConflictError{ShortID: shortID}
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

	if err := s.persist(); err != nil {
		delete(s.urls, id)
		delete(s.shortIDsByURL, url)
		s.records = s.records[:len(s.records)-1]
		s.nextUUID--
		return err
	}

	return nil
}

func (s *FileStore) BatchStore(records []BatchRecord) error {
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
		insertedRecords = append(insertedRecords, record)
	}

	if err := s.persist(); err != nil {
		for _, record := range insertedRecords {
			delete(s.urls, record.ID)
			delete(s.shortIDsByURL, record.OriginalURL)
		}
		s.records = s.records[:initialRecordsLen]
		s.nextUUID = initialNextUUID
		return err
	}

	return nil
}

func (s *FileStore) Load(id string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	url, ok := s.urls[id]
	if !ok {
		return "", ErrNotFound
	}

	return url, nil
}

func (s *FileStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	if len(strings.TrimSpace(string(data))) == 0 {
		return nil
	}

	var records []storedURLRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return err
	}

	for _, record := range records {
		if _, exists := s.urls[record.ShortURL]; exists {
			return fmt.Errorf("%w: %s", ErrIDAlreadyExists, record.ShortURL)
		}
		s.urls[record.ShortURL] = record.OriginalURL
		if _, exists := s.shortIDsByURL[record.OriginalURL]; !exists {
			s.shortIDsByURL[record.OriginalURL] = record.ShortURL
		}
	}

	s.records = append(s.records, records...)
	s.nextUUID = nextUUID(records)
	return nil
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
	if err := encoder.Encode(s.records); err != nil {
		_ = tmpFile.Close()
		return err
	}

	if err := tmpFile.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, s.path)
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
