// Package server 提供 HTTP 入口。业务路由后续加在 routes 里。
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"hearth/internal/config"
	"hearth/internal/media"
	"hearth/internal/store"
	"hearth/web"
)

type Server struct {
	log     *slog.Logger
	store   *store.Store
	cfg     config.Config
	bins    media.Binaries
	version string
}

func New(log *slog.Logger, st *store.Store, cfg config.Config, version string, bins media.Binaries) (*Server, error) {
	return &Server{log: log, store: st, cfg: cfg, bins: bins, version: version}, nil
}

func (s *Server) Run(ctx context.Context) error {
	go s.trashLoop(ctx)
	httpSrv := &http.Server{
		Addr:              s.cfg.HTTPAddr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		s.log.Info("listening", "addr", s.cfg.HTTPAddr)
		errCh <- httpSrv.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutCtx); err != nil {
			return err
		}
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /readyz", s.readyz)
	mux.HandleFunc("GET /api/meta", s.meta)
	mux.HandleFunc("POST /api/login", s.apiLogin)
	mux.HandleFunc("POST /api/logout", s.apiLogout)
	mux.HandleFunc("GET /api/me", s.apiMe)
	mux.HandleFunc("GET /api/files", s.apiFiles)
	mux.HandleFunc("POST /api/files", s.apiUpload)
	mux.HandleFunc("GET /api/files/{id}", s.apiFile)
	mux.HandleFunc("PATCH /api/files/{id}", s.apiRenameFile)
	mux.HandleFunc("POST /api/files/{id}/collection", s.apiSetCollection)
	mux.HandleFunc("POST /api/files/batch", s.apiBatchFiles)
	mux.HandleFunc("POST /api/files/{id}/favorite", s.apiFavorite)
	mux.HandleFunc("DELETE /api/files/{id}/favorite", s.apiUnfavorite)
	mux.HandleFunc("GET /api/favorites", s.apiFavorites)
	mux.HandleFunc("GET /api/collections", s.apiCollections)
	mux.HandleFunc("POST /api/collections", s.apiCreateCollection)
	mux.HandleFunc("GET /api/collections/{id}", s.apiCollection)
	mux.HandleFunc("PATCH /api/collections/{id}", s.apiRenameCollection)
	mux.HandleFunc("DELETE /api/collections/{id}", s.apiDeleteCollection)
	mux.HandleFunc("POST /api/collections/{id}/users", s.apiShareCollectionUser)
	mux.HandleFunc("DELETE /api/collections/{id}/users/{userID}", s.apiUnshareCollectionUser)
	mux.HandleFunc("POST /api/collections/{id}/groups", s.apiShareCollectionGroup)
	mux.HandleFunc("DELETE /api/collections/{id}/groups/{groupID}", s.apiUnshareCollectionGroup)
	mux.HandleFunc("GET /api/groups", s.apiGroups)
	mux.HandleFunc("POST /api/groups", s.apiCreateGroup)
	mux.HandleFunc("PATCH /api/groups/{id}", s.apiRenameGroup)
	mux.HandleFunc("DELETE /api/groups/{id}", s.apiDeleteGroup)
	mux.HandleFunc("POST /api/groups/{id}/members", s.apiAddGroupMember)
	mux.HandleFunc("DELETE /api/groups/{id}/members/{userID}", s.apiRemoveGroupMember)
	mux.HandleFunc("GET /api/search", s.apiSearch)
	mux.HandleFunc("GET /api/search/history", s.apiSearchHistory)
	mux.HandleFunc("DELETE /api/search/history", s.apiClearSearchHistory)
	mux.HandleFunc("DELETE /api/search/history/{id}", s.apiDeleteSearchHistory)
	mux.HandleFunc("GET /api/files/{id}/content", s.download)
	mux.HandleFunc("GET /api/files/{id}/thumb", s.thumb)
	mux.HandleFunc("POST /api/files/{id}/visibility", s.apiVisibility)
	mux.HandleFunc("DELETE /api/files/{id}", s.apiDeleteFile)
	mux.HandleFunc("POST /api/files/{id}/restore", s.apiRestoreFile)
	mux.HandleFunc("DELETE /api/files/{id}/purge", s.apiPurgeFile)
	mux.HandleFunc("POST /api/files/{id}/shares", s.apiCreateShare)
	mux.HandleFunc("DELETE /api/files/{id}/shares/{shareID}", s.apiDeleteShare)
	mux.HandleFunc("POST /api/account/password", s.apiPassword)
	mux.HandleFunc("PATCH /api/account", s.apiProfile)
	mux.HandleFunc("POST /api/account/avatar", s.apiUploadAvatar)
	mux.HandleFunc("DELETE /api/account/avatar", s.apiDeleteAvatar)
	mux.HandleFunc("GET /api/users/{id}/avatar", s.apiUserAvatar)
	mux.HandleFunc("GET /api/admin/users", s.apiUsers)
	mux.HandleFunc("POST /api/admin/users", s.apiCreateUser)
	mux.HandleFunc("POST /api/admin/users/{id}/disabled", s.apiDisabled)
	mux.HandleFunc("POST /api/admin/users/{id}/password", s.apiResetPassword)
	mux.HandleFunc("GET /api/admin/files", s.apiAdminFiles)
	mux.HandleFunc("DELETE /api/admin/files/{id}", s.apiAdminDeleteFile)
	mux.HandleFunc("GET /api/shares/{token}", s.apiShare)
	mux.HandleFunc("GET /api/shares/{token}/content", s.shareFile)
	assets, err := fs.Sub(web.Dist, "dist/assets")
	if err != nil {
		s.log.Error("web assets", "err", err)
	} else {
		mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assets))))
	}
	mux.HandleFunc("GET /{path...}", s.spa)
	mux.HandleFunc("GET /{$}", s.spa)
	return s.logRequests(mux)
}

func (s *Server) meta(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"name":    "hearth",
		"version": s.version,
	})
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		defer func() {
			if rec := recover(); rec != nil {
				s.log.Error("panic", "method", r.Method, "path", r.URL.Path, "err", rec)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"status": "error"})
			}
		}()
		next.ServeHTTP(sw, r)
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			return
		}
		s.log.Info("request", "method", r.Method, "path", r.URL.Path, "status", sw.status, "ms", time.Since(start).Milliseconds())
	})
}
