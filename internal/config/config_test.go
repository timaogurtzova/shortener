package config_test

import (
	"os"
	"path/filepath"
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
		wantGRPCAddress     string
		wantBaseURL         string
		wantHTTPS           bool
		wantFileStoragePath string
		wantStorageConfig   bool
		wantDatabaseDSN     string
		wantDatabaseConfig  bool
	}{
		{
			имя:                 "использует значения по умолчанию, когда нет env и флагов",
			wantAddress:         "localhost:8080",
			wantGRPCAddress:     "localhost:3200",
			wantBaseURL:         "http://localhost:8080",
			wantFileStoragePath: "storage.json",
			wantStorageConfig:   false,
			wantDatabaseDSN:     "",
			wantDatabaseConfig:  false,
		},
		{
			имя:                 "использует флаги, когда env отсутствуют",
			args:                []string{"-a", "localhost:9090", "-g", "localhost:9091", "-b", "http://localhost:9090", "-s", "-f", "/tmp/shortener.json", "-d", "postgres://shortener:secret@localhost:5432/shortener?sslmode=disable"},
			wantAddress:         "localhost:9090",
			wantGRPCAddress:     "localhost:9091",
			wantBaseURL:         "http://localhost:9090",
			wantHTTPS:           true,
			wantFileStoragePath: "/tmp/shortener.json",
			wantStorageConfig:   true,
			wantDatabaseDSN:     "postgres://shortener:secret@localhost:5432/shortener?sslmode=disable",
			wantDatabaseConfig:  true,
		},
		{
			имя: "использует переменные окружения, когда флаги отсутствуют",
			env: map[string]string{
				"SERVER_ADDRESS":    "localhost:7070",
				"GRPC_ADDRESS":      "localhost:7071",
				"BASE_URL":          "http://localhost:7070",
				"ENABLE_HTTPS":      "true",
				"FILE_STORAGE_PATH": "/var/tmp/shortener.json",
				"DATABASE_DSN":      "postgres://env:secret@localhost:5432/envdb?sslmode=disable",
			},
			wantAddress:         "localhost:7070",
			wantGRPCAddress:     "localhost:7071",
			wantBaseURL:         "http://localhost:7070",
			wantHTTPS:           true,
			wantFileStoragePath: "/var/tmp/shortener.json",
			wantStorageConfig:   true,
			wantDatabaseDSN:     "postgres://env:secret@localhost:5432/envdb?sslmode=disable",
			wantDatabaseConfig:  true,
		},
		{
			имя:  "переменные окружения имеют приоритет над флагами",
			args: []string{"-a", "localhost:9090", "-g", "localhost:9091", "-b", "http://localhost:9090", "-s", "-f", "/tmp/shortener.json", "-d", "postgres://flag:secret@localhost:5432/flagdb?sslmode=disable"},
			env: map[string]string{
				"SERVER_ADDRESS":    "localhost:7070",
				"GRPC_ADDRESS":      "localhost:7071",
				"BASE_URL":          "http://localhost:7070",
				"ENABLE_HTTPS":      "false",
				"FILE_STORAGE_PATH": "/var/tmp/shortener.json",
				"DATABASE_DSN":      "postgres://env:secret@localhost:5432/envdb?sslmode=disable",
			},
			wantAddress:         "localhost:7070",
			wantGRPCAddress:     "localhost:7071",
			wantBaseURL:         "http://localhost:7070",
			wantFileStoragePath: "/var/tmp/shortener.json",
			wantStorageConfig:   true,
			wantDatabaseDSN:     "postgres://env:secret@localhost:5432/envdb?sslmode=disable",
			wantDatabaseConfig:  true,
		},
		{
			имя:                 "разбирает поддерживаемые cli-флаги в формате через равно",
			args:                []string{"-a=localhost:6060", "-g=localhost:6061", "-b=http://localhost:6060", "-s=true", "-f=/tmp/storage.json", "-d=postgres://shortener:secret@localhost:5432/shortener?sslmode=disable"},
			wantAddress:         "localhost:6060",
			wantGRPCAddress:     "localhost:6061",
			wantBaseURL:         "http://localhost:6060",
			wantHTTPS:           true,
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
			wantGRPCAddress:     "localhost:3200",
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
			assert.Equal(t, tt.wantGRPCAddress, cfg.GRPC.Address)
			assert.Equal(t, tt.wantBaseURL, cfg.Server.BaseURL)
			assert.Equal(t, tt.wantHTTPS, cfg.Server.EnableHTTPS)
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
		[]string{"-a", "localhost:9090", "-g", "localhost:9091", "-b", "http://localhost:9090"},
		map[string]string{
			"SERVER_ADDRESS": "://bad address",
			"GRPC_ADDRESS":   "://bad gRPC address",
			"BASE_URL":       "bad-url",
		},
	)
	require.NoError(t, err)

	assert.Equal(t, "localhost:9090", cfg.Server.Address)
	assert.Equal(t, "localhost:9091", cfg.GRPC.Address)
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

func TestLoadConfigUsesAllSettingsFromJSONFile(t *testing.T) {
	configPath := writeConfigFile(t, `{
		"server_address": "localhost:9443",
		"grpc_address": "localhost:9444",
		"base_url": "https://localhost:9443",
		"trusted_subnet": "192.168.10.0/24",
		"file_storage_path": "/tmp/from-config.json",
		"database_dsn": "postgres://config:secret@localhost:5432/configdb?sslmode=disable",
		"enable_https": true,
		"server_idle_timeout": "11s",
		"server_read_timeout": 12000000000,
		"server_write_timeout": "13s",
		"audit_file": "/tmp/config-audit.log",
		"audit_url": "https://audit.example/events"
	}`)

	cfg, err := loadWithState(t, []string{"-c", configPath}, nil)
	require.NoError(t, err)

	assert.Equal(t, "localhost:9443", cfg.Server.Address)
	assert.Equal(t, "localhost:9444", cfg.GRPC.Address)
	assert.Equal(t, "https://localhost:9443", cfg.Server.BaseURL)
	assert.Equal(t, "192.168.10.0/24", cfg.Server.TrustedSubnet)
	assert.True(t, cfg.Server.EnableHTTPS)
	assert.Equal(t, 11*time.Second, cfg.Server.IdleTimeout)
	assert.Equal(t, 12*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 13*time.Second, cfg.Server.WriteTimeout)
	require.NotNil(t, cfg.Storage.FileStoragePath)
	assert.Equal(t, "/tmp/from-config.json", *cfg.Storage.FileStoragePath)
	require.NotNil(t, cfg.Database.DSN)
	assert.Equal(t, "postgres://config:secret@localhost:5432/configdb?sslmode=disable", *cfg.Database.DSN)
	require.NotNil(t, cfg.Audit.FilePath)
	assert.Equal(t, "/tmp/config-audit.log", *cfg.Audit.FilePath)
	require.NotNil(t, cfg.Audit.URL)
	assert.Equal(t, "https://audit.example/events", *cfg.Audit.URL)
}

func TestLoadConfigUsesTrustedSubnetByPriority(t *testing.T) {
	configPath := writeConfigFile(t, `{"trusted_subnet":"10.0.0.0/8"}`)

	tests := []struct {
		name string
		args []string
		env  map[string]string
		want string
	}{
		{
			name: "disabled by default",
			want: "",
		},
		{
			name: "uses JSON value",
			args: []string{"-c", configPath},
			want: "10.0.0.0/8",
		},
		{
			name: "CLI flag overrides JSON",
			args: []string{"-c", configPath, "-t", "192.168.0.0/16"},
			want: "192.168.0.0/16",
		},
		{
			name: "environment overrides CLI",
			args: []string{"-c", configPath, "-t", "192.168.0.0/16"},
			env:  map[string]string{"TRUSTED_SUBNET": "172.16.0.0/12"},
			want: "172.16.0.0/12",
		},
		{
			name: "empty environment value disables configured subnet",
			args: []string{"-c", configPath, "-t", "192.168.0.0/16"},
			env:  map[string]string{"TRUSTED_SUBNET": ""},
			want: "",
		},
		{
			name: "explicit empty CLI value disables JSON subnet",
			args: []string{"-c", configPath, "-t", ""},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := loadWithState(t, tt.args, tt.env)
			require.NoError(t, err)
			assert.Equal(t, tt.want, cfg.Server.TrustedSubnet)
		})
	}
}

