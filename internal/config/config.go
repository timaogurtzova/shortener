package config

import (
	"flag"
	"net"
	"net/url"
	"time"

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

func defaultConfig() *Configuration {
	return &Configuration{
		Server: ServerConfiguration{
			Address:      "localhost:8080",
			BaseURL:      "http://localhost:8080",
			IdleTimeout:  60 * time.Second,
			ReadTimeout:  60 * time.Second,
			WriteTimeout: 60 * time.Second,
		},
	}
}

func LoadConfig() (*Configuration, error) {
	cfg := defaultConfig()

	// Аргументы командной строки
	cliAddr := flag.String("a", "", "Address for HTTP server (host:port)")
	cliBaseURL := flag.String("b", "", "Base URL for short links")
	flag.Parse()

	// Переопределяем значения, если флаги заданы и они корректные
	if *cliAddr != "" {
		if isValidAddress(*cliAddr) {
			cfg.Server.Address = *cliAddr
			log.Info().Str("Address", *cliAddr).Msg("Overriding Address from CLI")
		} else {
			log.Warn().Str("Address", *cliAddr).Msg("Invalid CLI Address, using YAML value")
		}
	} else {
		log.Info().Msg("CLI flag -a not provided, using default Address")
	}

	if *cliBaseURL != "" {
		if isValidURL(*cliBaseURL) {
			cfg.Server.BaseURL = *cliBaseURL
			log.Info().Str("BaseURL", *cliBaseURL).Msg("Overriding BaseURL from CLI")
		} else {
			log.Warn().Str("BaseURL", *cliBaseURL).Msg("Invalid CLI BaseURL, using YAML value")
		}
	} else {
		log.Info().Msg("CLI flag -b not provided, using default BaseURL")
	}

	return cfg, nil
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
