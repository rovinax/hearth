package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"hearth/internal/auth"
)

var usernameRE = regexp.MustCompile(`^[a-z0-9_-]{3,32}$`)

type User struct {
	ID        int64
	Username  string
	Role      string
	Disabled  bool
	CreatedAt string
}

func NormalizeUsername(username string) (string, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if !usernameRE.MatchString(username) {
		return "", fmt.Errorf("%w: username", ErrInvalid)
	}
	return username, nil
}

func (s *Store) EnsureAdmin(ctx context.Context, username, password string) error {
	n, err := s.countEnabledAdmins(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	if username == "" || password == "" {
		return fmt.Errorf("no enabled admin; set HEARTH_ADMIN_USER and HEARTH_ADMIN_PASSWORD")
	}
	username, err = NormalizeUsername(username)
	if err != nil {
		return err
	}
	if err := auth.ValidatePassword(password); err != nil {
		return err
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	_, err = s.CreateUser(ctx, username, hash, RoleAdmin)
	return err
}

func (s *Store) CreateUser(ctx context.Context, username, passwordHash, role string) (User, error) {
	username, err := NormalizeUsername(username)
	if err != nil {
		return User{}, err
	}
	if role != RoleAdmin && role != RoleUser {
		return User{}, fmt.Errorf("%w: role", ErrInvalid)
	}
	var id int64
	err = s.db.QueryRowContext(ctx, `
INSERT INTO users (username, password_hash, role)
VALUES (?, ?, ?)
RETURNING id`, username, passwordHash, role).Scan(&id)
	if err != nil {
		if isUnique(err) {
			return User{}, ErrConflict
		}
		return User{}, err
	}
	return s.UserByID(ctx, id)
}

func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, username, role, disabled, created_at
FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func (s *Store) UserByUsername(ctx context.Context, username string) (User, string, error) {
	username, err := NormalizeUsername(username)
	if err != nil {
		return User{}, "", err
	}
	var user User
	var disabled int
	var hash string
	err = s.db.QueryRowContext(ctx, `
SELECT id, username, role, disabled, created_at, password_hash
FROM users WHERE username = ?`, username).Scan(
		&user.ID, &user.Username, &user.Role, &disabled, &user.CreatedAt, &hash,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, "", ErrNotFound
	}
	if err != nil {
		return User{}, "", err
	}
	user.Disabled = disabled != 0
	return user, hash, nil
}

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, username, role, disabled, created_at
FROM users
ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, user)
	}
	return out, rows.Err()
}

func (s *Store) SetPassword(ctx context.Context, id int64, passwordHash string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, id)
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

func (s *Store) SetDisabled(ctx context.Context, id int64, disabled bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var role string
	var already int
	err = tx.QueryRowContext(ctx, `SELECT role, disabled FROM users WHERE id = ?`, id).Scan(&role, &already)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if disabled && role == RoleAdmin && already == 0 {
		var n int
		if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*) FROM users WHERE role = 'admin' AND disabled = 0`).Scan(&n); err != nil {
			return err
		}
		if n <= 1 {
			return ErrLastAdmin
		}
	}
	flag := 0
	if disabled {
		flag = 1
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET disabled = ? WHERE id = ?`, flag, id); err != nil {
		return err
	}
	if disabled {
		if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) countEnabledAdmins(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM users WHERE role = 'admin' AND disabled = 0`).Scan(&n)
	return n, err
}

func scanUser(row scanner) (User, error) {
	var user User
	var disabled int
	err := row.Scan(&user.ID, &user.Username, &user.Role, &disabled, &user.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	user.Disabled = disabled != 0
	return user, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func isUnique(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique")
}

func utcStamp(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}
