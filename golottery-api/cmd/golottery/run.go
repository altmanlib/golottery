package main

import (
	"context"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

type runDeps struct {
	Addr       string
	Handler    http.Handler
	Logger     *slog.Logger
	CloseStore func() error
}

func run(parent context.Context, deps runDeps) error {
	ctx, stop := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	server := &http.Server{
		Addr:              deps.Addr,
		Handler:           deps.Handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		deps.Logger.Info("golottery listening", "addr", deps.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
			stop()
		}
	}()

	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-errCh:
	}

	stop()
	shutdownHTTP(server, deps.Logger)
	if deps.CloseStore != nil {
		if err := deps.CloseStore(); err != nil {
			deps.Logger.Warn("store close failed", "error", err)
		}
	}
	deps.Logger.Info("golottery stopped")
	return runErr
}

func shutdownHTTP(server *http.Server, logger *slog.Logger) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Warn("http shutdown failed", "error", err)
	}
}
