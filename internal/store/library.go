package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

type Collection struct {
	ID        int64
	OwnerID   int64
	OwnerName string
	Name      string
	MediaKind string
	CreatedAt string
	Users     []User
	Groups    []GroupRef
}

type Group struct {
	ID        int64
	OwnerID   int64
	OwnerName string
	Name      string
	CreatedAt string
	Members   []User
}

type GroupRef struct {
	ID   int64
	Name string
}

type SearchEntry struct {
	ID        int64
	Query     string
	CreatedAt string
}

type rowQuery interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (s *Store) CanOpen(ctx context.Context, actor User, file File) (bool, error) {
	if actor.Role == RoleAdmin || actor.ID == file.OwnerID || file.Visibility == VisShared {
		return true, nil
	}
	if file.CollectionID == 0 {
		return false, nil
	}
	return isMember(ctx, s.db, file.CollectionID, actor.ID)
}

func (s *Store) OpenFile(ctx context.Context, id int64, actor User) (File, error) {
	file, err := s.GetFile(ctx, id)
	if err != nil {
		return File{}, err
	}
	if file.DeletedAt != "" {
		if !canDelete(actor, file.OwnerID) {
			return File{}, ErrNotFound
		}
		files := []File{file}
		if err := s.MarkFavorites(ctx, actor.ID, files); err != nil {
			return File{}, err
		}
		return files[0], nil
	}
	open, err := s.CanOpen(ctx, actor, file)
	if err != nil {
		return File{}, err
	}
	if !open {
		return File{}, ErrNotFound
	}
	files := []File{file}
	if err := s.MarkFavorites(ctx, actor.ID, files); err != nil {
		return File{}, err
	}
	return files[0], nil
}

