package main

import (
	"github.com/rs/zerolog/log"
	"github.com/timaogurtzova/shortener/internal/config"
	"github.com/timaogurtzova/shortener/internal/http"
	"github.com/timaogurtzova/shortener/internal/http/handler"
	"github.com/timaogurtzova/shortener/internal/repository"
	"github.com/timaogurtzova/shortener/internal/service"
)

func main() {
	//Configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("Error loading config")
		return
	}
	//Repository
	repo := repository.NewInMemoryStore()
	//Service
	svc := service.NewShortenerService(repo)
	//Handlers
	createHandler := handler.NewCreateHandler(svc, cfg.Server.BaseURL)
	redirectHandler := handler.NewRedirectHandler(svc)
	router := httpserver.NewRouter(createHandler.Create, redirectHandler.Redirect)
	// HTTP server
	server := httpserver.NewServer(cfg, router)
	if err := server.Run(); err != nil {
		log.Fatal().Err(err).Msg("server stopped with error")
	}
}
