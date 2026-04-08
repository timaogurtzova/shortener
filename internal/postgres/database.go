package postgres

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"github.com/timaogurtzova/shortener/internal/config"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// ErrDatabaseNotConfigured возвращается, когда PostgreSQL не был настроен или не инициализирован.
var ErrDatabaseNotConfigured = errors.New("database is not configured")

// Database инкапсулирует подключение к базе данных и инфраструктурные операции над ним.
type Database struct {
	db *sql.DB
}

// Open создаёт подключение к PostgreSQL, проверяет его и применяет миграции.
func Open(ctx context.Context, cfg config.DatabaseConfiguration) (*Database, error) {
	if !cfg.IsConfigured() {
		return nil, nil
	}

	sqlDB, err := sql.Open("postgres", *cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if err := applyMigrations(sqlDB); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("apply migrations: %w", err)
	}

	return NewDatabase(sqlDB)
}

// NewDatabase создаёт инфраструктурную обёртку над готовым подключением к БД.
func NewDatabase(sqlDB *sql.DB) (*Database, error) {
	if sqlDB == nil {
		return nil, ErrDatabaseNotConfigured
	}

	return &Database{db: sqlDB}, nil
}

// SQLDB возвращает низкоуровневое подключение к базе данных для репозиториев.
func (d *Database) SQLDB() *sql.DB {
	if d == nil {
		return nil
	}

	return d.db
}

// Ping проверяет доступность базы данных.
func (d *Database) Ping(ctx context.Context) error {
	if d == nil || d.db == nil {
		return ErrDatabaseNotConfigured
	}

	return d.db.PingContext(ctx)
}

// Close закрывает подключение к базе данных.
func (d *Database) Close() error {
	if d == nil || d.db == nil {
		return nil
	}

	return d.db.Close()
}

func applyMigrations(db *sql.DB) error {
	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("run goose migrations: %w", err)
	}

	return nil
}
