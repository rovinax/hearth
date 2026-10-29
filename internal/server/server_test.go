package server_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hearth/internal/config"
	"hearth/internal/media"
	"hearth/internal/server"
	"hearth/internal/store"
)

func TestHealthAndReady(t *testing.T) {
	ctx := context.Background()
	cfg := config.Defaults()
	cfg.DataDir = t.TempDir()
	st, err := store.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	srv, err := server.New(slog.New(slog.DiscardHandler), st, cfg, "test", media.Binaries{})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	for _, path := range []string{"/", "/healthz", "/readyz", "/api/meta"} {
		res, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("%s: %d %s", path, res.StatusCode, body)
		}
		if path == "/api/meta" {
			var got map[string]string
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatal(err)
			}
			if got["name"] != "hearth" || got["version"] != "test" {
				t.Fatalf("meta: %s", body)
			}
		}
		if path == "/" && !strings.Contains(string(body), `id="root"`) {
			t.Fatalf("spa: %s", body)
		}
	}
}
