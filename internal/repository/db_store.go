package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"
	"github.com/timaogurtzova/shortener/internal/model"
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
	insertUserURLQuery = `
		INSERT INTO user_urls (user_id, short_url)
		VALUES ($1, $2)
		ON CONFLICT (user_id, short_url) DO NOTHING
	`
	selectUserURLsQuery = `
		SELECT short_urls.short_url, short_urls.original_url
		FROM user_urls
		JOIN short_urls ON short_urls.short_url = user_urls.short_url
		WHERE user_urls.user_id = $1
		ORDER BY user_urls.id
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
func (s *DBStore) Store(ctx context.Context, id, url, userID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var storedID string
	var created bool

	err = tx.QueryRowContext(ctx, storeShortURLQuery, id, url).Scan(&storedID, &created)
	if err == nil {
		if err = s.storeUserURL(ctx, tx, userID, storedID); err != nil {
			return err
		}

		if !created {
			if err = tx.Commit(); err != nil {
				return err
			}

			return &URLConflictError{ShortID: storedID}
		}

		return tx.Commit()
	}

	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return ErrIDAlreadyExists
	}

	return err
}

// BatchStore сохраняет пакет URL в рамках одной транзакции.
func (s *DBStore) BatchStore(ctx context.Context, records []BatchRecord) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, record := range records {
		var storedID string
		var created bool

		err = tx.QueryRowContext(ctx, storeShortURLQuery, record.ID, record.OriginalURL).Scan(&storedID, &created)
		if err == nil {
			if err = s.storeUserURL(ctx, tx, record.UserID, storedID); err != nil {
				return err
			}

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
func (s *DBStore) Load(ctx context.Context, id string) (string, error) {
	var originalURL string

	err := s.db.QueryRowContext(ctx, selectOriginalURLQuery, id).Scan(&originalURL)
	if err == nil {
		return originalURL, nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}

	return "", err
}

// FindByUserID возвращает все URL, сокращённые пользователем.
func (s *DBStore) FindByUserID(ctx context.Context, userID string) ([]model.UserURL, error) {
	rows, err := s.db.QueryContext(ctx, selectUserURLsQuery, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]model.UserURL, 0)
	for rows.Next() {
		var item model.UserURL
		if err := rows.Scan(&item.ShortID, &item.OriginalURL); err != nil {
			return nil, err
		}

		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (s *DBStore) storeUserURL(ctx context.Context, tx *sql.Tx, userID, shortID string) error {
	if userID == "" {
		return nil
	}

	_, err := tx.ExecContext(ctx, insertUserURLQuery, userID, shortID)
	return err
}
