package service

import "context"

// URLShortener описывает контракт сервиса сокращения URL для HTTP-обработчиков.
type URLShortener interface {
	Create(string) (string, error)
	Resolve(string) (string, error)
}

// HealthChecker описывает проверку доступности внешних зависимостей сервиса.
type HealthChecker interface {
	Ping(ctx context.Context) error
}
