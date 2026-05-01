package repository

import (
	"context"
	"errors"

	"github.com/timaogurtzova/shortener/internal/model"
)

var (
	ErrIDAlreadyExists  = errors.New("id already exists")
	ErrURLAlreadyExists = errors.New("url already exists")
	ErrNotFound         = errors.New("not found")
	ErrDeleted          = errors.New("url deleted")
)

// URLConflictError сообщает, что исходный URL уже был сокращён ранее.
type URLConflictError struct {
	ShortID string
}

func (e *URLConflictError) Error() string {
	return ErrURLAlreadyExists.Error()
}

func (e *URLConflictError) Unwrap() error {
	return ErrURLAlreadyExists
}

// BatchRecord описывает одну запись для пакетного сохранения.
type BatchRecord struct {
	ID          string
	OriginalURL string
	UserID      string
}

// URLRepository описывает контракт хранилища URL.
type URLRepository interface {
	Store(ctx context.Context, id, url, userID string) error
	BatchStore(ctx context.Context, records []BatchRecord) error
	Load(ctx context.Context, id string) (string, error)
	FindByUserID(ctx context.Context, userID string) ([]model.UserURL, error)
	MarkDeleted(ctx context.Context, userID string, shortIDs []string) error
}
