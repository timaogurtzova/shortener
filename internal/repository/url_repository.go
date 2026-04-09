package repository

import "errors"

var (
	ErrIDAlreadyExists  = errors.New("id already exists")
	ErrURLAlreadyExists = errors.New("url already exists")
	ErrNotFound         = errors.New("not found")
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
}

// URLRepository описывает контракт хранилища URL.
type URLRepository interface {
	Store(id, url string) error
	BatchStore(records []BatchRecord) error
	Load(id string) (string, error)
}
