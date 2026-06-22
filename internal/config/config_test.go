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
		имя                 string
		args                []string
		env                 map[string]string
		wantAddress         string
		wantBaseURL         string
		wantFileStoragePath string
		wantStorageConfig   bool
		wantDatabaseDSN     string
		wantDatabaseConfig  bool
	}{
		{
			имя:                 "использует значения по умолчанию, когда нет env и флагов",
			wantAddress:         "localhost:8080",
			wantBaseURL:         "http://localhost:8080",
			wantFileStoragePath: "storage.json",
			wantStorageConfig:   false,
			wantDatabaseDSN:     "",
			wantDatabaseConfig:  false,
		},
		{
			имя:                 "использует флаги, когда env отсутствуют",
			args:                []string{"-a", "localhost:9090", "-b", "http://localhost:9090", "-f", "/tmp/shortener.json", "-d", "postgres://shortener:secret@localhost:5432/shortener?sslmode=disable"},
			wantAddress:         "localhost:9090",
			wantBaseURL:         "http://localhost:9090",
			wantFileStoragePath: "/tmp/shortener.json",
			wantStorageConfig:   true,
			wantDatabaseDSN:     "postgres://shortener:secret@localhost:5432/shortener?sslmode=disable",
			wantDatabaseConfig:  true,
		},
		{
			имя: "использует переменные окружения, когда флаги отсутствуют",
			env: map[string]string{
				"SERVER_ADDRESS":    "localhost:7070",
				"BASE_URL":          "http://localhost:7070",
				"FILE_STORAGE_PATH": "/var/tmp/shortener.json",
				"DATABASE_DSN":      "postgres://env:secret@localhost:5432/envdb?sslmode=disable",
			},
			wantAddress:         "localhost:7070",
			wantBaseURL:         "http://localhost:7070",
			wantFileStoragePath: "/var/tmp/shortener.json",
			wantStorageConfig:   true,
			wantDatabaseDSN:     "postgres://env:secret@localhost:5432/envdb?sslmode=disable",
			wantDatabaseConfig:  true,
		},
		{
			имя:  "переменные окружения имеют приоритет над флагами",
			args: []string{"-a", "localhost:9090", "-b", "http://localhost:9090", "-f", "/tmp/shortener.json", "-d", "postgres://flag:secret@localhost:5432/flagdb?sslmode=disable"},
			env: map[string]string{
				"SERVER_ADDRESS":    "localhost:7070",
				"BASE_URL":          "http://localhost:7070",
				"FILE_STORAGE_PATH": "/var/tmp/shortener.json",
				"DATABASE_DSN":      "postgres://env:secret@localhost:5432/envdb?sslmode=disable",
			},
			wantAddress:         "localhost:7070",
			wantBaseURL:         "http://localhost:7070",
			wantFileStoragePath: "/var/tmp/shortener.json",
			wantStorageConfig:   true,
			wantDatabaseDSN:     "postgres://env:secret@localhost:5432/envdb?sslmode=disable",
			wantDatabaseConfig:  true,
		},
		{
			имя:                 "разбирает поддерживаемые cli-флаги в формате через равно",
			args:                []string{"-a=localhost:6060", "-b=http://localhost:6060", "-f=/tmp/storage.json", "-d=postgres://shortener:secret@localhost:5432/shortener?sslmode=disable"},
			wantAddress:         "localhost:6060",
			wantBaseURL:         "http://localhost:6060",
			wantFileStoragePath: "/tmp/storage.json",
			wantStorageConfig:   true,
			wantDatabaseDSN:     "postgres://shortener:secret@localhost:5432/shortener?sslmode=disable",
			wantDatabaseConfig:  true,
		},
		{
			имя:  "пустые значения из окружения не отключают настроенные флаги",
			args: []string{"-f", "/tmp/storage.json", "-d", "postgres://shortener:secret@localhost:5432/shortener?sslmode=disable"},
			env: map[string]string{
				"FILE_STORAGE_PATH": "",
				"DATABASE_DSN":      "",
			},
			wantAddress:         "localhost:8080",
			wantBaseURL:         "http://localhost:8080",
			wantFileStoragePath: "/tmp/storage.json",
			wantStorageConfig:   true,
			wantDatabaseDSN:     "postgres://shortener:secret@localhost:5432/shortener?sslmode=disable",
			wantDatabaseConfig:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.имя, func(t *testing.T) {
			cfg, err := loadWithState(t, tt.args, tt.env)
			require.NoError(t, err)

			assert.Equal(t, tt.wantAddress, cfg.Server.Address)
			assert.Equal(t, tt.wantBaseURL, cfg.Server.BaseURL)
			assert.Equal(t, tt.wantFileStoragePath, cfg.Storage.Path())
			assert.Equal(t, tt.wantStorageConfig, cfg.Storage.IsConfigured())
			if tt.wantDatabaseDSN == "" {
				assert.Nil(t, cfg.Database.DSN)
			} else {
				require.NotNil(t, cfg.Database.DSN)
				assert.Equal(t, tt.wantDatabaseDSN, *cfg.Database.DSN)
			}
			assert.Equal(t, tt.wantDatabaseConfig, cfg.Database.IsConfigured())
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

func TestLoadConfigUsesAuditSettings(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		env           map[string]string
		wantFilePath  string
		wantFileAudit bool
		wantURL       string
		wantURLAudit  bool
	}{
		{
			name: "audit receivers are disabled by default",
		},
		{
			name:          "uses audit flags",
			args:          []string{"--audit-file", "/tmp/audit.log", "--audit-url", "http://localhost:9090/audit"},
			wantFilePath:  "/tmp/audit.log",
			wantFileAudit: true,
			wantURL:       "http://localhost:9090/audit",
			wantURLAudit:  true,
		},
		{
			name: "audit env overrides flags",
			args: []string{"--audit-file", "/tmp/flag.log", "--audit-url", "http://localhost:9090/flag"},
			env: map[string]string{
				"AUDIT_FILE": "/tmp/env.log",
				"AUDIT_URL":  "http://localhost:9090/env",
			},
			wantFilePath:  "/tmp/env.log",
			wantFileAudit: true,
			wantURL:       "http://localhost:9090/env",
			wantURLAudit:  true,
		},
		{
			name: "empty audit env values keep flags",
			args: []string{"--audit-file", "/tmp/flag.log", "--audit-url", "http://localhost:9090/flag"},
			env: map[string]string{
				"AUDIT_FILE": "",
				"AUDIT_URL":  "",
			},
			wantFilePath:  "/tmp/flag.log",
			wantFileAudit: true,
			wantURL:       "http://localhost:9090/flag",
			wantURLAudit:  true,
		},
		{
			name:          "invalid audit url flag disables remote audit",
			args:          []string{"--audit-url", "ftp://localhost:9090/audit"},
			wantURLAudit:  false,
			wantFileAudit: false,
		},
		{
			name: "invalid audit url env keeps flag value",
			args: []string{"--audit-url", "http://localhost:9090/flag"},
			env: map[string]string{
				"AUDIT_URL": "ftp://localhost:9090/env",
			},
			wantURL:      "http://localhost:9090/flag",
			wantURLAudit: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := loadWithState(t, tt.args, tt.env)
			require.NoError(t, err)

			assert.Equal(t, tt.wantFileAudit, cfg.Audit.FileEnabled())
			if tt.wantFileAudit {
				require.NotNil(t, cfg.Audit.FilePath)
				assert.Equal(t, tt.wantFilePath, *cfg.Audit.FilePath)
			} else {
				assert.Nil(t, cfg.Audit.FilePath)
			}

			assert.Equal(t, tt.wantURLAudit, cfg.Audit.RemoteEnabled())
			if tt.wantURLAudit {
				require.NotNil(t, cfg.Audit.URL)
				assert.Equal(t, tt.wantURL, *cfg.Audit.URL)
			} else {
				assert.Nil(t, cfg.Audit.URL)
			}
		})
	}
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
		"FILE_STORAGE_PATH",
		"DATABASE_DSN",
		"AUDIT_FILE",
		"AUDIT_URL",
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
