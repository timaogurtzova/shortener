package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
	"github.com/timaogurtzova/shortener/internal/config"
	httpmiddleware "github.com/timaogurtzova/shortener/internal/http/middleware"
)

// Server управляет жизненным циклом HTTP-сервера приложения.
type Server struct {
	httpServer  *http.Server
	enableHTTPS bool
}

// RouterHandlers объединяет HTTP-обработчики роутера по именованным полям.
type RouterHandlers struct {
	// CreateShortURLPlainText обрабатывает POST / с исходным URL в теле запроса.
	CreateShortURLPlainText http.HandlerFunc

	// CreateShortURLJSON обрабатывает POST /api/shorten с JSON-телом запроса.
	CreateShortURLJSON http.HandlerFunc

	// CreateShortURLBatchJSON обрабатывает POST /api/shorten/batch с пакетом URL.
	CreateShortURLBatchJSON http.HandlerFunc

	// GetUserURLs обрабатывает GET /api/user/urls для истории пользователя.
	GetUserURLs http.HandlerFunc

	// DeleteUserURLs обрабатывает DELETE /api/user/urls для удаления URL пользователя.
	DeleteUserURLs http.HandlerFunc

	// Redirect обрабатывает GET /{id} и перенаправляет на исходный URL.
	Redirect http.HandlerFunc

	// Ping обрабатывает GET /ping для проверки доступности хранилища.
	Ping http.HandlerFunc
}

// NewServer создаёт HTTP-сервер с адресом, роутером и таймаутами из конфигурации.
func NewServer(cfg *config.Configuration, router http.Handler) *Server {
	return &Server{
		enableHTTPS: cfg.Server.EnableHTTPS,
		httpServer: &http.Server{
			Addr:         cfg.Server.Address,
			Handler:      router,
			IdleTimeout:  cfg.Server.IdleTimeout,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
		},
	}
}

// NewRouter создаёт HTTP-роутер приложения и регистрирует все публичные эндпоинты.
func NewRouter(handlers RouterHandlers) http.Handler {
	r := chi.NewRouter()

	r.Use(httpmiddleware.Logging)
	r.Use(httpmiddleware.GunzipRequest)
	r.Use(chimiddleware.Compress(5, "application/json", "text/html"))
	// Роуты приложения.
	r.Post("/", handlers.CreateShortURLPlainText)
	r.Post("/api/shorten", handlers.CreateShortURLJSON)
	r.Post("/api/shorten/batch", handlers.CreateShortURLBatchJSON)
	r.Get("/api/user/urls", handlers.GetUserURLs)
	r.Delete("/api/user/urls", handlers.DeleteUserURLs)
	r.Get("/ping", handlers.Ping)
	r.Get("/{id}", handlers.Redirect)

	// Ответы по умолчанию для неизвестных путей и методов.
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusBadRequest)
	})

	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "method not allowed", http.StatusBadRequest)
	})

	return r
}

// Run запускает HTTP-сервер и выполняет корректное завершение по сигналу ОС или ошибке сервера.
func (s *Server) Run() error {
	errChan := make(chan error, 1)
	// Запуск сервера в отдельной горутине
	go func() {
		protocol := "HTTP"
		if s.enableHTTPS {
			protocol = "HTTPS"
		}
		log.Info().
			Str("addr", s.httpServer.Addr).
			Msg(protocol + " server started")

		if err := s.listenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}

		log.Info().Msg("Stopped serving new connections")
		close(errChan)
	}()

	// Канал сигналов ОС
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Ждём либо сигнала ОС, либо ошибки сервера
	select {
	case sig := <-sigChan:
		log.Info().Str("signal", sig.String()).Msg("shutdown signal received")
	case err := <-errChan:
		if err != nil {
			return fmt.Errorf("server error: %w", err)
		}
	}

	// Корректное завершение работы сервера.
	shutdownTimeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	log.Info().Msg("shutting down http server")

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("could not gracefully shutdown: %w", err)
	}

	log.Info().Msg("Server shutdown gracefully")
	return nil
}

// listenAndServe выбирает HTTP- или HTTPS-режим согласно конфигурации сервера.
func (s *Server) listenAndServe() error {
	if !s.enableHTTPS {
		return s.httpServer.ListenAndServe()
	}

	tlsConfig, err := newTLSConfig(s.httpServer.Addr)
	if err != nil {
		return fmt.Errorf("create TLS config: %w", err)
	}
	s.httpServer.TLSConfig = tlsConfig

	// Сертификат уже находится в TLSConfig, поэтому пути к файлам не требуются.
	return s.httpServer.ListenAndServeTLS("", "")
}
