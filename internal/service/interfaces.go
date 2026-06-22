package service

import (
	"context"

	"github.com/timaogurtzova/shortener/internal/model"
)

// URLShortener описывает контракт сервиса сокращения URL для HTTP-обработчиков.
type URLShortener interface {
	// Create сохраняет исходный URL пользователя и возвращает короткий идентификатор.
	Create(ctx context.Context, url, userID string) (string, error)

	// CreateBatch сохраняет пакет исходных URL пользователя и возвращает короткие идентификаторы.
	CreateBatch(ctx context.Context, urls []string, userID string) ([]string, error)

	// Resolve возвращает исходный URL по короткому идентификатору.
	Resolve(ctx context.Context, id string) (string, error)

	// FindByUserID возвращает все URL, сокращённые указанным пользователем.
	FindByUserID(ctx context.Context, userID string) ([]model.UserURL, error)

	// DeleteUserURLs ставит короткие URL пользователя в очередь удаления.
	DeleteUserURLs(ctx context.Context, userID string, shortIDs []string) error
}

// HealthChecker описывает проверку доступности внешних зависимостей сервиса.
type HealthChecker interface {
	// Ping проверяет готовность зависимости к работе.
	Ping(ctx context.Context) error
}
