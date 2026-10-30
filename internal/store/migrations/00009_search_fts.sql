-- +goose Up

CREATE VIRTUAL TABLE files_fts USING fts5(
    name,
    collection_name,
    tokenize = 'unicode61 remove_diacritics 2'
);

CREATE VIRTUAL TABLE collections_fts USING fts5(
    name,
    tokenize = 'unicode61 remove_diacritics 2'
);

INSERT INTO files_fts(rowid, name, collection_name)
SELECT f.id, f.name, COALESCE(c.name, '')
FROM files f
LEFT JOIN collections c ON c.id = f.collection_id;

INSERT INTO collections_fts(rowid, name)
SELECT id, name FROM collections;

-- +goose StatementBegin
CREATE TRIGGER files_fts_ai AFTER INSERT ON files BEGIN
    INSERT INTO files_fts(rowid, name, collection_name)
    SELECT new.id, new.name, COALESCE((SELECT name FROM collections WHERE id = new.collection_id), '');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER files_fts_ad AFTER DELETE ON files BEGIN
    DELETE FROM files_fts WHERE rowid = old.id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER files_fts_au AFTER UPDATE OF name, collection_id ON files BEGIN
    DELETE FROM files_fts WHERE rowid = old.id;
    INSERT INTO files_fts(rowid, name, collection_name)
    SELECT new.id, new.name, COALESCE((SELECT name FROM collections WHERE id = new.collection_id), '');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER collections_fts_ai AFTER INSERT ON collections BEGIN
    INSERT INTO collections_fts(rowid, name) VALUES (new.id, new.name);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER collections_fts_ad AFTER DELETE ON collections BEGIN
    DELETE FROM collections_fts WHERE rowid = old.id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER collections_fts_au AFTER UPDATE OF name ON collections BEGIN
    DELETE FROM collections_fts WHERE rowid = old.id;
    INSERT INTO collections_fts(rowid, name) VALUES (new.id, new.name);
    DELETE FROM files_fts WHERE rowid IN (SELECT id FROM files WHERE collection_id = new.id);
    INSERT INTO files_fts(rowid, name, collection_name)
    SELECT f.id, f.name, new.name FROM files f WHERE f.collection_id = new.id;
END;
-- +goose StatementEnd

-- +goose Down

DROP TRIGGER IF EXISTS collections_fts_au;
DROP TRIGGER IF EXISTS collections_fts_ad;
DROP TRIGGER IF EXISTS collections_fts_ai;
DROP TRIGGER IF EXISTS files_fts_au;
DROP TRIGGER IF EXISTS files_fts_ad;
DROP TRIGGER IF EXISTS files_fts_ai;
DROP TABLE IF EXISTS collections_fts;
DROP TABLE IF EXISTS files_fts;
