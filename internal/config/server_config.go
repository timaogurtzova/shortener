package config

import (
	"fmt"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/rs/zerolog/log"
)

type Configuration struct {
	Server ServerConfiguration `yaml:"server"`
}
type ServerConfiguration struct {
	Port         string        `yaml:"port"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

const DefaultConfigPath = "config.yaml"

func LoadConfig() (*Configuration, error) {
	var cfg Configuration

	err := cleanenv.ReadConfig(DefaultConfigPath, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	log.Info().Msg("Config read")

	return &cfg, nil
}
