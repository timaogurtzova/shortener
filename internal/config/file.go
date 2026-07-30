package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog/log"
)

// fileConfig хранит настройки, которые могут быть заданы в JSON-файле.
type fileConfig struct {
	Address         *string       `json:"server_address"`
	GRPCAddress     *string       `json:"grpc_address"`
	BaseURL         *string       `json:"base_url"`
	TrustedSubnet   *string       `json:"trusted_subnet"`
	EnableHTTPS     *bool         `json:"enable_https"`
	IdleTimeout     *jsonDuration `json:"server_idle_timeout"`
	ReadTimeout     *jsonDuration `json:"server_read_timeout"`
	WriteTimeout    *jsonDuration `json:"server_write_timeout"`
	FileStoragePath *string       `json:"file_storage_path"`
	DatabaseDSN     *string       `json:"database_dsn"`
	AuditFilePath   *string       `json:"audit_file"`
	AuditURL        *string       `json:"audit_url"`
}

// jsonDuration позволяет задавать duration строкой ("15s") или числом наносекунд.
type jsonDuration time.Duration

func (d *jsonDuration) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return errors.New("duration value is empty")
	}

	switch {
	case data[0] == '"':
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return fmt.Errorf("decode duration string: %w", err)
		}
		value, err := time.ParseDuration(text)
		if err != nil {
			return fmt.Errorf("parse duration %q: %w", text, err)
		}
		*d = jsonDuration(value)
		return nil
	case data[0] == '-' || (data[0] >= '0' && data[0] <= '9'):
		var nanoseconds int64
		if err := json.Unmarshal(data, &nanoseconds); err != nil {
			return fmt.Errorf("decode duration nanoseconds: %w", err)
		}
		*d = jsonDuration(time.Duration(nanoseconds))
		return nil
	default:
		return fmt.Errorf("duration must be a string or an integer number of nanoseconds, got %q", data)
	}
}

func resolveConfigFilePath(cliValue string, envValue *string) string {
	if envValue != nil && *envValue != "" {
		return *envValue
	}
	return cliValue
}

func loadFileConfig(path string) (fileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return fileConfig{}, err
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var cfg fileConfig
	if err := decoder.Decode(&cfg); err != nil {
		return fileConfig{}, fmt.Errorf("decode JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return fileConfig{}, errors.New("decode JSON: multiple values are not allowed")
		}
		return fileConfig{}, fmt.Errorf("decode trailing JSON data: %w", err)
	}

	return cfg, nil
}

// applyFileConfig применяет значения JSON-файла поверх значений по умолчанию.
func applyFileConfig(cfg *Configuration, fileCfg fileConfig) {
	if fileCfg.Address != nil {
		if isValidAddress(*fileCfg.Address) {
			cfg.Server.Address = *fileCfg.Address
		} else {
			log.Warn().Str("Address", *fileCfg.Address).Msg("Invalid Address from config file, using previous value")
		}
	}

	if fileCfg.GRPCAddress != nil {
		if isValidAddress(*fileCfg.GRPCAddress) {
			cfg.GRPC.Address = *fileCfg.GRPCAddress
		} else {
			log.Warn().Str("GRPCAddress", *fileCfg.GRPCAddress).Msg("Invalid GRPCAddress from config file, using previous value")
		}
	}

	if fileCfg.BaseURL != nil {
		if isValidURL(*fileCfg.BaseURL) {
			cfg.Server.BaseURL = *fileCfg.BaseURL
		} else {
			log.Warn().Str("BaseURL", *fileCfg.BaseURL).Msg("Invalid BaseURL from config file, using previous value")
		}
	}

	if fileCfg.TrustedSubnet != nil {
		cfg.Server.TrustedSubnet = *fileCfg.TrustedSubnet
	}

	if fileCfg.EnableHTTPS != nil {
		cfg.Server.EnableHTTPS = *fileCfg.EnableHTTPS
	}
	if fileCfg.IdleTimeout != nil {
		cfg.Server.IdleTimeout = time.Duration(*fileCfg.IdleTimeout)
	}
	if fileCfg.ReadTimeout != nil {
		cfg.Server.ReadTimeout = time.Duration(*fileCfg.ReadTimeout)
	}
	if fileCfg.WriteTimeout != nil {
		cfg.Server.WriteTimeout = time.Duration(*fileCfg.WriteTimeout)
	}
	if fileCfg.FileStoragePath != nil {
		cfg.Storage.FileStoragePath = nonEmptyString(fileCfg.FileStoragePath)
	}
	if fileCfg.DatabaseDSN != nil {
		cfg.Database.DSN = nonEmptyString(fileCfg.DatabaseDSN)
	}
	if fileCfg.AuditFilePath != nil {
		cfg.Audit.FilePath = nonEmptyString(fileCfg.AuditFilePath)
	}
	if fileCfg.AuditURL != nil {
		if *fileCfg.AuditURL == "" {
			cfg.Audit.URL = nil
		} else if isValidHTTPURL(*fileCfg.AuditURL) {
			cfg.Audit.URL = nonEmptyString(fileCfg.AuditURL)
		} else {
			log.Warn().Str("AuditURL", redactURL(*fileCfg.AuditURL)).Msg("Invalid AuditURL from config file, using previous value")
		}
	}
}

func nonEmptyString(value *string) *string {
	if value == nil || *value == "" {
		return nil
	}
	result := *value
	return &result
}
