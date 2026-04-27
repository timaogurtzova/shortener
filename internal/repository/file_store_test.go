package repository_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/timaogurtzova/shortener/internal/repository"
)

type storedURLRecordJSON struct {
	UUID        string `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

type storedUserURLRecordJSON struct {
	UserID   string `json:"user_id,omitempty"`
	ShortURL string `json:"short_url"`
}

type storedFileDataJSON struct {
	URLs     []storedURLRecordJSON     `json:"urls"`
	UserURLs []storedUserURLRecordJSON `json:"user_urls,omitempty"`
}

type legacyStoredURLRecordJSON struct {
	UUID        string `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
	UserID      string `json:"user_id,omitempty"`
}

func TestFileStorePersistsAndRestoresURLs(t *testing.T) {
	storagePath := filepath.Join(t.TempDir(), "storage.json")

	store, err := repository.NewFileStore(storagePath)
	require.NoError(t, err)

	require.NoError(t, store.Store(context.Background(), "abc123", "http://yandex.ru", "user-1"))
	require.NoError(t, store.Store(context.Background(), "edVPg3ks", "http://ya.ru", "user-1"))

	data, err := os.ReadFile(storagePath)
	require.NoError(t, err)

	var fileData storedFileDataJSON
	require.NoError(t, json.Unmarshal(data, &fileData))
	require.Len(t, fileData.URLs, 2)
	assert.Equal(t, storedURLRecordJSON{
		UUID:        "1",
		ShortURL:    "abc123",
		OriginalURL: "http://yandex.ru",
	}, fileData.URLs[0])
	assert.Equal(t, storedURLRecordJSON{
		UUID:        "2",
		ShortURL:    "edVPg3ks",
		OriginalURL: "http://ya.ru",
	}, fileData.URLs[1])
	assert.Equal(t, []storedUserURLRecordJSON{
		{UserID: "user-1", ShortURL: "abc123"},
		{UserID: "user-1", ShortURL: "edVPg3ks"},
	}, fileData.UserURLs)

	restoredStore, err := repository.NewFileStore(storagePath)
	require.NoError(t, err)

	url, err := restoredStore.Load(context.Background(), "abc123")
	require.NoError(t, err)
	assert.Equal(t, "http://yandex.ru", url)

	url, err = restoredStore.Load(context.Background(), "edVPg3ks")
	require.NoError(t, err)
	assert.Equal(t, "http://ya.ru", url)

	userURLs, err := restoredStore.FindByUserID(context.Background(), "user-1")
	require.NoError(t, err)
	require.Len(t, userURLs, 2)
	assert.Equal(t, "abc123", userURLs[0].ShortID)
}

func TestFileStoreContinuesUUIDSequenceAfterReload(t *testing.T) {
	storagePath := filepath.Join(t.TempDir(), "storage.json")

	initialData := []legacyStoredURLRecordJSON{
		{
			UUID:        "3",
			ShortURL:    "dG56Hqxm",
			OriginalURL: "http://practicum.yandex.ru",
			UserID:      "legacy-user",
		},
	}

	file, err := os.Create(storagePath)
	require.NoError(t, err)

	encoder := json.NewEncoder(file)
	require.NoError(t, encoder.Encode(initialData))
	require.NoError(t, file.Close())

	store, err := repository.NewFileStore(storagePath)
	require.NoError(t, err)

	require.NoError(t, store.Store(context.Background(), "newID123", "http://example.com", "user-1"))

	data, err := os.ReadFile(storagePath)
	require.NoError(t, err)

	var fileData storedFileDataJSON
	require.NoError(t, json.Unmarshal(data, &fileData))
	require.Len(t, fileData.URLs, 2)
	assert.Equal(t, "4", fileData.URLs[1].UUID)
	assert.Equal(t, "newID123", fileData.URLs[1].ShortURL)
	assert.Equal(t, "http://example.com", fileData.URLs[1].OriginalURL)
	assert.Equal(t, []storedUserURLRecordJSON{
		{UserID: "legacy-user", ShortURL: "dG56Hqxm"},
		{UserID: "user-1", ShortURL: "newID123"},
	}, fileData.UserURLs)
}

