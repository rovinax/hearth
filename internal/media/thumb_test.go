package media_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"hearth/internal/auth"
	"hearth/internal/config"
	"hearth/internal/media"
	"hearth/internal/store"
)

func TestThumbJobUsesExternalTool(t *testing.T) {
	binDir := t.TempDir()
	script := filepath.Join(binDir, "vips")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf thumb > \"$3\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults()
	cfg.DataDir = t.TempDir()
	ctx := context.Background()
	st, err := store.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.Migrate(ctx, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}
	hash, err := auth.HashPassword("password1")
	if err != nil {
		t.Fatal(err)
	}
	user, err := st.CreateUser(ctx, "ada", hash, store.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	sum := strings.Repeat("ab", 32)
	rel := sum[:2] + "/" + sum[2:4] + "/" + sum
	if err := os.MkdirAll(filepath.Join(cfg.OriginalsDir(), sum[:2], sum[2:4]), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.OriginalsDir(), filepath.FromSlash(rel)), []byte("img"), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := st.AddFile(ctx, store.NewFile{
		OwnerID: user.ID, SHA256: sum, Size: 3, RelPath: rel,
		Name: "a.jpg", MediaKind: store.KindImage, Mime: "image/jpeg", Visibility: store.VisPrivate,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = media.ThumbJob(st, cfg.OriginalsDir(), cfg.CacheDir(), media.Binaries{Vips: script})(ctx, strconv.FormatInt(file.ID, 10))
	if err != nil {
		t.Fatal(err)
	}
	got, err := st.GetFile(ctx, file.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ThumbRel == "" {
		t.Fatal("thumb was not recorded")
	}
}

func TestTakenAtMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plain.txt")
	if err := os.WriteFile(path, []byte("no exif"), 0o644); err != nil {
		t.Fatal(err)
	}
	if media.TakenAt(path) != "" {
		t.Fatal("expected no timestamp")
	}
}
