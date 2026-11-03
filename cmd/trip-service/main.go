package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SizikovK/TripGo-MVP/internal/config"
	"github.com/SizikovK/TripGo-MVP/internal/container"
)

func main() {
	if err := run(); err != nil {
		slog.Error("service stopped with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	stopped := make(chan struct{})
	defer close(stopped)
	cfg, err := config.New()
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	app, err := container.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("initialize service: %w", err)
	}
	defer func() {
		if err := app.Close(ctx); err != nil {
			app.Logger.WarnContext(ctx, "cleanup failed", "error", err)
		}
	}()
	app.Logger.InfoContext(ctx, "service started", "addr", app.Server.Addr)
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- app.Server.ListenAndServe() }()
	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP server: %w", err)
		}
		return nil
	case <-ctx.Done():
		app.Logger.InfoContext(ctx, "service stopping")
		go func() {
			timer := time.NewTimer(cfg.ShutdownTimeout)
			defer timer.Stop()
			select {
			case <-stopped:
				return
			case <-timer.C:
				app.Logger.Error("shutdown timeout")
				os.Exit(1)
			}
		}()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := app.Server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown HTTP: %w", err)
		}
		app.Logger.InfoContext(shutdownCtx, "HTTP server stopped")
		return nil
	}
}
