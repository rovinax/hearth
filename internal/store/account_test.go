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

func TestEnsureAdminAndLastAdmin(t *testing.T) {
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
	if err := st.EnsureAdmin(ctx, "", ""); err == nil {
		t.Fatal("expected missing admin error")
	}
	if err := st.EnsureAdmin(ctx, "Admin", "password1"); err != nil {
		t.Fatal(err)
	}
	if err := st.EnsureAdmin(ctx, "other", "password2"); err != nil {
		t.Fatal(err)
	}
	user, hash, err := st.UserByUsername(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if !auth.VerifyPassword(hash, "password1") {
		t.Fatal("password was changed")
	}
	if err := st.SetDisabled(ctx, user.ID, true); !errors.Is(err, store.ErrLastAdmin) {
		t.Fatalf("disable last admin: %v", err)
	}
}
