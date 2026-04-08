package repository

import "errors"

var (
	ErrIDAlreadyExists = errors.New("id already exists")
	ErrNotFound        = errors.New("not found")
)

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
