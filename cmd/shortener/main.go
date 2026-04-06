package main

import (
	"database/sql"

	_ "github.com/lib/pq"
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
	repo, err := repository.NewFileStore(cfg.Storage.FileStoragePath)
	if err != nil {
		log.Fatal().Err(err).Msg("Error initializing file storage")
		return
	}
	//Database
	var db *sql.DB
	if cfg.Database.DSN != "" {
		db, err = sql.Open("postgres", cfg.Database.DSN)
		if err != nil {
			log.Fatal().Err(err).Msg("Error initializing database connection")
			return
		}
		defer db.Close()
	}
	//Service
	svc := service.NewShortenerService(repo)
	//Handlers
	createHandler := handler.NewCreateHandler(svc, cfg.Server.BaseURL)
	redirectHandler := handler.NewRedirectHandler(svc)
	pingHandler := handler.NewPingHandler(repository.NewDBHealthChecker(db))
	router := httpserver.NewRouter(createHandler.CreateShortURLPlainText, createHandler.CreateShortURLJSON, redirectHandler.Redirect, pingHandler.Ping)
	// HTTP server
	server := httpserver.NewServer(cfg, router)
	if err := server.Run(); err != nil {
		log.Fatal().Err(err).Msg("server stopped with error")
	}
}
