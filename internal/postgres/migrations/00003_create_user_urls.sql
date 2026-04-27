-- +goose Up
CREATE TABLE IF NOT EXISTS user_urls (
    id BIGSERIAL PRIMARY KEY,
    user_id TEXT NOT NULL,
    short_url TEXT NOT NULL,
    UNIQUE (user_id, short_url)
);

CREATE INDEX IF NOT EXISTS user_urls_user_id_idx
    ON user_urls (user_id, id);

-- +goose Down
DROP INDEX IF EXISTS user_urls_user_id_idx;
DROP TABLE IF EXISTS user_urls;
