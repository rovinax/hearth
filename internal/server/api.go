package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"hearth/internal/auth"
	"hearth/internal/store"
)

type userJSON struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	Role        string `json:"role"`
	Disabled    bool   `json:"disabled"`
	CreatedAt   string `json:"createdAt"`
	DisplayName string `json:"displayName"`
	Bio         string `json:"bio"`
	Theme       string `json:"theme"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
}

type fileJSON struct {
	ID             int64       `json:"id"`
	Name           string      `json:"name"`
	MediaKind      string      `json:"mediaKind"`
	Mime           string      `json:"mime"`
	Visibility     string      `json:"visibility"`
	Size           int64       `json:"size"`
	OwnerID        int64       `json:"ownerId"`
	OwnerName      string      `json:"ownerName"`
	TakenAt        string      `json:"takenAt"`
	HasThumb       bool        `json:"hasThumb"`
	CreatedAt      string      `json:"createdAt"`
	DeletedAt      string      `json:"deletedAt,omitempty"`
	ContentURL     string      `json:"contentUrl"`
	ThumbURL       string      `json:"thumbUrl"`
	CanEdit        bool        `json:"canEdit"`
	CollectionID   *int64      `json:"collectionId,omitempty"`
	CollectionName string      `json:"collectionName,omitempty"`
	Favorited      bool        `json:"favorited"`
	Shares         []shareJSON `json:"shares,omitempty"`
}

type shareJSON struct {
	ID        int64  `json:"id"`
	ExpiresAt string `json:"expiresAt"`
	CreatedAt string `json:"createdAt"`
}

func userOut(user store.User) userJSON {
	out := userJSON{
		ID: user.ID, Username: user.Username, Role: user.Role,
		Disabled: user.Disabled, CreatedAt: user.CreatedAt,
		DisplayName: user.DisplayName, Bio: user.Bio, Theme: user.Theme,
	}
	if user.AvatarRel != "" {
		out.AvatarURL = "/api/users/" + strconv.FormatInt(user.ID, 10) + "/avatar"
	}
	return out
}

func fileOut(file store.File, actor store.User) fileJSON {
	out := fileJSON{
		ID: file.ID, Name: file.Name, MediaKind: file.MediaKind, Mime: file.Mime,
		Visibility: file.Visibility, Size: file.Size, OwnerID: file.OwnerID,
		OwnerName: file.OwnerName, TakenAt: file.TakenAt, HasThumb: file.ThumbRel != "",
		CreatedAt:      file.CreatedAt,
		DeletedAt:      file.DeletedAt,
		ContentURL:     "/api/files/" + strconv.FormatInt(file.ID, 10) + "/content",
		CanEdit:        actor.Role == store.RoleAdmin || actor.ID == file.OwnerID,
		CollectionName: file.CollectionName,
		Favorited:      file.Favorited,
	}
	if file.CollectionID != 0 {
		id := file.CollectionID
		out.CollectionID = &id
	}
	if out.HasThumb {
		out.ThumbURL = "/api/files/" + strconv.FormatInt(file.ID, 10) + "/thumb"
	}
	return out
}

func filesOut(files []store.File, actor store.User) []fileJSON {
	out := make([]fileJSON, 0, len(files))
	for _, file := range files {
		out = append(out, fileOut(file, actor))
	}
	return out
}

func shareOut(share store.Share) shareJSON {
	return shareJSON{ID: share.ID, ExpiresAt: share.ExpiresAt, CreatedAt: share.CreatedAt}
}

func readJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func (s *Server) apiLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	user, hash, err := s.store.UserByUsername(r.Context(), body.Username)
	if err != nil || user.Disabled || !auth.VerifyPassword(hash, body.Password) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	csrf, err := s.setSession(w, r, user)
	if err != nil {
		s.log.Error("create session", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": userOut(user), "csrf": csrf})
}

func (s *Server) apiLogout(w http.ResponseWriter, r *http.Request) {
	_, csrf, ok := s.requireJSON(w, r)
	if !ok {
		return
	}
	if rejectJSONCSRF(w, r, csrf) {
		return
	}
	s.clearSession(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiMe(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": userOut(user), "csrf": csrf})
}

func (s *Server) apiFiles(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.requireJSON(w, r)
	if !ok {
		return
	}
	kind := cleanKind(r.URL.Query().Get("kind"))
	sortKey, order := r.URL.Query().Get("sort"), r.URL.Query().Get("order")
	if r.URL.Query().Get("trash") == "1" {
		files, err := s.store.ListTrash(r.Context(), user, kind, sortKey, order)
		if err != nil {
			writeStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"files": filesOut(files, user)})
		return
	}
	if raw := r.URL.Query().Get("collection"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
			return
		}
		files, err := s.store.ListCollectionFiles(r.Context(), user, id, kind, sortKey, order)
		if err != nil {
			writeStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"files": filesOut(files, user)})
		return
	}
	files, err := s.store.ListHomeFiles(r.Context(), user, kind, sortKey, order)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": filesOut(files, user)})
}

func (s *Server) apiUpload(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok {
		return
	}
	file, err := s.saveUpload(r, user, csrf)
	if err != nil {
		status, code := http.StatusBadRequest, "invalid"
		switch {
		case errors.Is(err, errTooLarge):
			status, code = http.StatusRequestEntityTooLarge, "too_large"
		case errors.Is(err, errCSRF):
			status, code = http.StatusForbidden, "csrf"
		case errors.Is(err, errNoFile):
			code = "no_file"
		case errors.Is(err, store.ErrInvalid):
			status, code = http.StatusBadRequest, "invalid"
		case errors.Is(err, store.ErrForbidden):
			status, code = http.StatusForbidden, "forbidden"
		case errors.Is(err, store.ErrNotFound):
			status, code = http.StatusNotFound, "not_found"
		default:
			s.log.Error("upload", "err", err)
			status, code = http.StatusInternalServerError, "internal"
		}
		writeJSON(w, status, map[string]string{"error": code})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"file": fileOut(file, user)})
}

func (s *Server) apiFile(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.requireJSON(w, r)
	if !ok {
		return
	}
	file, ok := s.visibleFile(w, r, user)
	if !ok {
		return
	}
	out := fileOut(file, user)
	if out.CanEdit {
		shares, err := s.store.ListShares(r.Context(), file.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
			return
		}
		out.Shares = make([]shareJSON, 0, len(shares))
		for _, share := range shares {
			out.Shares = append(out.Shares, shareOut(share))
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) apiVisibility(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	var body struct {
		Visibility string `json:"visibility"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	if err := s.store.SetVisibility(r.Context(), id, user, body.Visibility); err != nil {
		writeStoreErr(w, err)
		return
	}
	file, err := s.store.GetFile(r.Context(), id)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, fileOut(file, user))
}

