package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/timaogurtzova/shortener/internal/audit"
	"github.com/timaogurtzova/shortener/internal/auth"
	"github.com/timaogurtzova/shortener/internal/config"
	httpserver "github.com/timaogurtzova/shortener/internal/http"
	"github.com/timaogurtzova/shortener/internal/http/handler"
	"github.com/timaogurtzova/shortener/internal/postgres"
	"github.com/timaogurtzova/shortener/internal/repository"
	"github.com/timaogurtzova/shortener/internal/service"
)

const (
	auditQueueSize       = 1024
	auditDeliveryTimeout = 2 * time.Second
	auditShutdownTimeout = 5 * time.Second
	emptyBuildValue      = "N/A"
)

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func main() {
	printBuildInfo(os.Stdout)

	if err := run(); err != nil {
		log.Fatal().Err(err).Msg("application stopped with error")
	}
}

func printBuildInfo(w io.Writer) {
	fmt.Fprintf(w, "Build version: %s\n", buildValue(buildVersion))
	fmt.Fprintf(w, "Build date: %s\n", buildValue(buildDate))
	fmt.Fprintf(w, "Build commit: %s\n", buildValue(buildCommit))
}

func buildValue(value string) string {
	if value == "" {
		return emptyBuildValue
	}

	return value
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
			if closeErr := database.Close(); closeErr != nil {
				log.Error().Err(closeErr).Msg("Error closing database connection")
			}
		}()
	}

	// Выбираем хранилище URL по конфигурации.
	repo, err := newURLRepository(cfg, database)
	if err != nil {
		return fmt.Errorf("initialize url repository: %w", err)
	}

	// Собираем сервисный и HTTP-слои приложения.
	serviceCtx, cancelService := context.WithCancel(context.Background())
	defer cancelService()

	svc := service.NewShortenerService(serviceCtx, repo)
	authSecret, err := auth.NewRandomSecret(32)
	if err != nil {
		return fmt.Errorf("generate auth secret: %w", err)
	}

	authenticator, err := auth.NewAuthenticator(authSecret)
	if err != nil {
		return fmt.Errorf("initialize authenticator: %w", err)
	}

	auditDispatcher, err := newAuditDispatcher(cfg.Audit)
	if err != nil {
		return fmt.Errorf("initialize audit dispatcher: %w", err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), auditShutdownTimeout)
		defer cancel()
		if err := auditDispatcher.Close(closeCtx); err != nil {
			log.Error().Err(err).Msg("failed to close audit dispatcher")
		}
	}()

	createHandler := handler.NewCreateHandler(svc, cfg.Server.BaseURL, authenticator)
	userHandler := handler.NewUserHandler(svc, cfg.Server.BaseURL, authenticator)
	redirectHandler := handler.NewRedirectHandler(svc)
	createHandler.SetAuditPublisher(auditDispatcher)
	redirectHandler.SetAuditPublisher(auditDispatcher)
	redirectHandler.SetUserIDResolver(authenticator)
	pingHandler := handler.NewPingHandler(database)
	router := httpserver.NewRouter(httpserver.RouterHandlers{
		CreateShortURLPlainText: createHandler.CreateShortURLPlainText,
		CreateShortURLJSON:      createHandler.CreateShortURLJSON,
		CreateShortURLBatchJSON: createHandler.CreateShortURLBatchJSON,
		GetUserURLs:             userHandler.GetUserURLs,
		DeleteUserURLs:          userHandler.DeleteUserURLs,
		Redirect:                redirectHandler.Redirect,
		Ping:                    pingHandler.Ping,
	})

	// Запускаем HTTP-сервер.
	server := httpserver.NewServer(cfg, router)
	if err := server.Run(); err != nil {
		return fmt.Errorf("run http server: %w", err)
	}

	return nil
}

func newAuditDispatcher(cfg config.AuditConfiguration) (*audit.Dispatcher, error) {
	publisher := audit.NewPublisher()

	if cfg.FileEnabled() {
		fileObserver, err := audit.NewFileObserver(*cfg.FilePath)
		if err != nil {
			return nil, err
		}
		publisher.Register(fileObserver)
	}

	if cfg.RemoteEnabled() {
		httpObserver, err := audit.NewHTTPObserver(*cfg.URL)
		if err != nil {
			if closeErr := publisher.Close(); closeErr != nil {
				return nil, errors.Join(
					fmt.Errorf("create http audit observer: %w", err),
					fmt.Errorf("close audit observers: %w", closeErr),
				)
			}
			return nil, err
		}
		publisher.Register(httpObserver)
	}

	return audit.NewDispatcher(publisher, audit.DispatcherConfig{
		QueueSize:       auditQueueSize,
		DeliveryTimeout: auditDeliveryTimeout,
	}), nil
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
