-- +goose Up
CREATE TABLE IF NOT EXISTS short_urls (
    id BIGSERIAL PRIMARY KEY,
    short_url TEXT NOT NULL UNIQUE,
    original_url TEXT NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS short_urls;
