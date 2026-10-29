package store_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"hearth/internal/config"
	"hearth/internal/store"
)

func TestMigrateEnqueueClaimFinish(t *testing.T) {
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

	if _, err := st.ClaimJob(ctx); !errors.Is(err, store.ErrNoJob) {
		t.Fatalf("claim empty: %v", err)
	}
	id, err := st.Enqueue(ctx, "noop", `{}`, 3)
	if err != nil {
		t.Fatal(err)
	}
	job, err := st.ClaimJob(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if job.ID != id || job.Kind != "noop" || job.Attempts != 1 {
		t.Fatalf("job: %+v", job)
	}
	if err := st.RequeueRunning(ctx); err != nil {
		t.Fatal(err)
	}
	again, err := st.ClaimJob(ctx)
	if err != nil || again.ID != job.ID {
		t.Fatalf("requeue: %+v %v", again, err)
	}
	if err := st.FinishJob(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimJob(ctx); !errors.Is(err, store.ErrNoJob) {
		t.Fatalf("claim after finish: %v", err)
	}
}
