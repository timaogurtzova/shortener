package config

import (
	"flag"
	"fmt"
	"net/url"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/rs/zerolog/log"
)

type Configuration struct {
	Server ServerConfiguration `yaml:"server"`
}
type ServerConfiguration struct {
	Address      string        `yaml:"address"`
	BaseURL      string        `yaml:"base_url"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

const DefaultConfigPath = "config.yaml"

// LoadConfig загружает конфигурацию из YAML и переопределяет её аргументами командной строки
func LoadConfig() (*Configuration, error) {
	var cfg Configuration

	// 1. YAML для значений по умолчанию
	if err := cleanenv.ReadConfig(DefaultConfigPath, &cfg); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	log.Info().Msg("Config loaded from YAML")

	// 2. Аргументы командной строки
	cliAddr := flag.String("a", "", "Address for HTTP server (host:port)")
	cliBaseURL := flag.String("b", "", "Base URL for short links")
	flag.Parse()

	// 3. Переопределяем значения, если флаги заданы и они корректные
	if cliAddr != nil && *cliAddr != "" {
		if isValidAddress(*cliAddr) {
			cfg.Server.Address = *cliAddr
			log.Info().Str("Address", *cliAddr).Msg("Overriding Address from CLI")
		} else {
			log.Warn().Str("Address", *cliAddr).Msg("Invalid CLI Address, using YAML value")
		}
	} else {
		log.Info().Msg("CLI flag -a not provided, using YAML Address")
	}

	if cliBaseURL != nil && *cliBaseURL != "" {
		if isValidURL(*cliBaseURL) {
			cfg.Server.BaseURL = *cliBaseURL
			log.Info().Str("BaseURL", *cliBaseURL).Msg("Overriding BaseURL from CLI")
		} else {
			log.Warn().Str("BaseURL", *cliBaseURL).Msg("Invalid CLI BaseURL, using YAML value")
		}
	} else {
		log.Info().Msg("CLI flag -b not provided, using YAML BaseURL")
	}

	return &cfg, nil
}

// проверка корректности host:port
func isValidAddress(addr string) bool {
	return len(addr) > 0 && (addr[0] == ':' || containsPort(addr))
}

func containsPort(addr string) bool {
	for i := 0; i < len(addr); i++ {
		if addr[i] == ':' {
			return true
		}
	}
	return false
}

// Проверка корректности URL
func isValidURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme != "" && u.Host != ""
}
