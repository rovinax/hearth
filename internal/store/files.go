package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type File struct {
	ID             int64
	OwnerID        int64
	OwnerName      string
	BlobID         int64
	Name           string
	MediaKind      string
	Mime           string
	Visibility     string
	Size           int64
	SHA256         string
	RelPath        string
	TakenAt        string
	ThumbRel       string
	CollectionID   int64
	CollectionName string
	Favorited      bool
	DeletedAt      string
	CreatedAt      string
}

const TrashKeep = 30 * 24 * time.Hour

type RemovedFile struct {
	Rel   string
	Thumb string
}

type NewFile struct {
	OwnerID      int64
	SHA256       string
	Size         int64
	RelPath      string
	Name         string
	MediaKind    string
	Mime         string
	Visibility   string
	TakenAt      string
	CollectionID int64
}

type Usage struct {
	Files     int   `json:"files"`
	BlobBytes int64 `json:"blobBytes"`
}

func (s *Store) AddFile(ctx context.Context, in NewFile) (File, error) {
	if in.Visibility != VisPrivate && in.Visibility != VisShared {
		return File{}, fmt.Errorf("%w: visibility", ErrInvalid)
	}
	if !validKind(in.MediaKind) {
		return File{}, fmt.Errorf("%w: media kind", ErrInvalid)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return File{}, err
	}
	defer tx.Rollback()

	var blobID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM blobs WHERE sha256 = ?`, in.SHA256).Scan(&blobID)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `
INSERT INTO blobs (sha256, size, rel_path)
VALUES (?, ?, ?)
RETURNING id`, in.SHA256, in.Size, in.RelPath).Scan(&blobID)
		if isUnique(err) {
			err = tx.QueryRowContext(ctx, `SELECT id FROM blobs WHERE sha256 = ?`, in.SHA256).Scan(&blobID)
		}
	}
	if err != nil {
		return File{}, err
	}
	var collection any
	if in.CollectionID != 0 {
		var kind string
		err = tx.QueryRowContext(ctx, `SELECT media_kind FROM collections WHERE id = ?`, in.CollectionID).Scan(&kind)
		if errors.Is(err, sql.ErrNoRows) {
			return File{}, ErrNotFound
		}
		if err != nil {
			return File{}, err
		}
		if kind != in.MediaKind {
			return File{}, fmt.Errorf("%w: media kind", ErrInvalid)
		}
		member, err := isMember(ctx, tx, in.CollectionID, in.OwnerID)
		if err != nil {
			return File{}, err
		}
		if !member {
			return File{}, ErrForbidden
		}
		collection = in.CollectionID
	}
	var id int64
	if err := tx.QueryRowContext(ctx, `
INSERT INTO files (owner_id, blob_id, name, media_kind, mime, visibility, taken_at, collection_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id`, in.OwnerID, blobID, in.Name, in.MediaKind, in.Mime, in.Visibility, in.TakenAt, collection).Scan(&id); err != nil {
		return File{}, err
	}
	if err := tx.Commit(); err != nil {
		return File{}, err
	}
	return s.GetFile(ctx, id)
}

func (s *Store) GetFile(ctx context.Context, id int64) (File, error) {
	row := s.db.QueryRowContext(ctx, fileSelect+` WHERE f.id = ?`, id)
	file, err := scanFile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return File{}, ErrNotFound
	}
	return file, err
}

func (s *Store) ListHomeFiles(ctx context.Context, viewer User, kind, sortKey, order string) ([]File, error) {
	orderBy, err := fileOrder(sortKey, order, "f.created_at")
	if err != nil {
		return nil, err
	}
	admin := 0
	if viewer.Role == RoleAdmin {
		admin = 1
	}
	return s.queryFiles(ctx, viewer, fileSelect+`
WHERE f.deleted_at = ''
  AND (? = 1 OR f.owner_id = ? OR f.visibility = 'shared')
  AND NOT (f.owner_id = ? AND f.collection_id IS NOT NULL)
  AND (? = '' OR f.media_kind = ?)
ORDER BY `+orderBy, admin, viewer.ID, viewer.ID, kind, kind)
}

func (s *Store) ListAdminFiles(ctx context.Context, viewer User, kind, owner, sortKey, order string) ([]File, error) {
	orderBy, err := fileOrder(sortKey, order, "f.created_at")
	if err != nil {
		return nil, err
	}
	admin := 0
	if viewer.Role == RoleAdmin {
		admin = 1
	}
	return s.queryFiles(ctx, viewer, fileSelect+`
WHERE f.deleted_at = ''
  AND (? = 1 OR f.owner_id = ? OR f.visibility = 'shared')
  AND (? = '' OR f.media_kind = ?)
  AND (? = '' OR u.username = ?)
ORDER BY `+orderBy, admin, viewer.ID, kind, kind, owner, owner)
}

func (s *Store) DeleteFile(ctx context.Context, id int64, actor User) (string, string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback()

	var ownerID int64
	var visibility, deletedAt string
	var collectionID sql.NullInt64
	err = tx.QueryRowContext(ctx, `
SELECT f.owner_id, f.visibility, f.collection_id, f.deleted_at
FROM files f
WHERE f.id = ?`, id).Scan(&ownerID, &visibility, &collectionID, &deletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrNotFound
	}
	if err != nil {
		return "", "", err
	}
	probe := File{OwnerID: ownerID, Visibility: visibility}
	if collectionID.Valid {
		probe.CollectionID = collectionID.Int64
	}
	open, err := s.CanOpen(ctx, actor, probe)
	if err != nil {
		return "", "", err
	}
	if !open {
		return "", "", ErrNotFound
	}
	if !canDelete(actor, ownerID) {
		return "", "", ErrForbidden
	}
	if deletedAt != "" {
		return "", "", nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE files SET deleted_at = ? WHERE id = ?`, utcStamp(time.Now()), id); err != nil {
		return "", "", err
	}
	if err := tx.Commit(); err != nil {
		return "", "", err
	}
	return "", "", nil
}

