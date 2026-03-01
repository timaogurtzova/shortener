package http

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"github.com/timaogurtzova/shortener/internal/config"
)

type Server struct {
	server *http.Server
	router chi.Router
	cfg    *config.Configuration
}

func NewServer(cfg *config.Configuration, createHandler, redirectHandler http.Handler) *Server {
	s := &Server{
		router: configureRouter(createHandler, redirectHandler),
		cfg:    cfg,
	}
	s.server = s.setupHTTPServer()

	return s
}

func configureRouter(createHandler, redirectHandler http.Handler) chi.Router {
	r := chi.NewRouter()
	r.Post("/", createHandler.ServeHTTP)
	r.Get("/{id}", redirectHandler.ServeHTTP)

	// Любой неразрешённый метод на существующем пути
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	})

	// Любой несуществующий путь
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	})
	return r
}

func (s *Server) setupHTTPServer() *http.Server {
	return &http.Server{
		Addr:         fmt.Sprintf(":%s", strings.TrimPrefix(s.cfg.Server.Port, ":")),
		Handler:      s.router,
		IdleTimeout:  s.cfg.Server.IdleTimeout,
		ReadTimeout:  s.cfg.Server.ReadTimeout,
		WriteTimeout: s.cfg.Server.WriteTimeout,
	}
}

// Run запускает HTTP-сервер на указанном адресе
func (s *Server) Run() {
	log.Info().
		Str("addr", s.server.Addr).
		Msg("Starting HTTP server...")
	if err := s.server.ListenAndServe(); err != nil {
		log.Fatal().Err(err).Msg("HTTP server error")
	}
}
