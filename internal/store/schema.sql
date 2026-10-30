CREATE TABLE jobs (
    id INTEGER PRIMARY KEY,
    kind TEXT NOT NULL,
    payload TEXT NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 5,
    last_error TEXT NOT NULL DEFAULT '',
    run_after TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX jobs_status_run_after_idx ON jobs (status, run_after);

CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin', 'user')),
    disabled INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    display_name TEXT NOT NULL DEFAULT '',
    bio TEXT NOT NULL DEFAULT '',
    theme TEXT NOT NULL DEFAULT '',
    avatar_rel TEXT NOT NULL DEFAULT ''
);

CREATE TABLE sessions (
    token_hash TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id),
    csrf_token TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX sessions_user_idx ON sessions (user_id);

CREATE TABLE blobs (
    id INTEGER PRIMARY KEY,
    sha256 TEXT NOT NULL UNIQUE,
    size INTEGER NOT NULL,
    rel_path TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE collections (
    id INTEGER PRIMARY KEY,
    owner_id INTEGER NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    media_kind TEXT NOT NULL CHECK (media_kind IN ('image', 'video', 'audio', 'other')),
    media_kinds TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX collections_owner_idx ON collections (owner_id, id DESC);

CREATE TABLE files (
    id INTEGER PRIMARY KEY,
    owner_id INTEGER NOT NULL REFERENCES users(id),
    blob_id INTEGER NOT NULL REFERENCES blobs(id),
    name TEXT NOT NULL,
    media_kind TEXT NOT NULL CHECK (media_kind IN ('image', 'video', 'audio', 'other')),
    mime TEXT NOT NULL,
    visibility TEXT NOT NULL DEFAULT 'private' CHECK (visibility IN ('private', 'shared')),
    taken_at TEXT NOT NULL DEFAULT '',
    thumb_rel TEXT NOT NULL DEFAULT '',
    collection_id INTEGER REFERENCES collections(id) ON DELETE SET NULL,
    deleted_at TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX files_owner_idx ON files (owner_id, id DESC);
CREATE INDEX files_blob_idx ON files (blob_id);
CREATE INDEX files_collection_idx ON files (collection_id);
CREATE INDEX files_deleted_idx ON files (deleted_at);

CREATE TABLE shares (
    id INTEGER PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    file_id INTEGER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    created_by INTEGER NOT NULL REFERENCES users(id),
    expires_at TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX shares_file_idx ON shares (file_id);

CREATE TABLE groups (
    id INTEGER PRIMARY KEY,
    owner_id INTEGER NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE group_members (
    group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, user_id)
);

CREATE INDEX group_members_user_idx ON group_members (user_id);

CREATE TABLE collection_users (
    collection_id INTEGER NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (collection_id, user_id)
);

CREATE TABLE collection_groups (
    collection_id INTEGER NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    PRIMARY KEY (collection_id, group_id)
);

CREATE TABLE favorites (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    file_id INTEGER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    PRIMARY KEY (user_id, file_id)
);

CREATE INDEX favorites_user_idx ON favorites (user_id, created_at DESC);

CREATE TABLE search_history (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    query TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE UNIQUE INDEX search_history_user_query_idx ON search_history (user_id, query);

CREATE VIRTUAL TABLE files_fts USING fts5(
    name,
    collection_name,
    tokenize = 'unicode61 remove_diacritics 2'
);

CREATE VIRTUAL TABLE collections_fts USING fts5(
    name,
    tokenize = 'unicode61 remove_diacritics 2'
);
