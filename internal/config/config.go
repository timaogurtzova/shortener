package config

import (
	"flag"
	"io"
	"net"
	"net/url"
	"os"
	"time"

	env "github.com/caarlos0/env/v11"
	"github.com/rs/zerolog/log"
)

type Configuration struct {
	Server   ServerConfiguration
	Storage  StorageConfiguration
	Database DatabaseConfiguration
}

type ServerConfiguration struct {
	Address      string
	BaseURL      string
	IdleTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type StorageConfiguration struct {
	FileStoragePath string
}

type DatabaseConfiguration struct {
	DSN string
}

// defaultConfig возвращает конфигурацию со значениями по умолчанию.
func defaultConfig() *Configuration {
	return &Configuration{
		Server: ServerConfiguration{
			Address:      "localhost:8080",
			BaseURL:      "http://localhost:8080",
			IdleTimeout:  60 * time.Second,
			ReadTimeout:  60 * time.Second,
			WriteTimeout: 60 * time.Second,
		},
		Storage: StorageConfiguration{
			FileStoragePath: "storage.json",
		},
		Database: DatabaseConfiguration{
			DSN: "",
		},
	}
}

// LoadConfig загружает конфигурацию из аргументов текущего процесса и переменных окружения.
func LoadConfig() (*Configuration, error) {
	return loadConfig(os.Args[1:])
}

// loadConfig собирает конфигурацию с приоритетом окружение > флаги > значения по умолчанию.
func loadConfig(args []string) (*Configuration, error) {
	cfg := defaultConfig()

	cliCfg, err := parseCLIArgs(args)
	if err != nil {
		return nil, err
	}

	applyCLIConfig(cfg, cliCfg)

	envCfg, err := env.ParseAsWithOptions[envConfig](env.Options{})
	if err != nil {
		return nil, err
	}

	applyEnvConfig(cfg, envCfg)

	return cfg, nil
}

type cliConfig struct {
	Address         string
	BaseURL         string
	FileStoragePath string
	DatabaseDSN     string
}

// envConfig хранит только значения, явно заданные в переменных окружения.
type envConfig struct {
	Address         *string        `env:"SERVER_ADDRESS"`
	BaseURL         *string        `env:"BASE_URL"`
	IdleTimeout     *time.Duration `env:"SERVER_IDLE_TIMEOUT"`
	ReadTimeout     *time.Duration `env:"SERVER_READ_TIMEOUT"`
	WriteTimeout    *time.Duration `env:"SERVER_WRITE_TIMEOUT"`
	FileStoragePath *string        `env:"FILE_STORAGE_PATH"`
	DatabaseDSN     *string        `env:"DATABASE_DSN"`
}

// parseCLIArgs разбирает флаги конфигурации из аргументов командной строки.
func parseCLIArgs(args []string) (cliConfig, error) {
	var cfg cliConfig

	fs := flag.NewFlagSet("shortener", flag.ContinueOnError)

	// Подавляем вывод стандартного парсера, чтобы управлять ошибками самостоятельно.
	fs.SetOutput(io.Discard)

	fs.StringVar(&cfg.Address, "a", "", "server address")
	fs.StringVar(&cfg.BaseURL, "b", "", "base url")
	fs.StringVar(&cfg.FileStoragePath, "f", "", "file storage path")
	fs.StringVar(&cfg.DatabaseDSN, "d", "", "database dsn")

	if err := fs.Parse(args); err != nil {
		return cliConfig{}, err
	}

	return cfg, nil
}

// applyCLIConfig применяет значения из флагов поверх конфигурации по умолчанию.
func applyCLIConfig(cfg *Configuration, cliCfg cliConfig) {
	cfg.Server.Address = resolveAddress(cfg.Server.Address, cliCfg.Address)
	cfg.Server.BaseURL = resolveBaseURL(cfg.Server.BaseURL, cliCfg.BaseURL)
	cfg.Storage.FileStoragePath = resolveFileStoragePath(cfg.Storage.FileStoragePath, cliCfg.FileStoragePath)
	cfg.Database.DSN = resolveDatabaseDSN(cfg.Database.DSN, cliCfg.DatabaseDSN)
}

// applyEnvConfig применяет значения из переменных окружения поверх уже собранной конфигурации.
func applyEnvConfig(cfg *Configuration, envCfg envConfig) {
	if envCfg.Address != nil {
		if isValidAddress(*envCfg.Address) {
			cfg.Server.Address = *envCfg.Address
			log.Info().Str("Address", *envCfg.Address).Msg("Overriding Address from environment")
		} else {
			log.Warn().Str("Address", *envCfg.Address).Msg("Invalid Address from environment, using previous value")
		}
	}

	if envCfg.BaseURL != nil {
		if isValidURL(*envCfg.BaseURL) {
			cfg.Server.BaseURL = *envCfg.BaseURL
			log.Info().Str("BaseURL", *envCfg.BaseURL).Msg("Overriding BaseURL from environment")
		} else {
			log.Warn().Str("BaseURL", *envCfg.BaseURL).Msg("Invalid BaseURL from environment, using previous value")
		}
	}

	if envCfg.IdleTimeout != nil {
		cfg.Server.IdleTimeout = *envCfg.IdleTimeout
		log.Info().Dur("IdleTimeout", *envCfg.IdleTimeout).Msg("Overriding IdleTimeout from environment")
	}

	if envCfg.ReadTimeout != nil {
		cfg.Server.ReadTimeout = *envCfg.ReadTimeout
		log.Info().Dur("ReadTimeout", *envCfg.ReadTimeout).Msg("Overriding ReadTimeout from environment")
	}

	if envCfg.WriteTimeout != nil {
		cfg.Server.WriteTimeout = *envCfg.WriteTimeout
		log.Info().Dur("WriteTimeout", *envCfg.WriteTimeout).Msg("Overriding WriteTimeout from environment")
	}

	if envCfg.FileStoragePath != nil {
		if *envCfg.FileStoragePath != "" {
			cfg.Storage.FileStoragePath = *envCfg.FileStoragePath
			log.Info().Str("FileStoragePath", *envCfg.FileStoragePath).Msg("Overriding FileStoragePath from environment")
		} else {
			log.Warn().Msg("Empty FileStoragePath from environment, using previous value")
		}
	}

	if envCfg.DatabaseDSN != nil {
		if *envCfg.DatabaseDSN != "" {
			cfg.Database.DSN = *envCfg.DatabaseDSN
			log.Info().Str("DatabaseDSN", *envCfg.DatabaseDSN).Msg("Overriding DatabaseDSN from environment")
		} else {
			log.Warn().Msg("Empty DatabaseDSN from environment, using previous value")
		}
	}
}

// resolveAddress выбирает адрес сервера из флагов или оставляет значение по умолчанию.
func resolveAddress(defaultValue, cliValue string) string {
	if cliValue != "" {
		if isValidAddress(cliValue) {
			log.Info().Str("Address", cliValue).Msg("Overriding Address from CLI")
			return cliValue
		}
		log.Warn().Str("Address", cliValue).Msg("Invalid CLI Address, using default value")
	}

	log.Info().Str("Address", defaultValue).Msg("Using default Address")
	return defaultValue
}

// resolveBaseURL выбирает базовый URL из флагов или оставляет значение по умолчанию.
func resolveBaseURL(defaultValue, cliValue string) string {
	if cliValue != "" {
		if isValidURL(cliValue) {
			log.Info().Str("BaseURL", cliValue).Msg("Overriding BaseURL from CLI")
			return cliValue
		}
		log.Warn().Str("BaseURL", cliValue).Msg("Invalid CLI BaseURL, using default value")
	}

	log.Info().Str("BaseURL", defaultValue).Msg("Using default BaseURL")
	return defaultValue
}

// resolveFileStoragePath выбирает путь к файлу хранилища из флагов или оставляет значение по умолчанию.
func resolveFileStoragePath(defaultValue, cliValue string) string {
	if cliValue != "" {
		log.Info().Str("FileStoragePath", cliValue).Msg("Overriding FileStoragePath from CLI")
		return cliValue
	}

	log.Info().Str("FileStoragePath", defaultValue).Msg("Using default FileStoragePath")
	return defaultValue
}

// resolveDatabaseDSN выбирает DSN базы данных из флагов или оставляет значение по умолчанию.
func resolveDatabaseDSN(defaultValue, cliValue string) string {
	if cliValue != "" {
		log.Info().Str("DatabaseDSN", cliValue).Msg("Overriding DatabaseDSN from CLI")
		return cliValue
	}

	log.Info().Str("DatabaseDSN", defaultValue).Msg("Using default DatabaseDSN")
	return defaultValue
}

// isValidAddress проверяет, что адрес имеет формат host:port.
func isValidAddress(addr string) bool {
	_, err := net.ResolveTCPAddr("tcp", addr)
	return err == nil
}

// isValidURL проверяет корректность URL.
func isValidURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme != "" && u.Host != ""
}
