package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Jasonssssss9/media-share/backend/internal/config"
	"github.com/Jasonssssss9/media-share/backend/internal/posts"
	"github.com/Jasonssssss9/media-share/backend/internal/search"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		return fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	client, err := search.NewClient(cfg.ElasticsearchURL, cfg.ElasticsearchPassword, cfg.ElasticsearchIndex)
	if err != nil {
		return err
	}
	count, err := search.NewService(posts.NewService(db), client).Rebuild(ctx)
	if err != nil {
		return fmt.Errorf("rebuild search index: %w", err)
	}
	fmt.Printf("Rebuilt search index %s with %d posts.\n", cfg.ElasticsearchIndex, count)
	return nil
}
