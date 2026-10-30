package store_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"hearth/internal/auth"
	"hearth/internal/config"
	"hearth/internal/store"
)

func openMigrated(t *testing.T) (*store.Store, context.Context) {
	t.Helper()
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
	return st, ctx
}

func mustUser(t *testing.T, st *store.Store, ctx context.Context, name, role string) store.User {
	t.Helper()
	hash, err := auth.HashPassword("password1")
	if err != nil {
		t.Fatal(err)
	}
	u, err := st.CreateUser(ctx, name, hash, role)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func mustFile(t *testing.T, st *store.Store, ctx context.Context, owner store.User, name, visibility string, collectionID int64, sha string) store.File {
	t.Helper()
	f, err := st.AddFile(ctx, store.NewFile{
		OwnerID:      owner.ID,
		SHA256:       sha,
		Size:         12,
		RelPath:      "originals/" + sha,
		Name:         name,
		MediaKind:    store.KindOther,
		Mime:         "text/plain",
		Visibility:   visibility,
		CollectionID: collectionID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func fileNames(files []store.File) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Name
	}
	return out
}

func hasFile(files []store.File, name string) bool {
	for _, f := range files {
		if f.Name == name {
			return true
		}
	}
	return false
}

func hasCollection(cols []store.Collection, name string) bool {
	for _, c := range cols {
		if c.Name == name {
			return true
		}
	}
	return false
}

func TestSearchFTSRankingAndPermissions(t *testing.T) {
	st, ctx := openMigrated(t)
	ada := mustUser(t, st, ctx, "ada", store.RoleUser)
	bob := mustUser(t, st, ctx, "bob", store.RoleUser)
	admin := mustUser(t, st, ctx, "root", store.RoleAdmin)

	col, err := st.CreateCollection(ctx, ada, "海边相册", []string{store.KindOther})
	if err != nil {
		t.Fatal(err)
	}

	mustFile(t, st, ctx, ada, "sea", store.VisPrivate, 0, "sha-exact")
	mustFile(t, st, ctx, ada, "seaside.png", store.VisPrivate, 0, "sha-prefix")
	mustFile(t, st, ctx, ada, "the-sea-day.txt", store.VisPrivate, 0, "sha-contains")
	multi := mustFile(t, st, ctx, ada, "Hello World Photo.jpg", store.VisPrivate, 0, "sha-multi")
	zh := mustFile(t, st, ctx, ada, "海边.txt", store.VisPrivate, 0, "sha-zh")
	viaCol := mustFile(t, st, ctx, ada, "photo.jpg", store.VisPrivate, col.ID, "sha-viacol")
	shared := mustFile(t, st, ctx, ada, "sea-shared.txt", store.VisShared, 0, "sha-shared")
	mustFile(t, st, ctx, ada, "sea-secret.txt", store.VisPrivate, 0, "sha-secret")

	files, cols, err := st.Search(ctx, ada, "sea", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !hasFile(files, "sea") || !hasFile(files, "seaside.png") || !hasFile(files, "the-sea-day.txt") {
		t.Fatalf("expected sea* hits, got %v", fileNames(files))
	}
	// exact should rank before prefix; prefix before loose contains
	if files[0].Name != "sea" {
		t.Fatalf("exact rank first, got %v", fileNames(files))
	}
	prefixIdx, containsIdx := -1, -1
	for i, f := range files {
		if f.Name == "seaside.png" {
			prefixIdx = i
		}
		if f.Name == "the-sea-day.txt" {
			containsIdx = i
		}
	}
	if prefixIdx < 0 || containsIdx < 0 || prefixIdx >= containsIdx {
		t.Fatalf("prefix before contains, got %v", fileNames(files))
	}

	files, _, err = st.Search(ctx, ada, "Hello World", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !hasFile(files, multi.Name) || len(files) != 1 {
		t.Fatalf("multi-token: %v", fileNames(files))
	}
	files, _, err = st.Search(ctx, ada, "Hello Missing", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("AND should miss: %v", fileNames(files))
	}

	files, cols, err = st.Search(ctx, ada, "海边", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !hasFile(files, zh.Name) {
		t.Fatalf("chinese filename: %v", fileNames(files))
	}
	if !hasFile(files, viaCol.Name) {
		t.Fatalf("collection name association: %v", fileNames(files))
	}
	if !hasCollection(cols, col.Name) {
		t.Fatalf("collection hit: %+v", cols)
	}

	files, cols, err = st.Search(ctx, bob, "sea", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !hasFile(files, shared.Name) {
		t.Fatal("bob should see shared")
	}
	if hasFile(files, "sea-secret.txt") || hasFile(files, "seaside.png") {
		t.Fatalf("bob saw private: %v", fileNames(files))
	}
	if hasCollection(cols, col.Name) {
		t.Fatal("bob saw private collection")
	}

	files, _, err = st.Search(ctx, admin, "sea-secret", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !hasFile(files, "sea-secret.txt") {
		t.Fatalf("admin should see all: %v", fileNames(files))
	}

	_, _, err = st.Search(ctx, ada, "   ", "", "")
	if !errors.Is(err, store.ErrInvalid) {
		t.Fatalf("empty query: %v", err)
	}
}

func TestSearchRenameKeepsFTS(t *testing.T) {
	st, ctx := openMigrated(t)
	ada := mustUser(t, st, ctx, "ada", store.RoleUser)
	col, err := st.CreateCollection(ctx, ada, "旧名", []string{store.KindOther})
	if err != nil {
		t.Fatal(err)
	}
	f := mustFile(t, st, ctx, ada, "old.txt", store.VisPrivate, col.ID, "sha-ren")
	if _, err := st.RenameFile(ctx, ada, f.ID, "new-name.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.RenameCollection(ctx, col.ID, ada, "新相册"); err != nil {
		t.Fatal(err)
	}
	files, cols, err := st.Search(ctx, ada, "new-name", "", "")
	if err != nil || !hasFile(files, "new-name.txt") {
		t.Fatalf("renamed file: %v %v", fileNames(files), err)
	}
	files, cols, err = st.Search(ctx, ada, "新相册", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !hasCollection(cols, "新相册") {
		t.Fatalf("renamed collection: %+v", cols)
	}
	if !hasFile(files, "new-name.txt") {
		t.Fatalf("collection rename should refresh file fts: %v", fileNames(files))
	}
	files, _, err = st.Search(ctx, ada, "old", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if hasFile(files, "new-name.txt") || hasFile(files, "old.txt") {
		t.Fatalf("stale token still hits: %v", fileNames(files))
	}
}
