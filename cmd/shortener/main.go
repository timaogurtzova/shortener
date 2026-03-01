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
	//Загружаем конфигурацию из yml
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("Error loading config")
		return
	}

	repo := repository.NewInMemoryStore()
	svc := service.NewShortenerService(repo)

	createHandler := &handler.CreateHandler{Service: svc}
	redirectHandler := &handler.RedirectHandler{Service: svc}

	// Создаём сервер и запускаем его
	srv := http.NewServer(cfg, createHandler, redirectHandler)
	srv.Run()
}
