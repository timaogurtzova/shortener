package repository

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileStorePersistsAndRestoresURLs(t *testing.T) {
	storagePath := filepath.Join(t.TempDir(), "storage.json")

	store, err := NewFileStore(storagePath)
	require.NoError(t, err)

	require.NoError(t, store.Store(context.Background(), "abc123", "http://yandex.ru"))
	require.NoError(t, store.Store(context.Background(), "edVPg3ks", "http://ya.ru"))

	data, err := os.ReadFile(storagePath)
	require.NoError(t, err)

	var records []storedURLRecord
	require.NoError(t, json.Unmarshal(data, &records))
	require.Len(t, records, 2)
	assert.Equal(t, storedURLRecord{
		UUID:        "1",
		ShortURL:    "abc123",
		OriginalURL: "http://yandex.ru",
	}, records[0])
	assert.Equal(t, storedURLRecord{
		UUID:        "2",
		ShortURL:    "edVPg3ks",
		OriginalURL: "http://ya.ru",
	}, records[1])

	restoredStore, err := NewFileStore(storagePath)
	require.NoError(t, err)

	url, err := restoredStore.Load(context.Background(), "abc123")
	require.NoError(t, err)
	assert.Equal(t, "http://yandex.ru", url)

	url, err = restoredStore.Load(context.Background(), "edVPg3ks")
	require.NoError(t, err)
	assert.Equal(t, "http://ya.ru", url)
}

func TestFileStoreContinuesUUIDSequenceAfterReload(t *testing.T) {
	storagePath := filepath.Join(t.TempDir(), "storage.json")

	initialData := []storedURLRecord{
		{
			UUID:        "3",
			ShortURL:    "dG56Hqxm",
			OriginalURL: "http://practicum.yandex.ru",
		},
	}

	file, err := os.Create(storagePath)
	require.NoError(t, err)

	encoder := json.NewEncoder(file)
	require.NoError(t, encoder.Encode(initialData))
	require.NoError(t, file.Close())

	store, err := NewFileStore(storagePath)
	require.NoError(t, err)

	require.NoError(t, store.Store(context.Background(), "newID123", "http://example.com"))

	data, err := os.ReadFile(storagePath)
	require.NoError(t, err)

	var records []storedURLRecord
	require.NoError(t, json.Unmarshal(data, &records))
	require.Len(t, records, 2)
	assert.Equal(t, "4", records[1].UUID)
	assert.Equal(t, "newID123", records[1].ShortURL)
	assert.Equal(t, "http://example.com", records[1].OriginalURL)
}

func TestFileStoreBatchStorePersistsAndRestoresURLs(t *testing.T) {
	storagePath := filepath.Join(t.TempDir(), "storage.json")

	store, err := NewFileStore(storagePath)
	require.NoError(t, err)

	err = store.BatchStore(context.Background(), []BatchRecord{
		{ID: "abc123", OriginalURL: "http://yandex.ru"},
		{ID: "edVPg3ks", OriginalURL: "http://ya.ru"},
	})
	require.NoError(t, err)

	data, err := os.ReadFile(storagePath)
	require.NoError(t, err)

	var records []storedURLRecord
	require.NoError(t, json.Unmarshal(data, &records))
	require.Len(t, records, 2)
	assert.Equal(t, storedURLRecord{
		UUID:        "1",
		ShortURL:    "abc123",
		OriginalURL: "http://yandex.ru",
	}, records[0])
	assert.Equal(t, storedURLRecord{
		UUID:        "2",
		ShortURL:    "edVPg3ks",
		OriginalURL: "http://ya.ru",
	}, records[1])

	restoredStore, err := NewFileStore(storagePath)
	require.NoError(t, err)

	url, err := restoredStore.Load(context.Background(), "abc123")
	require.NoError(t, err)
	assert.Equal(t, "http://yandex.ru", url)

	url, err = restoredStore.Load(context.Background(), "edVPg3ks")
	require.NoError(t, err)
	assert.Equal(t, "http://ya.ru", url)
}

func TestFileStoreStoreReturnsConflictForExistingOriginalURL(t *testing.T) {
	storagePath := filepath.Join(t.TempDir(), "storage.json")

	store, err := NewFileStore(storagePath)
	require.NoError(t, err)

	require.NoError(t, store.Store(context.Background(), "abc123", "http://yandex.ru"))

	err = store.Store(context.Background(), "def456", "http://yandex.ru")
	require.Error(t, err)

	var conflictErr *URLConflictError
	require.ErrorAs(t, err, &conflictErr)
	assert.Equal(t, "abc123", conflictErr.ShortID)
}
