package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"
)

const (
	insertShortURLQuery = `
		INSERT INTO short_urls (short_url, original_url)
		VALUES ($1, $2)
	`
	selectOriginalURLQuery = `
		SELECT original_url
		FROM short_urls
		WHERE short_url = $1
	`
)

// DBStore хранит сокращённые URL в базе данных.
type DBStore struct {
	db *sql.DB
}

// NewDBStore создаёт хранилище URL на базе SQL-подключения.
func NewDBStore(db *sql.DB) (*DBStore, error) {
	if db == nil {
		return nil, errors.New("database is not configured")
	}

	return &DBStore{db: db}, nil
}

// Store сохраняет исходный URL по короткому идентификатору.
func (s *DBStore) Store(id, url string) error {
	_, err := s.db.ExecContext(context.Background(), insertShortURLQuery, id, url)
	if err == nil {
		return nil
	}

	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return errors.New("id already exists")
	}

	return err
}

// Load возвращает исходный URL по короткому идентификатору.
func (s *DBStore) Load(id string) (string, error) {
	var originalURL string

	err := s.db.QueryRowContext(context.Background(), selectOriginalURLQuery, id).Scan(&originalURL)
	if err == nil {
		return originalURL, nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return "", errors.New("not found")
	}

	return "", err
}
