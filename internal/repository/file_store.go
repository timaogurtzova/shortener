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
	mu       sync.RWMutex
	path     string
	urls     map[string]string
	records  []storedURLRecord
	nextUUID int
}

func NewFileStore(path string) (*FileStore, error) {
	store := &FileStore{
		path:     path,
		urls:     make(map[string]string),
		nextUUID: 1,
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
		return errors.New("id already exists")
	}

	record := storedURLRecord{
		UUID:        strconv.Itoa(s.nextUUID),
		ShortURL:    id,
		OriginalURL: url,
	}

	s.urls[id] = url
	s.records = append(s.records, record)
	s.nextUUID++

	if err := s.persist(); err != nil {
		delete(s.urls, id)
		s.records = s.records[:len(s.records)-1]
		s.nextUUID--
		return err
	}

	return nil
}

func (s *FileStore) Load(id string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	url, ok := s.urls[id]
	if !ok {
		return "", errors.New("not found")
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
			return fmt.Errorf("duplicate short url in file storage: %s", record.ShortURL)
		}
		s.urls[record.ShortURL] = record.OriginalURL
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
