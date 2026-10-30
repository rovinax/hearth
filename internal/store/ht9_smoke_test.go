package store_test

import (
	"context"
	"log/slog"
	"testing"

	"hearth/internal/config"
	"hearth/internal/store"
)

func TestHT9SakuraAndMultiKind(t *testing.T) {
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
	if err := st.EnsureAdmin(ctx, "admin", "password123"); err != nil {
		t.Fatal(err)
	}
	u, err := st.UserByID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	next, err := st.UpdateProfile(ctx, u.ID, "A", "", "sakura")
	if err != nil {
		t.Fatalf("sakura: %v", err)
	}
	if next.Theme != "sakura" {
		t.Fatalf("theme=%q", next.Theme)
	}
	if _, err := st.UpdateProfile(ctx, u.ID, "A", "", "nope"); err == nil {
		t.Fatal("bad theme accepted")
	}
	col, err := st.CreateCollection(ctx, next, "混合", []string{"image", "video"})
	if err != nil {
		t.Fatal(err)
	}
	if col.MediaKind != "image" || len(col.MediaKinds) != 2 || col.MediaKinds[0] != "image" || col.MediaKinds[1] != "video" {
		t.Fatalf("%+v", col)
	}
	g, err := st.CreateGroup(ctx, next, "家人")
	if err != nil {
		t.Fatal(err)
	}
	g2, err := st.RenameGroup(ctx, g.ID, next, "家族")
	if err != nil {
		t.Fatal(err)
	}
	if g2.Name != "家族" {
		t.Fatalf("%+v", g2)
	}
}
