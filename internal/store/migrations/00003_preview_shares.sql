-- +goose Up
ALTER TABLE files ADD COLUMN taken_at TEXT NOT NULL DEFAULT '';
ALTER TABLE files ADD COLUMN thumb_rel TEXT NOT NULL DEFAULT '';

CREATE TABLE shares (
    id INTEGER PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    file_id INTEGER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    created_by INTEGER NOT NULL REFERENCES users(id),
    expires_at TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX shares_file_idx ON shares (file_id);

-- +goose Down
DROP TABLE shares;
ALTER TABLE files DROP COLUMN thumb_rel;
ALTER TABLE files DROP COLUMN taken_at;