func TestLoadConfigSupportsLongConfigFlag(t *testing.T) {
	configPath := writeConfigFile(t, `{"server_address":"localhost:8181"}`)

	cfg, err := loadWithState(t, []string{"-config", configPath}, nil)
	require.NoError(t, err)

	assert.Equal(t, "localhost:8181", cfg.Server.Address)
}

func TestLoadConfigPriorityIncludesJSONFile(t *testing.T) {
	configPath := writeConfigFile(t, `{
		"server_address": "localhost:6060",
		"grpc_address": "localhost:6061",
		"base_url": "https://localhost:6060",
		"file_storage_path": "/tmp/file-config.json",
		"database_dsn": "postgres://file:secret@localhost:5432/filedb?sslmode=disable",
		"enable_https": true,
		"audit_file": "/tmp/file-audit.log",
		"audit_url": "https://file-audit.example/events"
	}`)

	cfg, err := loadWithState(t,
		[]string{
			"-c", configPath,
			"-a", "localhost:7070",
			"-g", "localhost:7071",
			"-b", "https://localhost:7070",
			"-f", "/tmp/cli-config.json",
			"-d", "postgres://cli:secret@localhost:5432/clidb?sslmode=disable",
			"-s=false",
			"--audit-file", "/tmp/cli-audit.log",
			"--audit-url", "https://cli-audit.example/events",
		},
		map[string]string{
			"SERVER_ADDRESS": "localhost:8081",
			"GRPC_ADDRESS":   "localhost:8082",
			"ENABLE_HTTPS":   "true",
		},
	)
	require.NoError(t, err)

	// env > CLI > JSON > defaults.
	assert.Equal(t, "localhost:8081", cfg.Server.Address)
	assert.Equal(t, "localhost:8082", cfg.GRPC.Address)
	assert.Equal(t, "https://localhost:7070", cfg.Server.BaseURL)
	assert.True(t, cfg.Server.EnableHTTPS)
	require.NotNil(t, cfg.Storage.FileStoragePath)
	assert.Equal(t, "/tmp/cli-config.json", *cfg.Storage.FileStoragePath)
	require.NotNil(t, cfg.Database.DSN)
	assert.Equal(t, "postgres://cli:secret@localhost:5432/clidb?sslmode=disable", *cfg.Database.DSN)
	require.NotNil(t, cfg.Audit.FilePath)
	assert.Equal(t, "/tmp/cli-audit.log", *cfg.Audit.FilePath)
	require.NotNil(t, cfg.Audit.URL)
	assert.Equal(t, "https://cli-audit.example/events", *cfg.Audit.URL)
}

