package repository_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/timaogurtzova/shortener/internal/repository"
)

func TestInMemoryStoreBatchStore(t *testing.T) {
	store := repository.NewInMemoryStore()

	err := store.BatchStore(context.Background(), []repository.BatchRecord{
		{ID: "abc123", OriginalURL: "http://yandex.ru", UserID: "user-1"},
		{ID: "edVPg3ks", OriginalURL: "http://ya.ru", UserID: "user-1"},
	})
	require.NoError(t, err)

	url, err := store.Load(context.Background(), "abc123")
	require.NoError(t, err)
	assert.Equal(t, "http://yandex.ru", url)

	url, err = store.Load(context.Background(), "edVPg3ks")
	require.NoError(t, err)
	assert.Equal(t, "http://ya.ru", url)

	userURLs, err := store.FindByUserID(context.Background(), "user-1")
	require.NoError(t, err)
	require.Len(t, userURLs, 2)
	assert.Equal(t, "abc123", userURLs[0].ShortID)
	assert.Equal(t, "http://yandex.ru", userURLs[0].OriginalURL)
}

func TestInMemoryStoreStoreReturnsConflictForExistingOriginalURL(t *testing.T) {
	store := repository.NewInMemoryStore()

	require.NoError(t, store.Store(context.Background(), "abc123", "http://yandex.ru", "user-1"))

	err := store.Store(context.Background(), "def456", "http://yandex.ru", "user-2")
	require.Error(t, err)

	var conflictErr *repository.URLConflictError
	require.ErrorAs(t, err, &conflictErr)
	assert.Equal(t, "abc123", conflictErr.ShortID)

	userURLs, err := store.FindByUserID(context.Background(), "user-2")
	require.NoError(t, err)
	require.Len(t, userURLs, 1)
	assert.Equal(t, "abc123", userURLs[0].ShortID)
}

func TestInMemoryStoreMarkDeleted(t *testing.T) {
	store := repository.NewInMemoryStore()

	require.NoError(t, store.Store(context.Background(), "abc123", "http://yandex.ru", "user-1"))
	require.NoError(t, store.MarkDeleted(context.Background(), "user-1", []string{"abc123"}))

	_, err := store.Load(context.Background(), "abc123")
	require.Error(t, err)
	assert.ErrorIs(t, err, repository.ErrDeleted)
}

func TestInMemoryStoreGetStats(t *testing.T) {
	store := repository.NewInMemoryStore()
	ctx := context.Background()

	require.NoError(t, store.Store(ctx, "abc123", "http://yandex.ru", "user-1"))
	require.NoError(t, store.Store(ctx, "def456", "http://example.com", "user-1"))
	require.Error(t, store.Store(ctx, "ignored", "http://yandex.ru", "user-2"))
	require.NoError(t, store.MarkDeleted(ctx, "user-1", []string{"def456"}))

	stats, err := store.GetStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, stats.URLs)
	assert.Equal(t, 2, stats.Users)
}
