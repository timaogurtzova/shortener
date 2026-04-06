package repository

import (
	"context"
	"database/sql"
	"errors"
)

type dbPinger interface {
	PingContext(ctx context.Context) error
}

// DBHealthChecker проверяет доступность базы данных через PingContext.
type DBHealthChecker struct {
	db dbPinger
}

// NewDBHealthChecker создаёт проверку доступности базы данных.
func NewDBHealthChecker(db *sql.DB) *DBHealthChecker {
	return &DBHealthChecker{db: db}
}

// Ping проверяет соединение с базой данных.
func (c *DBHealthChecker) Ping(ctx context.Context) error {
	if c.db == nil {
		return errors.New("database is not configured")
	}

	return c.db.PingContext(ctx)
}
