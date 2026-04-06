package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockDBPinger struct {
	pingContextFunc func(ctx context.Context) error
}

func (m *mockDBPinger) PingContext(ctx context.Context) error {
	if m.pingContextFunc != nil {
		return m.pingContextFunc(ctx)
	}

	return nil
}

func TestDBHealthCheckerPing(t *testing.T) {
	t.Run("база данных не настроена", func(t *testing.T) {
		checker := &DBHealthChecker{}

		err := checker.Ping(context.Background())
		require.Error(t, err)
		assert.Equal(t, "database is not configured", err.Error())
	})

	t.Run("ошибка при проверке соединения", func(t *testing.T) {
		checker := &DBHealthChecker{
			db: &mockDBPinger{
				pingContextFunc: func(ctx context.Context) error {
					return errors.New("ping failed")
				},
			},
		}

		err := checker.Ping(context.Background())
		require.Error(t, err)
		assert.Equal(t, "ping failed", err.Error())
	})

	t.Run("успешная проверка соединения", func(t *testing.T) {
		checker := &DBHealthChecker{
			db: &mockDBPinger{
				pingContextFunc: func(ctx context.Context) error {
					return nil
				},
			},
		}

		require.NoError(t, checker.Ping(context.Background()))
	})
}
