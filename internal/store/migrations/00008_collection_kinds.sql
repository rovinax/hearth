-- +goose Up
ALTER TABLE collections ADD COLUMN media_kinds TEXT NOT NULL DEFAULT '';
UPDATE collections SET media_kinds = media_kind WHERE media_kinds = '';

-- +goose Down
ALTER TABLE collections DROP COLUMN media_kinds;
