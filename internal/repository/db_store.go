package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"
)

const (
	storeShortURLQuery = `
		WITH inserted AS (
			INSERT INTO short_urls (short_url, original_url)
			VALUES ($1, $2)
			ON CONFLICT (original_url) DO NOTHING
			RETURNING short_url, TRUE AS created
		)
		SELECT short_url, created
		FROM inserted
		UNION ALL
		SELECT short_url, FALSE AS created
		FROM short_urls
		WHERE original_url = $2
		  AND NOT EXISTS (SELECT 1 FROM inserted)
		LIMIT 1
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
	var storedID string
	var created bool

	err := s.db.QueryRowContext(context.Background(), storeShortURLQuery, id, url).Scan(&storedID, &created)
	if err == nil {
		if !created {
			return &URLConflictError{ShortID: storedID}
		}

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

	for _, record := range records {
		var storedID string
		var created bool

		err = tx.QueryRowContext(context.Background(), storeShortURLQuery, record.ID, record.OriginalURL).Scan(&storedID, &created)
		if err == nil {
			if !created {
				return &URLConflictError{ShortID: storedID}
			}

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