func (s *Store) ListTrash(ctx context.Context, viewer User, kind, sortKey, order string) ([]File, error) {
	orderBy, err := fileOrder(sortKey, order, "f.deleted_at")
	if err != nil {
		return nil, err
	}
	admin := 0
	if viewer.Role == RoleAdmin {
		admin = 1
	}
	return s.queryFiles(ctx, viewer, fileSelect+`
WHERE f.deleted_at != ''
  AND (? = 1 OR f.owner_id = ?)
  AND (? = '' OR f.media_kind = ?)
ORDER BY `+orderBy, admin, viewer.ID, kind, kind)
}

func (s *Store) RestoreFile(ctx context.Context, id int64, actor User) (File, error) {
	file, err := s.GetFile(ctx, id)
	if err != nil {
		return File{}, err
	}
	if file.DeletedAt == "" || !canDelete(actor, file.OwnerID) {
		return File{}, ErrNotFound
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE files SET deleted_at = '' WHERE id = ?`, id); err != nil {
		return File{}, err
	}
	return s.fileForActor(ctx, id, actor)
}

func (s *Store) PurgeFile(ctx context.Context, id int64, actor User) (RemovedFile, error) {
	file, err := s.GetFile(ctx, id)
	if err != nil {
		return RemovedFile{}, err
	}
	if file.DeletedAt == "" || !canDelete(actor, file.OwnerID) {
		return RemovedFile{}, ErrNotFound
	}
	return s.purgeID(ctx, id)
}

func (s *Store) PurgeExpired(ctx context.Context, now time.Time) ([]RemovedFile, error) {
	cutoff := utcStamp(now.Add(-TrashKeep))
	rows, err := s.db.QueryContext(ctx, `
SELECT id FROM files WHERE deleted_at != '' AND deleted_at <= ?`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]RemovedFile, 0, len(ids))
	for _, id := range ids {
		removed, err := s.purgeID(ctx, id)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return out, err
		}
		out = append(out, removed)
	}
	return out, nil
}

func (s *Store) purgeID(ctx context.Context, id int64) (RemovedFile, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return RemovedFile{}, err
	}
	defer tx.Rollback()

	var blobID int64
	var rel, thumb string
	err = tx.QueryRowContext(ctx, `
SELECT f.blob_id, b.rel_path, f.thumb_rel
FROM files f
JOIN blobs b ON b.id = f.blob_id
WHERE f.id = ?`, id).Scan(&blobID, &rel, &thumb)
	if errors.Is(err, sql.ErrNoRows) {
		return RemovedFile{}, ErrNotFound
	}
	if err != nil {
		return RemovedFile{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM files WHERE id = ?`, id); err != nil {
		return RemovedFile{}, err
	}
	var left int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM files WHERE blob_id = ?`, blobID).Scan(&left); err != nil {
		return RemovedFile{}, err
	}
	removed := RemovedFile{}
	if left == 0 {
		if _, err := tx.ExecContext(ctx, `DELETE FROM blobs WHERE id = ?`, blobID); err != nil {
			return RemovedFile{}, err
		}
		removed.Rel = rel
	}
	if thumb != "" {
		var thumbs int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM files WHERE thumb_rel = ?`, thumb).Scan(&thumbs); err != nil {
			return RemovedFile{}, err
		}
		if thumbs == 0 {
			removed.Thumb = thumb
		}
	}
	if err := tx.Commit(); err != nil {
		return RemovedFile{}, err
	}
	return removed, nil
}