func (s *Store) MarkFavorites(ctx context.Context, userID int64, files []File) error {
	if len(files) == 0 {
		return nil
	}
	args := make([]any, 0, len(files)+1)
	args = append(args, userID)
	holders := make([]string, len(files))
	for i, file := range files {
		holders[i] = "?"
		args = append(args, file.ID)
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT file_id FROM favorites
WHERE user_id = ? AND file_id IN (`+strings.Join(holders, ",")+`)`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	found := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		found[id] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range files {
		files[i].Favorited = found[files[i].ID]
	}
	return nil
}

func (s *Store) CreateCollection(ctx context.Context, owner User, name, kind string) (Collection, error) {
	name, err := cleanLabel(name, 40)
	if err != nil {
		return Collection{}, err
	}
	if !validKind(kind) {
		return Collection{}, fmt.Errorf("%w: media kind", ErrInvalid)
	}
	var id int64
	err = s.db.QueryRowContext(ctx, `
INSERT INTO collections (owner_id, name, media_kind)
VALUES (?, ?, ?)
RETURNING id`, owner.ID, name, kind).Scan(&id)
	if err != nil {
		return Collection{}, err
	}
	return s.GetCollection(ctx, id, owner)
}

func (s *Store) ListCollections(ctx context.Context, viewer User, kind string) ([]Collection, error) {
	if kind != "" && !validKind(kind) {
		return nil, fmt.Errorf("%w: media kind", ErrInvalid)
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT c.id, c.owner_id, u.username, c.name, c.media_kind, c.created_at
FROM collections c
JOIN users u ON u.id = c.owner_id
WHERE `+memberWhere("c")+`
  AND (? = '' OR c.media_kind = ?)
ORDER BY c.id DESC`, viewer.ID, viewer.ID, viewer.ID, kind, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Collection
	for rows.Next() {
		item, err := scanCollection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetCollection(ctx context.Context, id int64, viewer User) (Collection, error) {
	member, err := isMember(ctx, s.db, id, viewer.ID)
	if err != nil {
		return Collection{}, err
	}
	if !member {
		return Collection{}, ErrNotFound
	}
	row := s.db.QueryRowContext(ctx, `
SELECT c.id, c.owner_id, u.username, c.name, c.media_kind, c.created_at
FROM collections c
JOIN users u ON u.id = c.owner_id
WHERE c.id = ?`, id)
	item, err := scanCollection(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Collection{}, ErrNotFound
	}
	if err != nil {
		return Collection{}, err
	}
	item.Users, err = s.collectionUsers(ctx, id)
	if err != nil {
		return Collection{}, err
	}
	item.Groups, err = s.collectionGroups(ctx, id)
	if err != nil {
		return Collection{}, err
	}
	return item, nil
}

func (s *Store) RenameCollection(ctx context.Context, id int64, actor User, name string) (Collection, error) {
	name, err := cleanLabel(name, 40)
	if err != nil {
		return Collection{}, err
	}
	member, err := isMember(ctx, s.db, id, actor.ID)
	if err != nil {
		return Collection{}, err
	}
	if !member {
		return Collection{}, ErrNotFound
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE collections SET name = ? WHERE id = ?`, name, id); err != nil {
		return Collection{}, err
	}
	return s.GetCollection(ctx, id, actor)
}

func (s *Store) DeleteCollection(ctx context.Context, id int64, actor User) error {
	owner, member, err := s.collectionRole(ctx, id, actor.ID)
	if err != nil {
		return err
	}
	if !member {
		return ErrNotFound
	}
	if !owner {
		return ErrForbidden
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM collections WHERE id = ?`, id)
	return err
}

func (s *Store) ShareCollectionUser(ctx context.Context, id int64, actor User, username string) (Collection, error) {
	member, err := isMember(ctx, s.db, id, actor.ID)
	if err != nil {
		return Collection{}, err
	}
	if !member {
		return Collection{}, ErrNotFound
	}
	user, _, err := s.UserByUsername(ctx, username)
	if err != nil {
		return Collection{}, err
	}
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO collection_users (collection_id, user_id) VALUES (?, ?)
ON CONFLICT DO NOTHING`, id, user.ID); err != nil {
		return Collection{}, err
	}
	return s.GetCollection(ctx, id, actor)
}

func (s *Store) UnshareCollectionUser(ctx context.Context, id, userID int64, actor User) (Collection, error) {
	member, err := isMember(ctx, s.db, id, actor.ID)
	if err != nil {
		return Collection{}, err
	}
	if !member {
		return Collection{}, ErrNotFound
	}
	if _, err := s.db.ExecContext(ctx, `
DELETE FROM collection_users WHERE collection_id = ? AND user_id = ?`, id, userID); err != nil {
		return Collection{}, err
	}
	return s.GetCollection(ctx, id, actor)
}

func (s *Store) ShareCollectionGroup(ctx context.Context, id, groupID int64, actor User) (Collection, error) {
	member, err := isMember(ctx, s.db, id, actor.ID)
	if err != nil {
		return Collection{}, err
	}
	if !member {
		return Collection{}, ErrNotFound
	}
	inGroup, err := isGroupMember(ctx, s.db, groupID, actor.ID)
	if err != nil {
		return Collection{}, err
	}
	if !inGroup {
		exists, err := s.groupExists(ctx, groupID)
		if err != nil {
			return Collection{}, err
		}
		if !exists {
			return Collection{}, ErrNotFound
		}
		return Collection{}, ErrForbidden
	}
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO collection_groups (collection_id, group_id) VALUES (?, ?)
ON CONFLICT DO NOTHING`, id, groupID); err != nil {
		return Collection{}, err
	}
	return s.GetCollection(ctx, id, actor)
}

func (s *Store) UnshareCollectionGroup(ctx context.Context, id, groupID int64, actor User) (Collection, error) {
	member, err := isMember(ctx, s.db, id, actor.ID)
	if err != nil {
		return Collection{}, err
	}
	if !member {
		return Collection{}, ErrNotFound
	}
	if _, err := s.db.ExecContext(ctx, `
DELETE FROM collection_groups WHERE collection_id = ? AND group_id = ?`, id, groupID); err != nil {
		return Collection{}, err
	}
	return s.GetCollection(ctx, id, actor)
}

func (s *Store) ListCollectionFiles(ctx context.Context, viewer User, id int64, kind, sortKey, order string) ([]File, error) {
	member, err := isMember(ctx, s.db, id, viewer.ID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, ErrNotFound
	}
	orderBy, err := fileOrder(sortKey, order, "f.created_at")
	if err != nil {
		return nil, err
	}
	return s.queryFiles(ctx, viewer, fileSelect+`
WHERE f.collection_id = ?
  AND f.deleted_at = ''
  AND (? = '' OR f.media_kind = ?)
ORDER BY `+orderBy, id, kind, kind)
}

func (s *Store) SetFileCollection(ctx context.Context, actor User, fileID int64, collectionID int64) (File, error) {
	file, err := s.OpenFile(ctx, fileID, actor)
	if err != nil {
		return File{}, err
	}
	if err := errIfDeleted(file); err != nil {
		return File{}, err
	}
	if collectionID == 0 {
		if file.CollectionID == 0 {
			return file, nil
		}
		member, err := isMember(ctx, s.db, file.CollectionID, actor.ID)
		if err != nil {
			return File{}, err
		}
		if !member {
			return File{}, ErrForbidden
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE files SET collection_id = NULL WHERE id = ?`, fileID); err != nil {
			return File{}, err
		}
		return s.fileForActor(ctx, fileID, actor)
	}
	var kind string
	err = s.db.QueryRowContext(ctx, `SELECT media_kind FROM collections WHERE id = ?`, collectionID).Scan(&kind)
	if errors.Is(err, sql.ErrNoRows) {
		return File{}, ErrNotFound
	}
	if err != nil {
		return File{}, err
	}
	member, err := isMember(ctx, s.db, collectionID, actor.ID)
	if err != nil {
		return File{}, err
	}
	if !member {
		return File{}, ErrForbidden
	}
	if kind != file.MediaKind {
		return File{}, fmt.Errorf("%w: media kind", ErrInvalid)
	}
	if file.CollectionID != 0 && file.CollectionID != collectionID {
		current, err := isMember(ctx, s.db, file.CollectionID, actor.ID)
		if err != nil {
			return File{}, err
		}
		if !current {
			return File{}, ErrForbidden
		}
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE files SET collection_id = ? WHERE id = ?`, collectionID, fileID); err != nil {
		return File{}, err
	}
	return s.fileForActor(ctx, fileID, actor)
}

func (s *Store) fileForActor(ctx context.Context, id int64, actor User) (File, error) {
	file, err := s.GetFile(ctx, id)
	if err != nil {
		return File{}, err
	}
	files := []File{file}
	if err := s.MarkFavorites(ctx, actor.ID, files); err != nil {
		return File{}, err
	}
	return files[0], nil
}

func (s *Store) RenameFile(ctx context.Context, actor User, id int64, name string) (File, error) {
	name, err := cleanLabel(name, 200)
	if err != nil {
		return File{}, err
	}
	file, err := s.OpenFile(ctx, id, actor)
	if err != nil {
		return File{}, err
	}
	if err := errIfDeleted(file); err != nil {
		return File{}, err
	}
	if !canDelete(actor, file.OwnerID) {
		return File{}, ErrForbidden
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE files SET name = ? WHERE id = ?`, name, id); err != nil {
		return File{}, err
	}
	return s.OpenFile(ctx, id, actor)
}

func (s *Store) AddFavorite(ctx context.Context, actor User, fileID int64) error {
	file, err := s.OpenFile(ctx, fileID, actor)
	if err != nil {
		return err
	}
	if err := errIfDeleted(file); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO favorites (user_id, file_id) VALUES (?, ?)
ON CONFLICT DO NOTHING`, actor.ID, fileID)
	return err
}

func (s *Store) RemoveFavorite(ctx context.Context, actor User, fileID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM favorites WHERE user_id = ? AND file_id = ?`, actor.ID, fileID)
	return err
}

func (s *Store) ListFavorites(ctx context.Context, viewer User, kind, sortKey, order string) ([]File, error) {
	orderBy, err := fileOrder(sortKey, order, "fav.created_at")
	if err != nil {
		return nil, err
	}
	return s.queryFiles(ctx, viewer, fileSelect+`
JOIN favorites fav ON fav.file_id = f.id AND fav.user_id = ?
WHERE f.deleted_at = ''
  AND (? = '' OR f.media_kind = ?)
ORDER BY `+orderBy, viewer.ID, kind, kind)
}

func (s *Store) Search(ctx context.Context, viewer User, query, sortKey, order string) ([]File, []Collection, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil, fmt.Errorf("%w: query", ErrInvalid)
	}
	if utf8.RuneCountInString(query) > 200 {
		query = string([]rune(query)[:200])
	}
	orderBy, err := fileOrder(sortKey, order, "f.created_at")
	if err != nil {
		return nil, nil, err
	}
	pattern := likeContains(query)
	admin := 0
	if viewer.Role == RoleAdmin {
		admin = 1
	}
	files, err := s.queryFiles(ctx, viewer, fileSelect+`
WHERE f.deleted_at = ''
  AND f.name LIKE ? ESCAPE '\'
  AND (
    ? = 1 OR f.owner_id = ? OR f.visibility = 'shared'
    OR EXISTS (
      SELECT 1 FROM collections c
      WHERE c.id = f.collection_id AND `+memberWhere("c")+`
    )
  )
ORDER BY `+orderBy, pattern, admin, viewer.ID, viewer.ID, viewer.ID, viewer.ID)
	if err != nil {
		return nil, nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT c.id, c.owner_id, u.username, c.name, c.media_kind, c.created_at
FROM collections c
JOIN users u ON u.id = c.owner_id
WHERE c.name LIKE ? ESCAPE '\'
  AND `+memberWhere("c")+`
ORDER BY c.name COLLATE NOCASE ASC, c.id ASC`, pattern, viewer.ID, viewer.ID, viewer.ID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var collections []Collection
	for rows.Next() {
		item, err := scanCollection(rows)
		if err != nil {
			return nil, nil, err
		}
		collections = append(collections, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if err := s.RememberSearch(ctx, viewer.ID, query); err != nil {
		return nil, nil, err
	}
	return files, collections, nil
}

func (s *Store) RememberSearch(ctx context.Context, userID int64, query string) error {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	if utf8.RuneCountInString(query) > 200 {
		query = string([]rune(query)[:200])
	}
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO search_history (user_id, query)
VALUES (?, ?)
ON CONFLICT(user_id, query) DO UPDATE SET created_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')`, userID, query); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
DELETE FROM search_history
WHERE user_id = ?
  AND id NOT IN (
    SELECT id FROM search_history
    WHERE user_id = ?
    ORDER BY created_at DESC, id DESC
    LIMIT 20
  )`, userID, userID)
	return err
}

func (s *Store) ListSearchHistory(ctx context.Context, userID int64) ([]SearchEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, query, created_at
FROM search_history
WHERE user_id = ?
ORDER BY created_at DESC, id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SearchEntry
	for rows.Next() {
		var item SearchEntry
		if err := rows.Scan(&item.ID, &item.Query, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) DeleteSearchHistory(ctx context.Context, userID, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM search_history WHERE id = ? AND user_id = ?`, id, userID)
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

func (s *Store) ClearSearchHistory(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM search_history WHERE user_id = ?`, userID)
	return err
}

func (s *Store) CreateGroup(ctx context.Context, owner User, name string) (Group, error) {
	name, err := cleanLabel(name, 40)
	if err != nil {
		return Group{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Group{}, err
	}
	defer tx.Rollback()
	var id int64
	if err := tx.QueryRowContext(ctx, `
INSERT INTO groups (owner_id, name) VALUES (?, ?) RETURNING id`, owner.ID, name).Scan(&id); err != nil {
		return Group{}, err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO group_members (group_id, user_id) VALUES (?, ?)`, id, owner.ID); err != nil {
		return Group{}, err
	}
	if err := tx.Commit(); err != nil {
		return Group{}, err
	}
	return s.GetGroup(ctx, id, owner)
}

func (s *Store) ListGroups(ctx context.Context, viewer User) ([]Group, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT g.id, g.owner_id, ou.username, g.name, g.created_at,
       u.id, u.username, u.role, u.disabled, u.created_at
FROM groups g
JOIN users ou ON ou.id = g.owner_id
JOIN group_members mine ON mine.group_id = g.id AND mine.user_id = ?
JOIN group_members gm ON gm.group_id = g.id
JOIN users u ON u.id = gm.user_id
ORDER BY g.id DESC, u.id`, viewer.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Group
	var current *Group
	for rows.Next() {
		var group Group
		var member User
		var disabled int
		if err := rows.Scan(
			&group.ID, &group.OwnerID, &group.OwnerName, &group.Name, &group.CreatedAt,
			&member.ID, &member.Username, &member.Role, &disabled, &member.CreatedAt,
		); err != nil {
			return nil, err
		}
		member.Disabled = disabled != 0
		if current == nil || current.ID != group.ID {
			out = append(out, group)
			current = &out[len(out)-1]
		}
		current.Members = append(current.Members, member)
	}
	return out, rows.Err()
}

func (s *Store) GetGroup(ctx context.Context, id int64, viewer User) (Group, error) {
	groups, err := s.ListGroups(ctx, viewer)
	if err != nil {
		return Group{}, err
	}
	for _, group := range groups {
		if group.ID == id {
			return group, nil
		}
	}
	exists, err := s.groupExists(ctx, id)
	if err != nil {
		return Group{}, err
	}
	if !exists {
		return Group{}, ErrNotFound
	}
	return Group{}, ErrNotFound
}

func (s *Store) RenameGroup(ctx context.Context, id int64, actor User, name string) (Group, error) {
	name, err := cleanLabel(name, 40)
	if err != nil {
		return Group{}, err
	}
	owner, member, err := s.groupRole(ctx, id, actor.ID)
	if err != nil {
		return Group{}, err
	}
	if !member {
		return Group{}, ErrNotFound
	}
	if !owner {
		return Group{}, ErrForbidden
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE groups SET name = ? WHERE id = ?`, name, id); err != nil {
		return Group{}, err
	}
	return s.GetGroup(ctx, id, actor)
}

func (s *Store) DeleteGroup(ctx context.Context, id int64, actor User) error {
	owner, member, err := s.groupRole(ctx, id, actor.ID)
	if err != nil {
		return err
	}
	if !member {
		return ErrNotFound
	}
	if !owner {
		return ErrForbidden
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM groups WHERE id = ?`, id)
	return err
}

func (s *Store) AddGroupMember(ctx context.Context, id int64, actor User, username string) (Group, error) {
	owner, member, err := s.groupRole(ctx, id, actor.ID)
	if err != nil {
		return Group{}, err
	}
	if !member {
		return Group{}, ErrNotFound
	}
	if !owner {
		return Group{}, ErrForbidden
	}
	user, _, err := s.UserByUsername(ctx, username)
	if err != nil {
		return Group{}, err
	}
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO group_members (group_id, user_id) VALUES (?, ?)
ON CONFLICT DO NOTHING`, id, user.ID); err != nil {
		return Group{}, err
	}
	return s.GetGroup(ctx, id, actor)
}

func (s *Store) RemoveGroupMember(ctx context.Context, id, userID int64, actor User) (Group, error) {
	owner, member, err := s.groupRole(ctx, id, actor.ID)
	if err != nil {
		return Group{}, err
	}
	if !member {
		return Group{}, ErrNotFound
	}
	targetOwner, _, err := s.groupRole(ctx, id, userID)
	if err != nil {
		return Group{}, err
	}
	if targetOwner {
		return Group{}, ErrForbidden
	}
	if !owner && actor.ID != userID {
		return Group{}, ErrForbidden
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM group_members WHERE group_id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return Group{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return Group{}, err
	}
	if n == 0 {
		return Group{}, ErrNotFound
	}
	if actor.ID == userID {
		return Group{}, nil
	}
	return s.GetGroup(ctx, id, actor)
}

func (s *Store) collectionUsers(ctx context.Context, id int64) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT u.id, u.username, u.role, u.disabled, u.created_at
FROM collection_users cu
JOIN users u ON u.id = cu.user_id
WHERE cu.collection_id = ?
ORDER BY u.username`, id)
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

func (s *Store) collectionGroups(ctx context.Context, id int64) ([]GroupRef, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT g.id, g.name
FROM collection_groups cg
JOIN groups g ON g.id = cg.group_id
WHERE cg.collection_id = ?
ORDER BY g.name`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GroupRef
	for rows.Next() {
		var item GroupRef
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) collectionRole(ctx context.Context, id, userID int64) (owner bool, member bool, err error) {
	var ownerID int64
	err = s.db.QueryRowContext(ctx, `SELECT owner_id FROM collections WHERE id = ?`, id).Scan(&ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	if ownerID == userID {
		return true, true, nil
	}
	member, err = isMember(ctx, s.db, id, userID)
	return false, member, err
}

func (s *Store) groupRole(ctx context.Context, id, userID int64) (owner bool, member bool, err error) {
	var ownerID int64
	err = s.db.QueryRowContext(ctx, `SELECT owner_id FROM groups WHERE id = ?`, id).Scan(&ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	member, err = isGroupMember(ctx, s.db, id, userID)
	if err != nil {
		return false, false, err
	}
	return ownerID == userID, member, nil
}

func (s *Store) groupExists(ctx context.Context, id int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM groups WHERE id = ?`, id).Scan(&n)
	return n > 0, err
}

func isMember(ctx context.Context, q rowQuery, collectionID, userID int64) (bool, error) {
	var ok int
	err := q.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1 FROM collections c
    WHERE c.id = ? AND `+memberWhere("c")+`
)`, collectionID, userID, userID, userID).Scan(&ok)
	return ok == 1, err
}

func isGroupMember(ctx context.Context, q rowQuery, groupID, userID int64) (bool, error) {
	var ok int
	err := q.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1 FROM group_members WHERE group_id = ? AND user_id = ?
)`, groupID, userID).Scan(&ok)
	return ok == 1, err
}

func memberWhere(alias string) string {
	return `(` + alias + `.owner_id = ? OR EXISTS (
        SELECT 1 FROM collection_users cu
        WHERE cu.collection_id = ` + alias + `.id AND cu.user_id = ?
    ) OR EXISTS (
        SELECT 1 FROM collection_groups cg
        JOIN group_members gm ON gm.group_id = cg.group_id AND gm.user_id = ?
        WHERE cg.collection_id = ` + alias + `.id
    ))`
}

func scanCollection(row scanner) (Collection, error) {
	var item Collection
	err := row.Scan(&item.ID, &item.OwnerID, &item.OwnerName, &item.Name, &item.MediaKind, &item.CreatedAt)
	return item, err
}

func cleanLabel(name string, max int) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > max || strings.ContainsRune(name, 0) {
		return "", fmt.Errorf("%w: name", ErrInvalid)
	}
	return name, nil
}

func fileOrder(sortKey, order, added string) (string, error) {
	var col string
	switch sortKey {
	case "", "added":
		col = added
	case "name":
		col = "f.name COLLATE NOCASE"
	case "size":
		col = "b.size"
	default:
		return "", fmt.Errorf("%w: sort", ErrInvalid)
	}
	dir := "DESC"
	switch order {
	case "", "desc":
	case "asc":
		dir = "ASC"
	default:
		return "", fmt.Errorf("%w: order", ErrInvalid)
	}
	return col + " " + dir + ", f.id " + dir, nil
}

func likeContains(query string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + replacer.Replace(query) + "%"
}
