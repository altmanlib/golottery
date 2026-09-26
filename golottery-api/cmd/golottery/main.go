package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"golottery/api/internal/apihttp"
	"golottery/api/internal/auth"
	"golottery/api/internal/config"
	"golottery/api/internal/draw"
	"golottery/api/internal/event"
	"golottery/api/internal/guest"
	"golottery/api/internal/httpapi"
	"golottery/api/internal/org"
	"golottery/api/internal/platform"
	"golottery/api/internal/ratelimit"
	"golottery/api/internal/redisx"
	"golottery/api/internal/settings"
	"golottery/api/internal/store"
	"golottery/api/internal/wechat"
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

	rdb, err := redisx.Open(ctx, cfg.RedisURL)
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("redis open error: %w", err)
	}
	closeAll := func() error {
		_ = rdb.Close()
		return db.Close()
	}

	settingsStore := settings.NewStore(db.Gorm)
	snapshot, err := settingsStore.Snapshot(ctx)
	if err != nil {
		_ = closeAll()
		return fmt.Errorf("settings snapshot error: %w", err)
	}
	if err := cfg.Apply(snapshot, func(format string, args ...any) {
		logger.Warn(fmt.Sprintf(format, args...))
	}); err != nil {
		_ = closeAll()
		return fmt.Errorf("settings apply error: %w", err)
	}

	seeded, err := platform.Seed(ctx, db.Gorm, cfg.PlatformUser, cfg.PlatformPasswordHash)
	if err != nil {
		_ = closeAll()
		return fmt.Errorf("platform seed error: %w", err)
	}
	if seeded {
		logger.Info("seeded platform operator", "username", cfg.PlatformUser)
	}

	tokens := auth.NewTokenIssuer(db.Gorm, cfg.ConsoleSessionTTL, cfg.HostSessionTTL, cfg.PlatformSessionTTL)
	limiter := auth.NewLoginLimiter(db.Gorm, cfg.LoginMaxFailures, cfg.LoginWindow)

	wechatClient := wechat.New(wechat.Config{
		AppID:      cfg.WechatAppID,
		AppSecret:  cfg.WechatAppSecret,
		BaseURL:    cfg.WechatAPIBase,
		EnvVersion: cfg.WechatEnvVersion,
		Redis:      rdb,
		Logger:     logger,
	})
	draws := draw.NewService(draw.Config{
		DB: db.Gorm, Tokens: tokens, Limiter: limiter, Redis: rdb, Logger: logger,
	})

	router := httpapi.NewRouter(httpapi.Deps{
		Logger:         logger,
		TrustedProxies: cfg.TrustedProxies,
		Readiness:      db.Ping,
	})
	if err := apihttp.Register(router, apihttp.Deps{
		DB:       db.Gorm,
		Redis:    rdb,
		Tokens:   tokens,
		Platform: platform.NewService(db.Gorm, tokens, limiter),
		Orgs:     org.NewService(db.Gorm),
		Accounts: org.NewAccounts(db.Gorm, tokens, limiter),
		Events:   event.NewService(db.Gorm),
		Wechat:   wechatClient,
		Guests: guest.NewService(db.Gorm, guest.Config{
			Mode:       cfg.GuestLoginMode,
			SessionTTL: cfg.GuestSessionTTL,
			Wechat:     wechatClient,
			Limiter:    ratelimit.New(rdb, logger),
		}),
		Draws:  draws,
		Logger: logger,
	}); err != nil {
		_ = closeAll()
		return err
	}

	return run(ctx, runDeps{
		Addr:       cfg.Addr(),
		Handler:    router,
		Logger:     logger,
		CloseStore: closeAll,
		OnShutdown: draws.Hub().Close,
	})
}
