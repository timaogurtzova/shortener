package main

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/timaogurtzova/shortener/internal/config"
	httpserver "github.com/timaogurtzova/shortener/internal/http"
	"github.com/timaogurtzova/shortener/internal/http/handler"
	"github.com/timaogurtzova/shortener/internal/postgres"
	"github.com/timaogurtzova/shortener/internal/repository"
	"github.com/timaogurtzova/shortener/internal/service"
)

func main() {
	if err := run(); err != nil {
		log.Fatal().Err(err).Msg("application stopped with error")
	}
}

// run инициализирует зависимости приложения и запускает HTTP-сервер.
func run() error {
	// Загружаем конфигурацию приложения.
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Инициализируем базу данных, если она явно настроена.
	database, err := postgres.Open(context.Background(), cfg.Database)
	if err != nil {
		return fmt.Errorf("initialize database connection: %w", err)
	}
	if database != nil {
		defer func() {
			if err := database.Close(); err != nil {
				log.Error().Err(err).Msg("Error closing database connection")
			}
		}()
	}

	// Выбираем хранилище URL по конфигурации.
	repo, err := newURLRepository(cfg, database)
	if err != nil {
		return fmt.Errorf("initialize url repository: %w", err)
	}

	// Собираем сервисный и HTTP-слои приложения.
	svc := service.NewShortenerService(repo)
	createHandler := handler.NewCreateHandler(svc, cfg.Server.BaseURL)
	redirectHandler := handler.NewRedirectHandler(svc)
	pingHandler := handler.NewPingHandler(database)
	router := httpserver.NewRouter(createHandler.CreateShortURLPlainText, createHandler.CreateShortURLJSON, redirectHandler.Redirect, pingHandler.Ping)

	// Запускаем HTTP-сервер.
	server := httpserver.NewServer(cfg, router)
	if err := server.Run(); err != nil {
		return fmt.Errorf("run http server: %w", err)
	}

	return nil
}

// newURLRepository выбирает хранилище URL по приоритету:
// PostgreSQL -> файл -> память.
func newURLRepository(cfg *config.Configuration, database *postgres.Database) (repository.URLRepository, error) {
	if cfg.Database.IsConfigured() {
		log.Info().Msg("Using PostgreSQL storage")
		if database == nil {
			return nil, postgres.ErrDatabaseNotConfigured
		}
		return repository.NewDBStore(database.SQLDB())
	}

	if cfg.Storage.IsConfigured() {
		log.Info().Str("FileStoragePath", cfg.Storage.Path()).Msg("Using file storage")
		return repository.NewFileStore(cfg.Storage.Path())
	}

	log.Info().Msg("Using in-memory storage")
	return repository.NewInMemoryStore(), nil
}
