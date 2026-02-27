package main

import (
	"github.com/timaogurtzova/shortener/internal/http"
	"github.com/timaogurtzova/shortener/internal/http/handler"
	"github.com/timaogurtzova/shortener/internal/repository"
	"github.com/timaogurtzova/shortener/internal/service"
)

func main() {
	baseURL := "http://localhost:8080"
	addr := ":8080"

	repo := repository.NewInMemoryStore()
	svc := service.NewShortenerService(repo)

	createHandler := &handler.CreateHandler{Service: svc, BaseURL: baseURL}
	redirectHandler := &handler.RedirectHandler{Service: svc}

	// Создаём сервер и запускаем его
	srv := http.NewServer(addr, createHandler, redirectHandler)
	srv.Run()
}