func TestExplicitHTTPSFlagOverridesJSONFile(t *testing.T) {
	configPath := writeConfigFile(t, `{"enable_https":true}`)

	cfg, err := loadWithState(t, []string{"-c", configPath, "-s=false"}, nil)
	require.NoError(t, err)

	assert.False(t, cfg.Server.EnableHTTPS)
}

func TestLoadConfigUsesConfigPathFromEnvironment(t *testing.T) {
	flagConfigPath := writeConfigFile(t, `{"server_address":"localhost:6060"}`)
	envConfigPath := writeConfigFile(t, `{"server_address":"localhost:7070"}`)

	cfg, err := loadWithState(t,
		[]string{"-c", flagConfigPath},
		map[string]string{"CONFIG": envConfigPath},
	)
	require.NoError(t, err)

	assert.Equal(t, "localhost:7070", cfg.Server.Address)
}

func TestLoadConfigReturnsConfigFileErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "malformed JSON",
			content: `{"server_address":`,
		},
		{
			name:    "unknown option",
			content: `{"unknown_option":true}`,
		},
		{
			name:    "multiple JSON values",
			content: `{} {}`,
		},
		{
			name:    "invalid duration",
			content: `{"server_idle_timeout":"tomorrow"}`,
		},
		{
			name:    "unsupported duration type",
			content: `{"server_idle_timeout":true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := writeConfigFile(t, tt.content)

			_, err := loadWithState(t, []string{"-c", configPath}, nil)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "load config file")
		})
	}

	_, err := loadWithState(t, []string{"-c", filepath.Join(t.TempDir(), "missing.json")}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load config file")
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
		"GRPC_ADDRESS",
		"BASE_URL",
		"TRUSTED_SUBNET",
		"ENABLE_HTTPS",
		"SERVER_IDLE_TIMEOUT",
		"SERVER_READ_TIMEOUT",
		"SERVER_WRITE_TIMEOUT",
		"FILE_STORAGE_PATH",
		"DATABASE_DSN",
		"AUDIT_FILE",
		"AUDIT_URL",
		"CONFIG",
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

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}
