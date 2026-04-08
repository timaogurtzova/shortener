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
		return ErrIDAlreadyExists
	}

	return err
}

// BatchStore сохраняет пакет URL в рамках одной транзакции.
func (s *DBStore) BatchStore(records []BatchRecord) error {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(context.Background(), insertShortURLQuery)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, record := range records {
		_, err = stmt.ExecContext(context.Background(), record.ID, record.OriginalURL)
		if err == nil {
			continue
		}

		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return ErrIDAlreadyExists
		}

		return err
	}

	return tx.Commit()
}

// Load возвращает исходный URL по короткому идентификатору.
func (s *DBStore) Load(id string) (string, error) {
	var originalURL string

	err := s.db.QueryRowContext(context.Background(), selectOriginalURLQuery, id).Scan(&originalURL)
	if err == nil {
		return originalURL, nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}

	return "", err
}
