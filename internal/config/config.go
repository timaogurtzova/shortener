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

// Configuration объединяет все настройки приложения.
type Configuration struct {
	// Server хранит настройки HTTP-сервера.
	Server ServerConfiguration

	// Storage хранит настройки файлового хранилища.
	Storage StorageConfiguration

	// Database хранит настройки подключения к базе данных.
	Database DatabaseConfiguration

	// Audit хранит настройки приёмников аудита.
	Audit AuditConfiguration
}

const defaultFileStoragePath = "storage.json"

// ServerConfiguration описывает настройки HTTP-сервера.
type ServerConfiguration struct {
	// Address задаёт адрес прослушивания HTTP-сервера.
	Address string

	// BaseURL задаёт базовый URL для формирования коротких ссылок.
	BaseURL string

	// IdleTimeout задаёт максимальное время ожидания неактивного соединения.
	IdleTimeout time.Duration

	// ReadTimeout задаёт максимальное время чтения HTTP-запроса.
	ReadTimeout time.Duration

	// WriteTimeout задаёт максимальное время записи HTTP-ответа.
	WriteTimeout time.Duration
}

// StorageConfiguration описывает настройки файлового хранилища.
type StorageConfiguration struct {
	// FileStoragePath хранит путь к файлу хранилища, если он был явно задан.
	FileStoragePath *string
}

// DatabaseConfiguration описывает настройки подключения к базе данных.
type DatabaseConfiguration struct {
	// DSN хранит строку подключения к базе данных, если она была явно задана.
	DSN *string
}

// AuditConfiguration описывает настройки приёмников аудита.
type AuditConfiguration struct {
	// FilePath хранит путь к JSONL-файлу аудита, если файловый аудит включён.
	FilePath *string

	// URL хранит адрес удалённого HTTP-приёмника аудита, если удалённый аудит включён.
	URL *string
}

// IsConfigured сообщает, что путь к файловому хранилищу был явно задан через env или CLI.
func (c StorageConfiguration) IsConfigured() bool {
	return c.FileStoragePath != nil
}

// Path возвращает путь к файловому хранилищу или значение по умолчанию.
func (c StorageConfiguration) Path() string {
	if c.FileStoragePath == nil {
		return defaultFileStoragePath
	}

	return *c.FileStoragePath
}

// IsConfigured сообщает, что DSN базы данных был явно задан через env или CLI.
func (c DatabaseConfiguration) IsConfigured() bool {
	return c.DSN != nil
}

// FileEnabled сообщает, что аудит в файл был явно настроен.
func (c AuditConfiguration) FileEnabled() bool {
	return c.FilePath != nil
}

// RemoteEnabled сообщает, что аудит на удалённый сервер был явно настроен.
func (c AuditConfiguration) RemoteEnabled() bool {
	return c.URL != nil
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
			FileStoragePath: nil,
		},
		Database: DatabaseConfiguration{
			DSN: nil,
		},
		Audit: AuditConfiguration{
			FilePath: nil,
			URL:      nil,
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
	AuditFilePath   string
	AuditURL        string
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
	AuditFilePath   *string        `env:"AUDIT_FILE"`
	AuditURL        *string        `env:"AUDIT_URL"`
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
	fs.StringVar(&cfg.AuditFilePath, "audit-file", "", "audit file path")
	fs.StringVar(&cfg.AuditURL, "audit-url", "", "audit receiver url")

	if err := fs.Parse(args); err != nil {
		return cliConfig{}, err
	}

	return cfg, nil
}

