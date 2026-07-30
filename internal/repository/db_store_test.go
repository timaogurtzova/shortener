package repository_test

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/timaogurtzova/shortener/internal/repository"
)

const expectedStatsQuery = `
	SELECT
		(SELECT COUNT(*) FROM short_urls),
		(SELECT COUNT(DISTINCT user_id) FROM user_urls)
`

func TestNewDBStore(t *testing.T) {
	store, err := repository.NewDBStore(nil)
	require.Error(t, err)
	assert.Nil(t, store)
	assert.Equal(t, "database is not configured", err.Error())
}

func TestDBStoreGetStats(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})

	store, err := repository.NewDBStore(db)
	require.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(expectedStatsQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"urls", "users"}).AddRow(7, 3))

	stats, err := store.GetStats(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 7, stats.URLs)
	assert.Equal(t, 3, stats.Users)
	mock.ExpectClose()
	require.NoError(t, db.Close())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDBStoreGetStatsReturnsQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})

	store, err := repository.NewDBStore(db)
	require.NoError(t, err)

	wantErr := errors.New("query stats")
	mock.ExpectQuery(regexp.QuoteMeta(expectedStatsQuery)).WillReturnError(wantErr)

	_, err = store.GetStats(context.Background())

	require.ErrorIs(t, err, wantErr)
	mock.ExpectClose()
	require.NoError(t, db.Close())
	require.NoError(t, mock.ExpectationsWereMet())
}