func TestFileStoreBatchStorePersistsAndRestoresURLs(t *testing.T) {
	storagePath := filepath.Join(t.TempDir(), "storage.json")

	store, err := repository.NewFileStore(storagePath)
	require.NoError(t, err)

	err = store.BatchStore(context.Background(), []repository.BatchRecord{
		{ID: "abc123", OriginalURL: "http://yandex.ru", UserID: "user-1"},
		{ID: "edVPg3ks", OriginalURL: "http://ya.ru", UserID: "user-1"},
	})
	require.NoError(t, err)

	data, err := os.ReadFile(storagePath)
	require.NoError(t, err)

	var fileData storedFileDataJSON
	require.NoError(t, json.Unmarshal(data, &fileData))
	require.Len(t, fileData.URLs, 2)
	assert.Equal(t, storedURLRecordJSON{
		UUID:        "1",
		ShortURL:    "abc123",
		OriginalURL: "http://yandex.ru",
	}, fileData.URLs[0])
	assert.Equal(t, storedURLRecordJSON{
		UUID:        "2",
		ShortURL:    "edVPg3ks",
		OriginalURL: "http://ya.ru",
	}, fileData.URLs[1])
	assert.Equal(t, []storedUserURLRecordJSON{
		{UserID: "user-1", ShortURL: "abc123"},
		{UserID: "user-1", ShortURL: "edVPg3ks"},
	}, fileData.UserURLs)

	restoredStore, err := repository.NewFileStore(storagePath)
	require.NoError(t, err)

	url, err := restoredStore.Load(context.Background(), "abc123")
	require.NoError(t, err)
	assert.Equal(t, "http://yandex.ru", url)

	url, err = restoredStore.Load(context.Background(), "edVPg3ks")
	require.NoError(t, err)
	assert.Equal(t, "http://ya.ru", url)

	userURLs, err := restoredStore.FindByUserID(context.Background(), "user-1")
	require.NoError(t, err)
	require.Len(t, userURLs, 2)
}

func TestFileStoreStoreReturnsConflictForExistingOriginalURL(t *testing.T) {
	storagePath := filepath.Join(t.TempDir(), "storage.json")

	store, err := repository.NewFileStore(storagePath)
	require.NoError(t, err)

	require.NoError(t, store.Store(context.Background(), "abc123", "http://yandex.ru", "user-1"))

	err = store.Store(context.Background(), "def456", "http://yandex.ru", "user-2")
	require.Error(t, err)

	var conflictErr *repository.URLConflictError
	require.ErrorAs(t, err, &conflictErr)
	assert.Equal(t, "abc123", conflictErr.ShortID)

	userURLs, err := store.FindByUserID(context.Background(), "user-2")
	require.NoError(t, err)
	require.Len(t, userURLs, 1)
	assert.Equal(t, "abc123", userURLs[0].ShortID)

	data, err := os.ReadFile(storagePath)
	require.NoError(t, err)

	var fileData storedFileDataJSON
	require.NoError(t, json.Unmarshal(data, &fileData))
	require.Len(t, fileData.URLs, 1)
	assert.Equal(t, []storedUserURLRecordJSON{
		{UserID: "user-1", ShortURL: "abc123"},
		{UserID: "user-2", ShortURL: "abc123"},
	}, fileData.UserURLs)
}

func TestFileStoreLoadsLegacyArrayFormatWithoutDuplicatingURLs(t *testing.T) {
	storagePath := filepath.Join(t.TempDir(), "storage.json")

	initialData := []legacyStoredURLRecordJSON{
		{
			UUID:        "1",
			ShortURL:    "abc123",
			OriginalURL: "http://yandex.ru",
			UserID:      "user-1",
		},
		{
			UUID:        "2",
			ShortURL:    "abc123",
			OriginalURL: "http://yandex.ru",
			UserID:      "user-2",
		},
	}

	file, err := os.Create(storagePath)
	require.NoError(t, err)

	encoder := json.NewEncoder(file)
	require.NoError(t, encoder.Encode(initialData))
	require.NoError(t, file.Close())

	store, err := repository.NewFileStore(storagePath)
	require.NoError(t, err)

	userOneURLs, err := store.FindByUserID(context.Background(), "user-1")
	require.NoError(t, err)
	require.Len(t, userOneURLs, 1)

	userTwoURLs, err := store.FindByUserID(context.Background(), "user-2")
	require.NoError(t, err)
	require.Len(t, userTwoURLs, 1)

	require.NoError(t, store.Store(context.Background(), "new12345", "http://example.com", "user-3"))

	data, err := os.ReadFile(storagePath)
	require.NoError(t, err)

	var fileData storedFileDataJSON
	require.NoError(t, json.Unmarshal(data, &fileData))
	require.Len(t, fileData.URLs, 2)
	assert.Equal(t, []storedUserURLRecordJSON{
		{UserID: "user-1", ShortURL: "abc123"},
		{UserID: "user-2", ShortURL: "abc123"},
		{UserID: "user-3", ShortURL: "new12345"},
	}, fileData.UserURLs)
}
