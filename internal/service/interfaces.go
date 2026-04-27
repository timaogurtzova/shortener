package service

import (
	"context"

	"github.com/timaogurtzova/shortener/internal/model"
)

// URLShortener описывает контракт сервиса сокращения URL для HTTP-обработчиков.
type URLShortener interface {
	Create(ctx context.Context, url, userID string) (string, error)
	CreateBatch(ctx context.Context, urls []string, userID string) ([]string, error)
	Resolve(ctx context.Context, id string) (string, error)
	FindByUserID(ctx context.Context, userID string) ([]model.UserURL, error)
}

// HealthChecker описывает проверку доступности внешних зависимостей сервиса.
type HealthChecker interface {
	Ping(ctx context.Context) error
}
