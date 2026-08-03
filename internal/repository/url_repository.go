package repository

import (
	"context"
	"errors"

	"github.com/timaogurtzova/shortener/internal/model"
)

// Ошибки репозитория описывают ожидаемые причины отказа хранилища URL.
var (
	// ErrIDAlreadyExists возвращается, когда короткий идентификатор уже занят.
	ErrIDAlreadyExists = errors.New("id already exists")

	// ErrURLAlreadyExists возвращается, когда исходный URL уже был сохранён ранее.
	ErrURLAlreadyExists = errors.New("url already exists")

	// ErrNotFound возвращается, когда короткий идентификатор не найден.
	ErrNotFound = errors.New("not found")

	// ErrDeleted возвращается, когда короткий URL помечен как удалённый.
	ErrDeleted = errors.New("url deleted")
)

// URLConflictError сообщает, что исходный URL уже был сокращён ранее.
type URLConflictError struct {
	// ShortID хранит уже существующий короткий идентификатор для исходного URL.
	ShortID string
}

// Error возвращает текст ошибки конфликта исходного URL.
func (e *URLConflictError) Error() string {
	return ErrURLAlreadyExists.Error()
}

// Unwrap возвращает базовую ошибку ErrURLAlreadyExists.
func (e *URLConflictError) Unwrap() error {
	return ErrURLAlreadyExists
}

// BatchRecord описывает одну запись для пакетного сохранения.
type BatchRecord struct {
	// ID содержит короткий идентификатор URL.
	ID string

	// OriginalURL содержит исходный URL.
	OriginalURL string

	// UserID содержит идентификатор пользователя, создавшего короткий URL.
	UserID string
}

// URLRepository описывает контракт хранилища URL.
type URLRepository interface {
	StatsProvider

	// Store сохраняет исходный URL по короткому идентификатору.
	Store(ctx context.Context, id, url, userID string) error

	// BatchStore сохраняет пакет исходных URL.
	BatchStore(ctx context.Context, records []BatchRecord) error

	// Load возвращает исходный URL по короткому идентификатору.
	Load(ctx context.Context, id string) (string, error)

	// FindByUserID возвращает все URL, связанные с пользователем.
	FindByUserID(ctx context.Context, userID string) ([]model.UserURL, error)

	// MarkDeleted помечает указанные короткие URL пользователя как удалённые.
	MarkDeleted(ctx context.Context, userID string, shortIDs []string) error
}

// StatsProvider описывает получение агрегированной статистики хранилища.
type StatsProvider interface {
	// GetStats возвращает количество сокращённых URL и уникальных пользователей.
	GetStats(ctx context.Context) (model.Stats, error)
}
