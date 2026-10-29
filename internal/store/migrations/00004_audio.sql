-- +goose NO TRANSACTION
-- +goose Up
PRAGMA foreign_keys=OFF;

CREATE TABLE files_audio (
    id INTEGER PRIMARY KEY,
    owner_id INTEGER NOT NULL REFERENCES users(id),
    blob_id INTEGER NOT NULL REFERENCES blobs(id),
    name TEXT NOT NULL,
    media_kind TEXT NOT NULL CHECK (media_kind IN ('image', 'video', 'audio', 'other')),
    mime TEXT NOT NULL,
    visibility TEXT NOT NULL DEFAULT 'private' CHECK (visibility IN ('private', 'shared')),
    taken_at TEXT NOT NULL DEFAULT '',
    thumb_rel TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

INSERT INTO files_audio (id, owner_id, blob_id, name, media_kind, mime, visibility, taken_at, thumb_rel, created_at)
SELECT id, owner_id, blob_id, name, media_kind, mime, visibility, taken_at, thumb_rel, created_at FROM files;

DROP TABLE files;
ALTER TABLE files_audio RENAME TO files;

CREATE INDEX files_owner_idx ON files (owner_id, id DESC);
CREATE INDEX files_blob_idx ON files (blob_id);

PRAGMA foreign_keys=ON;

-- +goose Down
PRAGMA foreign_keys=OFF;

CREATE TABLE files_legacy (
    id INTEGER PRIMARY KEY,
    owner_id INTEGER NOT NULL REFERENCES users(id),
    blob_id INTEGER NOT NULL REFERENCES blobs(id),
    name TEXT NOT NULL,
    media_kind TEXT NOT NULL CHECK (media_kind IN ('image', 'video', 'other')),
    mime TEXT NOT NULL,
    visibility TEXT NOT NULL DEFAULT 'private' CHECK (visibility IN ('private', 'shared')),
    taken_at TEXT NOT NULL DEFAULT '',
    thumb_rel TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

INSERT INTO files_legacy (id, owner_id, blob_id, name, media_kind, mime, visibility, taken_at, thumb_rel, created_at)
SELECT id, owner_id, blob_id, name,
       CASE WHEN media_kind = 'audio' THEN 'other' ELSE media_kind END,
       mime, visibility, taken_at, thumb_rel, created_at
FROM files;

DROP TABLE files;
ALTER TABLE files_legacy RENAME TO files;

CREATE INDEX files_owner_idx ON files (owner_id, id DESC);
CREATE INDEX files_blob_idx ON files (blob_id);

PRAGMA foreign_keys=ON;
