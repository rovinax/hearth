// Package jobs 是进程内任务循环。具体任务在后续阶段注册。
package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"hearth/internal/store"
)

type Handler func(ctx context.Context, payload string) error

type Runner struct {
	log      *slog.Logger
	store    *store.Store
	workers  int
	poll     time.Duration
	handlers map[string]Handler
}

func New(log *slog.Logger, st *store.Store, workers int, poll time.Duration) *Runner {
	if workers < 1 {
		workers = 1
	}
	if poll <= 0 {
		poll = time.Second
	}
	return &Runner{
		log:      log,
		store:    st,
		workers:  workers,
		poll:     poll,
		handlers: map[string]Handler{},
	}
}

func (r *Runner) Register(kind string, handler Handler) {
	r.handlers[kind] = handler
}

func (r *Runner) Run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errCh := make(chan error, r.workers)
	for i := 0; i < r.workers; i++ {
		go func() { errCh <- r.loop(ctx) }()
	}
	<-ctx.Done()
	for i := 0; i < r.workers; i++ {
		<-errCh
	}
}

func (r *Runner) loop(ctx context.Context) error {
	ticker := time.NewTicker(r.poll)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		job, err := r.store.ClaimJob(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if !errors.Is(err, store.ErrNoJob) {
				r.log.Error("claim job", "err", err)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
			}
			continue
		}
		r.dispatch(ctx, job)
	}
}

func (r *Runner) dispatch(ctx context.Context, job store.Job) {
	err := r.call(ctx, job)
	if err == nil {
		if finishErr := r.store.FinishJob(ctx, job.ID); finishErr != nil {
			r.log.Error("finish job", "id", job.ID, "err", finishErr)
		}
		return
	}
	r.log.Error("job failed", "id", job.ID, "kind", job.Kind, "err", err)
	if failErr := r.store.FailJob(ctx, job, err.Error()); failErr != nil {
		r.log.Error("record job failure", "id", job.ID, "err", failErr)
	}
}

func (r *Runner) call(ctx context.Context, job store.Job) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("panic: %v", rec)
		}
	}()
	handler, ok := r.handlers[job.Kind]
	if !ok {
		return fmt.Errorf("no handler for %s", job.Kind)
	}
	return handler(ctx, job.Payload)
}
