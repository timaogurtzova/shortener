package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/timaogurtzova/shortener/internal/config"
)

func TestLoadConfigPriority(t *testing.T) {
	tests := []struct {
		имя         string
		args        []string
		env         map[string]string
		wantAddress string
		wantBaseURL string
	}{
		{
			имя:         "использует значения по умолчанию, когда нет env и флагов",
			wantAddress: "localhost:8080",
			wantBaseURL: "http://localhost:8080",
		},
		{
			имя:         "использует флаги, когда env отсутствуют",
			args:        []string{"-a", "localhost:9090", "-b", "http://localhost:9090"},
			wantAddress: "localhost:9090",
			wantBaseURL: "http://localhost:9090",
		},
		{
			имя: "использует переменные окружения, когда флаги отсутствуют",
			env: map[string]string{
				"SERVER_ADDRESS": "localhost:7070",
				"BASE_URL":       "http://localhost:7070",
			},
			wantAddress: "localhost:7070",
			wantBaseURL: "http://localhost:7070",
		},
		{
			имя:  "переменные окружения имеют приоритет над флагами",
			args: []string{"-a", "localhost:9090", "-b", "http://localhost:9090"},
			env: map[string]string{
				"SERVER_ADDRESS": "localhost:7070",
				"BASE_URL":       "http://localhost:7070",
			},
			wantAddress: "localhost:7070",
			wantBaseURL: "http://localhost:7070",
		},
		{
			имя:         "игнорирует посторонние флаги при разборе поддерживаемых cli-флагов",
			args:        []string{"-test.v=true", "-a=localhost:6060", "-b=http://localhost:6060"},
			wantAddress: "localhost:6060",
			wantBaseURL: "http://localhost:6060",
		},
	}

	for _, tt := range tests {
		t.Run(tt.имя, func(t *testing.T) {
			cfg, err := loadWithState(t, tt.args, tt.env)
			require.NoError(t, err)

			assert.Equal(t, tt.wantAddress, cfg.Server.Address)
			assert.Equal(t, tt.wantBaseURL, cfg.Server.BaseURL)
		})
	}
}

func TestLoadConfigFallsBackWhenEnvironmentValuesAreInvalid(t *testing.T) {
	cfg, err := loadWithState(t,
		[]string{"-a", "localhost:9090", "-b", "http://localhost:9090"},
		map[string]string{
			"SERVER_ADDRESS": "://bad address",
			"BASE_URL":       "bad-url",
		},
	)
	require.NoError(t, err)

	assert.Equal(t, "localhost:9090", cfg.Server.Address)
	assert.Equal(t, "http://localhost:9090", cfg.Server.BaseURL)
}

func TestLoadConfigUsesTimeoutsFromEnvironment(t *testing.T) {
	cfg, err := loadWithState(t, nil, map[string]string{
		"SERVER_IDLE_TIMEOUT":  "15s",
		"SERVER_READ_TIMEOUT":  "20s",
		"SERVER_WRITE_TIMEOUT": "25s",
	})
	require.NoError(t, err)

	assert.Equal(t, 15*time.Second, cfg.Server.IdleTimeout)
	assert.Equal(t, 20*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 25*time.Second, cfg.Server.WriteTimeout)
}

func loadWithState(t *testing.T, args []string, envVars map[string]string) (*config.Configuration, error) {
	t.Helper()

	oldArgs := os.Args
	os.Args = append([]string{"shortener"}, args...)
	t.Cleanup(func() {
		os.Args = oldArgs
	})

	for _, key := range []string{
		"SERVER_ADDRESS",
		"BASE_URL",
		"SERVER_IDLE_TIMEOUT",
		"SERVER_READ_TIMEOUT",
		"SERVER_WRITE_TIMEOUT",
	} {
		previousValue, wasSet := os.LookupEnv(key)

		if value, ok := envVars[key]; ok {
			require.NoError(t, os.Setenv(key, value))
		} else {
			require.NoError(t, os.Unsetenv(key))
		}

		t.Cleanup(func() {
			if wasSet {
				_ = os.Setenv(key, previousValue)
				return
			}
			_ = os.Unsetenv(key)
		})
	}

	return config.LoadConfig()
}
