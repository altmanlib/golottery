package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"golottery/api/internal/config"
	"golottery/api/internal/settings"
	"golottery/api/internal/store"
)

func runSettings(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: golottery settings list|set|unset")
	}
	cfg, err := config.Bootstrap()
	if err != nil {
		return err
	}
	db, err := store.Open(ctx, store.Config{URL: cfg.DatabaseURL})
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	if err := db.Migrate(ctx); err != nil {
		return err
	}
	s := settings.NewStore(db.Gorm)

	switch args[0] {
	case "list":
		return listSettings(ctx, s, os.Stdout)
	case "set":
		if len(args) != 3 {
			return fmt.Errorf("usage: golottery settings set KEY VALUE")
		}
		return setSetting(ctx, s, args[1], args[2])
	case "unset":
		if len(args) != 2 {
			return fmt.Errorf("usage: golottery settings unset KEY")
		}
		if err := requireAppKey(args[1]); err != nil {
			return err
		}
		return s.Unset(ctx, args[1])
	default:
		return fmt.Errorf("unknown settings command %q", args[0])
	}
}

func listSettings(ctx context.Context, s *settings.Store, w io.Writer) error {
	rows, err := s.List(ctx)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "KEY\tVALUE\tCATEGORY")
	for _, row := range rows {
		value := row.Value
		if spec, ok := config.Lookup(row.Key); ok && spec.Secret {
			value = config.MaskSecret(value)
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", row.Key, value, row.Group)
	}
	return tw.Flush()
}

func setSetting(ctx context.Context, s *settings.Store, key, value string) error {
	if err := config.ValidateAppValue(key, value); err != nil {
		return err
	}
	spec, _ := config.Lookup(key)
	return s.Set(ctx, key, value, string(spec.Group), "cli")
}

func requireAppKey(key string) error {
	spec, ok := config.Lookup(key)
	if !ok {
		return fmt.Errorf("unknown key %q", key)
	}
	if spec.Scope != config.ScopeApp {
		return fmt.Errorf("%s is infrastructure config; set it via .env or environment", key)
	}
	return nil
}
