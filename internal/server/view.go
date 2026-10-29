package server

import (
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"

	"hearth/internal/store"
)

func (s *Server) thumb(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.requireJSON(w, r)
	if !ok {
		return
	}
	file, ok := s.visibleFile(w, r, user)
	if !ok {
		return
	}
	if file.ThumbRel == "" {
		http.NotFound(w, r)
		return
	}
	path, err := thumbDiskPath(s.cfg.CacheDir(), file.ThumbRel)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	mimeType := "image/jpeg"
	name := "thumb.jpg"
	if strings.HasSuffix(strings.ToLower(file.ThumbRel), ".png") {
		mimeType = "image/png"
		name = "thumb.png"
	}
	s.servePath(w, r, path, mimeType, name, store.KindImage)
}

func (s *Server) shareFile(w http.ResponseWriter, r *http.Request) {
	file, ok := s.sharedFile(w, r)
	if !ok {
		return
	}
	path, err := blobDiskPath(s.cfg.OriginalsDir(), file.RelPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.servePath(w, r, path, file.Mime, file.Name, file.MediaKind)
}

func (s *Server) visibleFile(w http.ResponseWriter, r *http.Request, user store.User) (store.File, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return store.File{}, false
	}
	file, err := s.store.OpenFile(r.Context(), id, user)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return store.File{}, false
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return store.File{}, false
	}
	return file, true
}

func (s *Server) editableFile(w http.ResponseWriter, r *http.Request, user store.User) (store.File, bool) {
	file, ok := s.visibleFile(w, r, user)
	if !ok {
		return store.File{}, false
	}
	if user.Role != store.RoleAdmin && user.ID != file.OwnerID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return store.File{}, false
	}
	return file, true
}

func (s *Server) sharedFile(w http.ResponseWriter, r *http.Request) (store.File, bool) {
	file, err := s.store.FileByShareToken(r.Context(), r.PathValue("token"))
	if err != nil {
		http.NotFound(w, r)
		return store.File{}, false
	}
	return file, true
}

func (s *Server) servePath(w http.ResponseWriter, r *http.Request, path, mimeType, name, kind string) {
	f, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "文件不可用", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", contentDisposition(kind, name))
	http.ServeContent(w, r, name, stat.ModTime(), f)
}

func (s *Server) absolute(r *http.Request, path string) string {
	scheme := "http"
	if r.TLS != nil || s.cfg.Auth.CookieSecure {
		scheme = "https"
	}
	return scheme + "://" + r.Host + path
}
