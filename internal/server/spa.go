package server

import (
	"net/http"

	"hearth/web"
)

func (s *Server) spa(w http.ResponseWriter, r *http.Request) {
	body, err := web.Dist.ReadFile("dist/index.html")
	if err != nil {
		http.Error(w, "页面不可用", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(body)
}
