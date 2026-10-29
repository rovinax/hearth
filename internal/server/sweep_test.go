package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"hearth/internal/auth"
	"hearth/internal/config"
	"hearth/internal/media"
	"hearth/internal/store"
)

func TestSweepTrashRemovesBlob(t *testing.T) {
	ctx := context.Background()
	cfg := config.Defaults()
	cfg.DataDir = t.TempDir()
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
	content := []byte("gone")
	sum := sha256.Sum256(content)
	hexSum := hex.EncodeToString(sum[:])
	rel := hexSum[:2] + "/" + hexSum[2:4] + "/" + hexSum
	path, err := blobDiskPath(cfg.OriginalsDir(), rel)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := st.AddFile(ctx, store.NewFile{
		OwnerID: user.ID, SHA256: hexSum, Size: int64(len(content)), RelPath: rel,
		Name: "a.txt", MediaKind: store.KindOther, Mime: "text/plain", Visibility: store.VisPrivate,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := st.AddFile(ctx, store.NewFile{
		OwnerID: user.ID, SHA256: hexSum, Size: int64(len(content)), RelPath: rel,
		Name: "b.txt", MediaKind: store.KindOther, Mime: "text/plain", Visibility: store.VisPrivate,
	})
	if err != nil {
		t.Fatal(err)
	}
	srv, err := New(slog.New(slog.DiscardHandler), st, cfg, "test", media.Binaries{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.DeleteFile(ctx, first.ID, user); err != nil {
		t.Fatal(err)
	}
	srv.sweepTrash(ctx, time.Now().Add(31*24*time.Hour))
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("blob removed while still referenced: %v", err)
	}
	if _, err := st.GetFile(ctx, second.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.DeleteFile(ctx, second.ID, user); err != nil {
		t.Fatal(err)
	}
	srv.sweepTrash(ctx, time.Now().Add(31*24*time.Hour))
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expired blob still on disk: %v", err)
	}
	if _, err := st.GetFile(ctx, second.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expired row: %v", err)
	}
}
