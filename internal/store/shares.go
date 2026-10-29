package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"hearth/internal/auth"
)

type Share struct {
	ID        int64
	FileID    int64
	ExpiresAt string
	CreatedAt string
}

func (s *Store) CreateShare(ctx context.Context, fileID, userID int64, rawToken string, expires time.Time) (int64, error) {
	expiresAt := ""
	if !expires.IsZero() {
		expiresAt = utcStamp(expires)
	}
	res, err := s.db.ExecContext(ctx, `
INSERT INTO shares (token_hash, file_id, created_by, expires_at)
VALUES (?, ?, ?, ?)`, auth.HashToken(rawToken), fileID, userID, expiresAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) ListShares(ctx context.Context, fileID int64) ([]Share, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, file_id, expires_at, created_at
FROM shares
WHERE file_id = ?
ORDER BY id DESC`, fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Share
	for rows.Next() {
		var share Share
		if err := rows.Scan(&share.ID, &share.FileID, &share.ExpiresAt, &share.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, share)
	}
	return out, rows.Err()
}

func (s *Store) DeleteShare(ctx context.Context, id, fileID int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM shares WHERE id = ? AND file_id = ?`, id, fileID)
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

func (s *Store) FileByShareToken(ctx context.Context, rawToken string) (File, error) {
	row := s.db.QueryRowContext(ctx, fileSelect+`
JOIN shares sh ON sh.file_id = f.id
WHERE sh.token_hash = ?
  AND f.deleted_at = ''
  AND (sh.expires_at = '' OR sh.expires_at > ?)`, auth.HashToken(rawToken), utcStamp(time.Now()))
	file, err := scanFile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return File{}, ErrNotFound
	}
	return file, err
}