// applyCLIConfig применяет значения из флагов поверх конфигурации по умолчанию.
func applyCLIConfig(cfg *Configuration, cliCfg cliConfig) {
	cfg.Server.Address = resolveAddress(cfg.Server.Address, cliCfg.Address)
	cfg.Server.BaseURL = resolveBaseURL(cfg.Server.BaseURL, cliCfg.BaseURL)
	cfg.Storage.FileStoragePath = resolveFileStoragePath(cliCfg.FileStoragePath)
	cfg.Database.DSN = resolveDatabaseDSN(cliCfg.DatabaseDSN)
	cfg.Audit.FilePath = resolveAuditFilePath(cliCfg.AuditFilePath)
	cfg.Audit.URL = resolveAuditURL(cliCfg.AuditURL)
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
			cfg.Storage.FileStoragePath = envCfg.FileStoragePath
			log.Info().Str("FileStoragePath", *envCfg.FileStoragePath).Msg("Overriding FileStoragePath from environment")
		} else {
			log.Warn().Msg("Empty FileStoragePath from environment, using previous value")
		}
	}

	if envCfg.DatabaseDSN != nil {
		if *envCfg.DatabaseDSN != "" {
			cfg.Database.DSN = envCfg.DatabaseDSN
			log.Info().Str("DatabaseDSN", *envCfg.DatabaseDSN).Msg("Overriding DatabaseDSN from environment")
		} else {
			log.Warn().Msg("Empty DatabaseDSN from environment, using previous value")
		}
	}

	if envCfg.AuditFilePath != nil {
		if *envCfg.AuditFilePath != "" {
			cfg.Audit.FilePath = envCfg.AuditFilePath
			log.Info().Str("AuditFilePath", *envCfg.AuditFilePath).Msg("Overriding AuditFilePath from environment")
		} else {
			log.Warn().Msg("Empty AuditFilePath from environment, using previous value")
		}
	}

	if envCfg.AuditURL != nil {
		if *envCfg.AuditURL != "" && isValidHTTPURL(*envCfg.AuditURL) {
			cfg.Audit.URL = envCfg.AuditURL
			log.Info().Str("AuditURL", redactURL(*envCfg.AuditURL)).Msg("Overriding AuditURL from environment")
		} else {
			log.Warn().Str("AuditURL", redactURLPointer(envCfg.AuditURL)).Msg("Invalid AuditURL from environment, using previous value")
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

// resolveFileStoragePath выбирает путь к файлу хранилища из флагов.
func resolveFileStoragePath(cliValue string) *string {
	if cliValue != "" {
		log.Info().Str("FileStoragePath", cliValue).Msg("Overriding FileStoragePath from CLI")
		value := cliValue
		return &value
	}

	log.Info().Str("FileStoragePath", defaultFileStoragePath).Msg("Using default FileStoragePath")
	return nil
}

// resolveDatabaseDSN выбирает DSN базы данных из флагов.
func resolveDatabaseDSN(cliValue string) *string {
	if cliValue != "" {
		log.Info().Str("DatabaseDSN", cliValue).Msg("Overriding DatabaseDSN from CLI")
		value := cliValue
		return &value
	}

	log.Info().Str("DatabaseDSN", "").Msg("Using default DatabaseDSN")
	return nil
}

// resolveAuditFilePath выбирает путь к файлу аудита из флагов.
func resolveAuditFilePath(cliValue string) *string {
	if cliValue != "" {
		log.Info().Str("AuditFilePath", cliValue).Msg("Overriding AuditFilePath from CLI")
		value := cliValue
		return &value
	}

	log.Info().Msg("Audit file receiver is disabled")
	return nil
}

// resolveAuditURL выбирает URL удалённого приёмника аудита из флагов.
func resolveAuditURL(cliValue string) *string {
	if cliValue != "" {
		if isValidHTTPURL(cliValue) {
			log.Info().Str("AuditURL", redactURL(cliValue)).Msg("Overriding AuditURL from CLI")
			value := cliValue
			return &value
		}
		log.Warn().Str("AuditURL", redactURL(cliValue)).Msg("Invalid CLI AuditURL, audit receiver is disabled")
	}

	log.Info().Msg("Remote audit receiver is disabled")
	return nil
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

func isValidHTTPURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func redactURLPointer(value *string) string {
	if value == nil {
		return ""
	}

	return redactURL(*value)
}

func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "<invalid-url>"
	}

	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""

	return u.String()
}
