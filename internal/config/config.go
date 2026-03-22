package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	env "github.com/caarlos0/env/v11"
	"github.com/rs/zerolog/log"
)

type Configuration struct {
	Server  ServerConfiguration
	Storage StorageConfiguration
}

type ServerConfiguration struct {
	Address      string        `env:"SERVER_ADDRESS"`
	BaseURL      string        `env:"BASE_URL"`
	IdleTimeout  time.Duration `env:"SERVER_IDLE_TIMEOUT"`
	ReadTimeout  time.Duration `env:"SERVER_READ_TIMEOUT"`
	WriteTimeout time.Duration `env:"SERVER_WRITE_TIMEOUT"`
}

type StorageConfiguration struct {
	FileStoragePath string `env:"FILE_STORAGE_PATH"`
}

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
	}
}

func LoadConfig() (*Configuration, error) {
	return loadConfig(os.Args[1:], env.Options{})
}

func loadConfig(args []string, envOptions env.Options) (*Configuration, error) {
	cfg := defaultConfig()

	cliCfg, err := parseCLIArgs(args)
	if err != nil {
		return nil, err
	}

	cfg.Server.Address = resolveAddress(cfg.Server.Address, cliCfg.Address)
	cfg.Server.BaseURL = resolveBaseURL(cfg.Server.BaseURL, cliCfg.BaseURL)
	cfg.Storage.FileStoragePath = resolveFileStoragePath(cfg.Storage.FileStoragePath, cliCfg.FileStoragePath)

	addressBeforeEnv := cfg.Server.Address
	baseURLBeforeEnv := cfg.Server.BaseURL
	fileStoragePathBeforeEnv := cfg.Storage.FileStoragePath

	var addressFromEnv bool
	var baseURLFromEnv bool
	var fileStoragePathFromEnv bool

	userOnSet := envOptions.OnSet
	envOptions.OnSet = func(tag string, value interface{}, isDefault bool) {
		switch tag {
		case "SERVER_ADDRESS":
			addressFromEnv = true
		case "BASE_URL":
			baseURLFromEnv = true
		case "FILE_STORAGE_PATH":
			fileStoragePathFromEnv = true
		}

		if userOnSet != nil {
			userOnSet(tag, value, isDefault)
		}
	}

	if err := env.ParseWithOptions(&cfg.Server, envOptions); err != nil {
		return nil, err
	}
	if err := env.ParseWithOptions(&cfg.Storage, envOptions); err != nil {
		return nil, err
	}

	if addressFromEnv {
		if isValidAddress(cfg.Server.Address) {
			log.Info().Str("Address", cfg.Server.Address).Msg("Overriding Address from environment")
		} else {
			log.Warn().Str("Address", cfg.Server.Address).Msg("Invalid Address from environment, falling back to CLI or default value")
			cfg.Server.Address = addressBeforeEnv
		}
	}

	if baseURLFromEnv {
		if isValidURL(cfg.Server.BaseURL) {
			log.Info().Str("BaseURL", cfg.Server.BaseURL).Msg("Overriding BaseURL from environment")
		} else {
			log.Warn().Str("BaseURL", cfg.Server.BaseURL).Msg("Invalid BaseURL from environment, falling back to CLI or default value")
			cfg.Server.BaseURL = baseURLBeforeEnv
		}
	}
	if fileStoragePathFromEnv {
		if cfg.Storage.FileStoragePath != "" {
			log.Info().Str("FileStoragePath", cfg.Storage.FileStoragePath).Msg("Overriding FileStoragePath from environment")
		} else {
			log.Warn().Str("FileStoragePath", cfg.Storage.FileStoragePath).Msg("Empty FileStoragePath from environment, falling back to CLI or default value")
			cfg.Storage.FileStoragePath = fileStoragePathBeforeEnv
		}
	}

	return cfg, nil
}

type cliConfig struct {
	Address         string
	BaseURL         string
	FileStoragePath string
}

func parseCLIArgs(args []string) (cliConfig, error) {
	var cfg cliConfig

	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch {
		case arg == "--":
			return cfg, nil
		case arg == "-a":
			if i+1 >= len(args) {
				return cliConfig{}, fmt.Errorf("flag needs an argument: -a")
			}
			cfg.Address = args[i+1]
			i++
		case strings.HasPrefix(arg, "-a="):
			cfg.Address = strings.TrimPrefix(arg, "-a=")
		case arg == "-b":
			if i+1 >= len(args) {
				return cliConfig{}, fmt.Errorf("flag needs an argument: -b")
			}
			cfg.BaseURL = args[i+1]
			i++
		case strings.HasPrefix(arg, "-b="):
			cfg.BaseURL = strings.TrimPrefix(arg, "-b=")
		case arg == "-f":
			if i+1 >= len(args) {
				return cliConfig{}, fmt.Errorf("flag needs an argument: -f")
			}
			cfg.FileStoragePath = args[i+1]
			i++
		case strings.HasPrefix(arg, "-f="):
			cfg.FileStoragePath = strings.TrimPrefix(arg, "-f=")
		}
	}

	return cfg, nil
}

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

func resolveFileStoragePath(defaultValue, cliValue string) string {
	if cliValue != "" {
		log.Info().Str("FileStoragePath", cliValue).Msg("Overriding FileStoragePath from CLI")
		return cliValue
	}

	log.Info().Str("FileStoragePath", defaultValue).Msg("Using default FileStoragePath")
	return defaultValue
}

// проверка корректности host:port
func isValidAddress(addr string) bool {
	_, err := net.ResolveTCPAddr("tcp", addr)
	return err == nil
}

// Проверка корректности URL
func isValidURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme != "" && u.Host != ""
}
