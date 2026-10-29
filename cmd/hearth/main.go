package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"hearth/internal/buildinfo"
	"hearth/internal/config"
	"hearth/internal/jobs"
	"hearth/internal/media"
	"hearth/internal/server"
	"hearth/internal/store"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "hearth: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := config.Load(args, os.Getenv)
	if err != nil {
		return err
	}
	logger := config.NewLogger(cfg.Log)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, cfg)
	if err != nil {
		return err
	}
	defer st.Close()

	if err := st.Migrate(ctx, logger); err != nil {
		return err
	}
	if err := st.EnsureAdmin(ctx, cfg.AdminUser, cfg.AdminPassword); err != nil {
		return err
	}
	if err := st.RequeueRunning(ctx); err != nil {
		return err
	}
	if err := media.Reclassify(ctx, st, cfg.OriginalsDir(), logger); err != nil {
		return err
	}
	if err := media.BackfillOfficeThumbs(ctx, st, cfg.OriginalsDir(), cfg.CacheDir(), logger); err != nil {
		return err
	}

	bins := media.Discover()
	logger.Info("media tools", "ffmpeg", bins.FFmpeg, "ffprobe", bins.FFProbe, "vips", bins.Vips)

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	runner := jobs.New(logger, st, cfg.Jobs.Workers, cfg.Jobs.PollInterval.Std())
	runner.Register("thumbnail", media.ThumbJob(st, cfg.OriginalsDir(), cfg.CacheDir(), bins))
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		runner.Run(runCtx)
	}()

	srv, err := server.New(logger, st, cfg, buildinfo.Version, bins)
	if err != nil {
		return err
	}
	err = srv.Run(runCtx)
	cancel()
	wg.Wait()
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	logger.Info("stopped")
	return nil
}
