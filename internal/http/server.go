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
	"github.com/rs/zerolog/log"
	"github.com/timaogurtzova/shortener/internal/config"
)

type Server struct {
	httpServer *http.Server
}

func NewServer(cfg *config.Configuration, router http.Handler) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:         cfg.Server.Address,
			Handler:      router,
			IdleTimeout:  cfg.Server.IdleTimeout,
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
		},
	}
}

func NewRouter(createHandlerFunc, redirectHandlerFunc http.HandlerFunc) http.Handler {
	r := chi.NewRouter()

	r.Use(loggingMiddleware)
	// --- Routes ---
	r.Post("/", createHandlerFunc)
	r.Get("/{id}", redirectHandlerFunc)

	// --- Fallback ---
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusBadRequest)
	})

	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "method not allowed", http.StatusBadRequest)
	})

	return r
}

// --- Middleware для логирования ---
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info().
			Str("method", r.Method).
			Str("url", r.URL.String()).
			Msg("Incoming request")
		next.ServeHTTP(w, r)
	})
}

// Run
// — запускает сервер
// — обрабатывает graceful shutdown (и по сигналу, и по ошибке сервера)
func (s *Server) Run() error {
	errChan := make(chan error, 1)
	// Запуск сервера в отдельной горутине
	go func() {
		log.Info().
			Str("addr", s.httpServer.Addr).
			Msg("HTTP server started")

		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}

		log.Info().Msg("Stopped serving new connections")
		close(errChan)
	}()

	// Канал сигналов ОС
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Ждём либо сигнала ОС, либо ошибки сервера
	select {
	case sig := <-sigChan:
		log.Info().Str("signal", sig.String()).Msg("shutdown signal received")
	case err := <-errChan:
		if err != nil {
			return fmt.Errorf("server error: %w", err)
		}
	}

	// graceful shutdown
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
