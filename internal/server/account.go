package server

import (
	"bytes"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

var avatarRelRE = regexp.MustCompile(`^avatars/[1-9][0-9]*\.(jpg|png|gif)$`)

func (s *Server) apiProfile(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	var body struct {
		DisplayName *string `json:"displayName"`
		Bio         *string `json:"bio"`
		Theme       *string `json:"theme"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	name, bio, theme := user.DisplayName, user.Bio, user.Theme
	if body.DisplayName != nil {
		name = *body.DisplayName
	}
	if body.Bio != nil {
		bio = *body.Bio
	}
	if body.Theme != nil {
		theme = *body.Theme
	}
	next, err := s.store.UpdateProfile(r.Context(), user.ID, name, bio, theme)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": userOut(next)})
}

func (s *Server) apiUploadAvatar(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
	if err := r.ParseMultipartForm(4 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no_file"})
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 4<<20+1))
	if err != nil || len(data) == 0 || len(data) > 4<<20 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "too_large"})
		return
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 4096 || cfg.Height > 4096 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	ext := map[string]string{"jpeg": "jpg", "png": "png", "gif": "gif"}[format]
	if ext == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid"})
		return
	}
	rel := "avatars/" + strconv.FormatInt(user.ID, 10) + "." + ext
	dir := filepath.Join(s.cfg.CacheDir(), "avatars")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	path := filepath.Join(dir, strconv.FormatInt(user.ID, 10)+"."+ext)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	next, err := s.store.SetAvatar(r.Context(), user.ID, rel)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if user.AvatarRel != "" && user.AvatarRel != rel {
		if old, err := avatarDiskPath(s.cfg.CacheDir(), user.AvatarRel); err == nil {
			_ = os.Remove(old)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": userOut(next)})
}

func (s *Server) apiDeleteAvatar(w http.ResponseWriter, r *http.Request) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok || rejectJSONCSRF(w, r, csrf) {
		return
	}
	if user.AvatarRel != "" {
		if path, err := avatarDiskPath(s.cfg.CacheDir(), user.AvatarRel); err == nil {
			_ = os.Remove(path)
		}
	}
	next, err := s.store.SetAvatar(r.Context(), user.ID, "")
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": userOut(next)})
}

func (s *Server) apiUserAvatar(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.requireJSON(w, r); !ok {
		return
	}
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	user, err := s.store.UserByID(r.Context(), id)
	if err != nil || user.AvatarRel == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	path, err := avatarDiskPath(s.cfg.CacheDir(), user.AvatarRel)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeFile(w, r, path)
}

func avatarDiskPath(root, rel string) (string, error) {
	if !avatarRelRE.MatchString(rel) {
		return "", os.ErrInvalid
	}
	return filepath.Join(root, filepath.FromSlash(rel)), nil
}