func (s *Server) apiRestoreFile(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	file, err := s.store.RestoreFile(r.Context(), id, user)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, fileOut(file, user))
}

func (s *Server) apiPurgeFile(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	removed, err := s.store.PurgeFile(r.Context(), id, user)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	s.dropRemoved([]store.RemovedFile{removed})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiDeleteFile(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	s.removeOwnedFile(w, r, user)
}

func (s *Server) apiAdminDeleteFile(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireAdminJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	s.removeOwnedFile(w, r, user)
}

func (s *Server) removeOwnedFile(w http.ResponseWriter, r *http.Request, user store.User) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	rel, thumb, err := s.store.DeleteFile(r.Context(), id, user)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	s.removeStored(s.cfg.OriginalsDir(), rel, blobDiskPath)
	s.removeStored(s.cfg.CacheDir(), thumb, thumbDiskPath)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiCreateShare(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	file, ok := s.editableFile(w, r, user)
	if !ok {
		return
	}
	if file.DeletedAt != "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	var body struct {
		Expires string `json:"expires"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	var expires time.Time
	switch body.Expires {
	case "", "never":
	case "7d":
		expires = time.Now().Add(7 * 24 * time.Hour)
	case "30d":
		expires = time.Now().Add(30 * 24 * time.Hour)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	raw, err := auth.NewToken()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	id, err := s.store.CreateShare(r.Context(), file.ID, user.ID, raw, expires)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	expiresAt := ""
	if !expires.IsZero() {
		expiresAt = expires.UTC().Format(time.RFC3339)
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":        id,
		"token":     raw,
		"expiresAt": expiresAt,
		"url":       s.absolute(r, "/s/"+raw),
	})
}

func (s *Server) apiDeleteShare(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	file, ok := s.editableFile(w, r, user)
	if !ok {
		return
	}
	shareID, err := strconv.ParseInt(r.PathValue("shareID"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	if err := s.store.DeleteShare(r.Context(), shareID, file.ID); err != nil {
		writeStoreErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiPassword(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	var body struct {
		Current  string `json:"current"`
		Password string `json:"password"`
		Confirm  string `json:"confirm"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	if body.Password != body.Confirm {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "confirm"})
		return
	}
	if auth.ValidatePassword(body.Password) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	_, hash, err := s.store.UserByUsername(r.Context(), user.Username)
	if err != nil || !auth.VerifyPassword(hash, body.Current) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password"})
		return
	}
	encoded, err := auth.HashPassword(body.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	if err := s.store.SetPassword(r.Context(), user.ID, encoded); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiUsers(w http.ResponseWriter, r *http.Request) {
	_, _, ok := s.requireAdminJSON(w, r)
	if !ok {
		return
	}
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	out := make([]userJSON, 0, len(users))
	for _, user := range users {
		out = append(out, userOut(user))
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out})
}

