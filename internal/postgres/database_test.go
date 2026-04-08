package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDatabase(t *testing.T) {
	database, err := NewDatabase(nil)
	require.Error(t, err)
	assert.Nil(t, database)
	assert.ErrorIs(t, err, ErrDatabaseNotConfigured)
}

func TestDatabasePing(t *testing.T) {
	t.Run("база данных не настроена", func(t *testing.T) {
		var database *Database

		err := database.Ping(context.Background())
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrDatabaseNotConfigured)
	})

	t.Run("закрытие nil-подключения не падает", func(t *testing.T) {
		var database *Database
		require.NoError(t, database.Close())
		assert.Nil(t, database.SQLDB())
	})
}