func (s *Store) SetThumb(ctx context.Context, id int64, thumbRel string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE files SET thumb_rel = ? WHERE id = ?`, thumbRel, id)
	return err
}

func (s *Store) SetVisibility(ctx context.Context, id int64, actor User, visibility string) error {
	if visibility != VisPrivate && visibility != VisShared {
		return fmt.Errorf("%w: visibility", ErrInvalid)
	}
	file, err := s.GetFile(ctx, id)
	if err != nil {
		return err
	}
	open, err := s.CanOpen(ctx, actor, file)
	if err != nil {
		return err
	}
	if !open {
		return ErrNotFound
	}
	if !canDelete(actor, file.OwnerID) {
		return ErrForbidden
	}
	if err := errIfDeleted(file); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE files SET visibility = ? WHERE id = ?`, visibility, id)
	return err
}

func (s *Store) Usage(ctx context.Context) (Usage, error) {
	var usage Usage
	err := s.db.QueryRowContext(ctx, `
SELECT
    (SELECT COUNT(*) FROM files),
    COALESCE((SELECT SUM(size) FROM blobs), 0)`).Scan(&usage.Files, &usage.BlobBytes)
	return usage, err
}

func validKind(kind string) bool {
	return kind == KindImage || kind == KindVideo || kind == KindAudio || kind == KindOther
}

func (s *Store) ListStoredFiles(ctx context.Context) ([]File, error) {
	rows, err := s.db.QueryContext(ctx, fileSelect)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var files []File
	for rows.Next() {
		file, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, rows.Err()
}

func (s *Store) SetMedia(ctx context.Context, id int64, kind, mimeType string) error {
	if !validKind(kind) || mimeType == "" {
		return fmt.Errorf("%w: media", ErrInvalid)
	}
	res, err := s.db.ExecContext(ctx, `UPDATE files SET media_kind = ?, mime = ? WHERE id = ?`, kind, mimeType, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func canDelete(actor User, ownerID int64) bool {
	return actor.Role == RoleAdmin || actor.ID == ownerID
}

func errIfDeleted(file File) error {
	if file.DeletedAt != "" {
		return ErrNotFound
	}
	return nil
}

const fileSelect = `
SELECT f.id, f.owner_id, u.username, f.blob_id, f.name, f.media_kind, f.mime,
       f.visibility, b.size, b.sha256, b.rel_path, f.taken_at, f.thumb_rel, f.created_at,
       f.collection_id, COALESCE(c.name, ''), f.deleted_at
FROM files f
JOIN users u ON u.id = f.owner_id
JOIN blobs b ON b.id = f.blob_id
LEFT JOIN collections c ON c.id = f.collection_id
`

func scanFile(row scanner) (File, error) {
	var file File
	var collectionID sql.NullInt64
	err := row.Scan(
		&file.ID, &file.OwnerID, &file.OwnerName, &file.BlobID, &file.Name, &file.MediaKind,
		&file.Mime, &file.Visibility, &file.Size, &file.SHA256, &file.RelPath, &file.TakenAt,
		&file.ThumbRel, &file.CreatedAt, &collectionID, &file.CollectionName, &file.DeletedAt,
	)
	if err != nil {
		return File{}, err
	}
	if collectionID.Valid {
		file.CollectionID = collectionID.Int64
	}
	return file, nil
}

func (s *Store) queryFiles(ctx context.Context, viewer User, query string, args ...any) ([]File, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []File
	for rows.Next() {
		file, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, file)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.MarkFavorites(ctx, viewer.ID, out); err != nil {
		return nil, err
	}
	return out, nil
}
