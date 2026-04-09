-- +goose Up
DELETE FROM short_urls
WHERE id IN (
    SELECT id
    FROM (
        SELECT id,
               ROW_NUMBER() OVER (PARTITION BY original_url ORDER BY id) AS row_number
        FROM short_urls
    ) duplicated_rows
    WHERE row_number > 1
);

CREATE UNIQUE INDEX IF NOT EXISTS short_urls_original_url_idx
    ON short_urls (original_url);

-- +goose Down
DROP INDEX IF EXISTS short_urls_original_url_idx;
