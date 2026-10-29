package jobs_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"hearth/internal/config"
	"hearth/internal/jobs"
	"hearth/internal/store"
)

func TestRunnerRunsRegisteredHandler(t *testing.T) {
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
	if _, err := st.Enqueue(ctx, "ping", `{"ok":true}`, 3); err != nil {
		t.Fatal(err)
	}

	done := make(chan string, 1)
	runner := jobs.New(slog.New(slog.DiscardHandler), st, 1, 20*time.Millisecond)
	runner.Register("ping", func(context.Context, string) error {
		done <- "ran"
		return nil
	})

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go runner.Run(runCtx)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler was not called")
	}
	cancel()
}
