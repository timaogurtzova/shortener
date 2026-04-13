package service

import "context"

// URLShortener описывает контракт сервиса сокращения URL для HTTP-обработчиков.
type URLShortener interface {
	Create(ctx context.Context, url string) (string, error)
	CreateBatch(ctx context.Context, urls []string) ([]string, error)
	Resolve(ctx context.Context, id string) (string, error)
}

// HealthChecker описывает проверку доступности внешних зависимостей сервиса.
type HealthChecker interface {
	Ping(ctx context.Context) error
}
