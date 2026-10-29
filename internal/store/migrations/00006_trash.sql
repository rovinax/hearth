-- +goose Up

ALTER TABLE files ADD COLUMN deleted_at TEXT NOT NULL DEFAULT '';

CREATE INDEX files_deleted_idx ON files (deleted_at);

-- +goose Down

DROP INDEX files_deleted_idx;
ALTER TABLE files DROP COLUMN deleted_at;
