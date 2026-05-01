-- +goose Up
ALTER TABLE user_urls
    DROP CONSTRAINT IF EXISTS user_urls_short_url_fkey;

-- +goose Down
ALTER TABLE user_urls
    ADD CONSTRAINT user_urls_short_url_fkey
    FOREIGN KEY (short_url) REFERENCES short_urls(short_url) ON DELETE CASCADE;
