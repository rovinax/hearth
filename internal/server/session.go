package server

import (
	"crypto/subtle"
	"net/http"
	"time"

	"hearth/internal/auth"
	"hearth/internal/store"
)

const sessionCookie = "hearth_session"

func (s *Server) userFrom(r *http.Request) (store.User, string, bool) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		return store.User{}, "", false
	}
	user, csrf, err := s.store.LookupSession(r.Context(), auth.HashToken(cookie.Value))
	if err != nil {
		return store.User{}, "", false
	}
	return user, csrf, true
}

func (s *Server) requireJSON(w http.ResponseWriter, r *http.Request) (store.User, string, bool) {
	user, csrf, ok := s.userFrom(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return store.User{}, "", false
	}
	return user, csrf, true
}

func (s *Server) requireAdminJSON(w http.ResponseWriter, r *http.Request) (store.User, string, bool) {
	user, csrf, ok := s.requireJSON(w, r)
	if !ok {
		return store.User{}, "", false
	}
	if user.Role != store.RoleAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return store.User{}, "", false
	}
	return user, csrf, true
}

func rejectJSONCSRF(w http.ResponseWriter, r *http.Request, token string) bool {
	if !csrfOK(r.Header.Get("X-CSRF-Token"), token) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "csrf"})
		return true
	}
	return false
}

func (s *Server) requireUser(w http.ResponseWriter, r *http.Request) (store.User, string, bool) {
	user, csrf, ok := s.userFrom(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return store.User{}, "", false
	}
	return user, csrf, true
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) (store.User, string, bool) {
	user, csrf, ok := s.requireUser(w, r)
	if !ok {
		return store.User{}, "", false
	}
	if user.Role != store.RoleAdmin {
		http.Error(w, "没有权限", http.StatusForbidden)
		return store.User{}, "", false
	}
	return user, csrf, true
}

func (s *Server) setSession(w http.ResponseWriter, r *http.Request, user store.User) (string, error) {
	raw, err := auth.NewToken()
	if err != nil {
		return "", err
	}
	csrf, err := auth.NewToken()
	if err != nil {
		return "", err
	}
	expires := time.Now().Add(s.cfg.Auth.SessionTTL.Std())
	if err := s.store.CreateSession(r.Context(), user.ID, auth.HashToken(raw), csrf, expires); err != nil {
		return "", err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    raw,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cfg.Auth.CookieSecure,
		Expires:  expires,
	})
	return csrf, nil
}

func (s *Server) clearSession(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil && cookie.Value != "" {
		_ = s.store.DeleteSession(r.Context(), auth.HashToken(cookie.Value))
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cfg.Auth.CookieSecure,
		MaxAge:   -1,
	})
}

func csrfOK(sent, token string) bool {
	if sent == "" || token == "" || len(sent) != len(token) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(sent), []byte(token)) == 1
}

func (s *Server) rejectCSRF(w http.ResponseWriter, r *http.Request, token string) bool {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "请求无效", http.StatusBadRequest)
		return true
	}
	if !csrfOK(r.FormValue("csrf"), token) {
		http.Error(w, "请求已失效，请刷新页面", http.StatusForbidden)
		return true
	}
	return false
}