func (s *Server) apiCreateUser(w http.ResponseWriter, r *http.Request) {
	_, csrf, ok := s.requireAdminJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	if body.Role == "" {
		body.Role = store.RoleUser
	}
	if auth.ValidatePassword(body.Password) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	user, err := s.store.CreateUser(r.Context(), body.Username, hash, body.Role)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, userOut(user))
}

func (s *Server) apiDisabled(w http.ResponseWriter, r *http.Request) {
	_, csrf, ok := s.requireAdminJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	var body struct {
		Disabled bool `json:"disabled"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	if err := s.store.SetDisabled(r.Context(), id, body.Disabled); err != nil {
		writeStoreErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiResetPassword(w http.ResponseWriter, r *http.Request) {
	_, csrf, ok := s.requireAdminJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := readJSON(r, &body); err != nil || auth.ValidatePassword(body.Password) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	if err := s.store.SetPassword(r.Context(), id, hash); err != nil {
		writeStoreErr(w, err)
		return
	}
	if err := s.store.DeleteUserSessions(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) apiAdminFiles(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.requireAdminJSON(w, r)
	if !ok {
		return
	}
	files, err := s.store.ListAdminFiles(r.Context(), user, cleanKind(r.URL.Query().Get("kind")), r.URL.Query().Get("user"), r.URL.Query().Get("sort"), r.URL.Query().Get("order"))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	usage, err := s.store.Usage(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"files": filesOut(files, user),
		"usage": usage,
	})
}

func (s *Server) apiShare(w http.ResponseWriter, r *http.Request) {
	file, ok := s.sharedFile(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":       file.Name,
		"mediaKind":  file.MediaKind,
		"mime":       file.Mime,
		"size":       file.Size,
		"contentUrl": "/api/shares/" + r.PathValue("token") + "/content",
	})
}

func writeStoreErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
	case errors.Is(err, store.ErrForbidden):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
	case errors.Is(err, store.ErrConflict):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "conflict"})
	case errors.Is(err, store.ErrLastAdmin):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "last_admin"})
	case errors.Is(err, store.ErrInvalid):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
	}
}
