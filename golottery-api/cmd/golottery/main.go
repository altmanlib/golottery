package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"golottery/api/internal/apihttp"
	"golottery/api/internal/config"
	"golottery/api/internal/httpapi"
	"golottery/api/internal/settings"
	"golottery/api/internal/store"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "settings":
			if err := runSettings(context.Background(), os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "settings: %v\n", err)
				os.Exit(1)
			}
			return
		case "hash-password":
			if err := runHashPassword(); err != nil {
				fmt.Fprintf(os.Stderr, "hash-password: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	if err := runServer(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func runServer() error {
	cfg, err := config.Bootstrap()
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx := context.Background()

	db, err := store.Open(ctx, store.Config{URL: cfg.DatabaseURL, Logger: logger})
	if err != nil {
		return fmt.Errorf("store open error: %w", err)
	}
	if err := db.Migrate(ctx); err != nil {
		_ = db.Close()
		return fmt.Errorf("store migrate error: %w", err)
	}

	settingsStore := settings.NewStore(db.Gorm)
	snapshot, err := settingsStore.Snapshot(ctx)
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("settings snapshot error: %w", err)
	}
	if err := cfg.Apply(snapshot, func(format string, args ...any) {
		logger.Warn(fmt.Sprintf(format, args...))
	}); err != nil {
		_ = db.Close()
		return fmt.Errorf("settings apply error: %w", err)
	}

	router := httpapi.NewRouter(httpapi.Deps{
		Logger:         logger,
		TrustedProxies: cfg.TrustedProxies,
		Readiness:      db.Ping,
	})
	apihttp.Register(router, db.Gorm)

	return run(ctx, runDeps{
		Addr:       cfg.Addr(),
		Handler:    router,
		Logger:     logger,
		CloseStore: db.Close,
	})
}
