package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryStoreBatchStore(t *testing.T) {
	store := NewInMemoryStore()

	err := store.BatchStore(context.Background(), []BatchRecord{
		{ID: "abc123", OriginalURL: "http://yandex.ru"},
		{ID: "edVPg3ks", OriginalURL: "http://ya.ru"},
	})
	require.NoError(t, err)

	url, err := store.Load(context.Background(), "abc123")
	require.NoError(t, err)
	assert.Equal(t, "http://yandex.ru", url)

	url, err = store.Load(context.Background(), "edVPg3ks")
	require.NoError(t, err)
	assert.Equal(t, "http://ya.ru", url)
}

func TestInMemoryStoreStoreReturnsConflictForExistingOriginalURL(t *testing.T) {
	store := NewInMemoryStore()

	require.NoError(t, store.Store(context.Background(), "abc123", "http://yandex.ru"))

	err := store.Store(context.Background(), "def456", "http://yandex.ru")
	require.Error(t, err)

	var conflictErr *URLConflictError
	require.ErrorAs(t, err, &conflictErr)
	assert.Equal(t, "abc123", conflictErr.ShortID)
}
