package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

func (s *Store) CreateSession(ctx context.Context, userID int64, tokenHash, csrf string, expires time.Time) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO sessions (token_hash, user_id, csrf_token, expires_at)
VALUES (?, ?, ?, ?)`, tokenHash, userID, csrf, utcStamp(expires))
	return err
}

func (s *Store) LookupSession(ctx context.Context, tokenHash string) (User, string, error) {
	var user User
	var disabled int
	var csrf string
	err := s.db.QueryRowContext(ctx, `
SELECT u.id, u.username, u.role, u.disabled, u.created_at, u.display_name, u.bio, u.theme, u.avatar_rel, s.csrf_token
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = ? AND s.expires_at > ?`, tokenHash, utcStamp(time.Now())).Scan(
		&user.ID, &user.Username, &user.Role, &disabled, &user.CreatedAt,
		&user.DisplayName, &user.Bio, &user.Theme, &user.AvatarRel, &csrf,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, "", ErrNotFound
	}
	if err != nil {
		return User{}, "", err
	}
	user.Disabled = disabled != 0
	if user.Disabled {
		return User{}, "", ErrNotFound
	}
	return user, csrf, nil
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (s *Store) DeleteUserSessions(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}
