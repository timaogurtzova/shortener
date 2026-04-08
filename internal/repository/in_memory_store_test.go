package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryStoreBatchStore(t *testing.T) {
	store := NewInMemoryStore()

	err := store.BatchStore([]BatchRecord{
		{ID: "abc123", OriginalURL: "http://yandex.ru"},
		{ID: "edVPg3ks", OriginalURL: "http://ya.ru"},
	})
	require.NoError(t, err)

	url, err := store.Load("abc123")
	require.NoError(t, err)
	assert.Equal(t, "http://yandex.ru", url)

	url, err = store.Load("edVPg3ks")
	require.NoError(t, err)
	assert.Equal(t, "http://ya.ru", url)
}
