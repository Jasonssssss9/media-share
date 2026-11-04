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

	"github.com/Jasonssssss9/media-share/backend/internal/api"
	"github.com/Jasonssssss9/media-share/backend/internal/auth"
	"github.com/Jasonssssss9/media-share/backend/internal/config"
	"github.com/Jasonssssss9/media-share/backend/internal/imagegen"
	"github.com/Jasonssssss9/media-share/backend/internal/media"
	"github.com/Jasonssssss9/media-share/backend/internal/posts"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("configure database: %w", err)
	}
	defer db.Close()

	connectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = db.Ping(connectCtx)
	cancel()
	if err != nil {
		return fmt.Errorf("connect to PostgreSQL: %w", err)
	}

	authService, err := auth.NewService(db, cfg.JWTSecret)
	if err != nil {
		return err
	}
	mediaService, err := media.NewService(db, cfg.MediaDir)
	if err != nil {
		return fmt.Errorf("configure image storage: %w", err)
	}
	mux := api.NewHandler(db.Ping)
	api.RegisterPosts(mux, posts.NewService(db), mediaService, authService, cfg.AppOrigin, logger)
	api.RegisterMedia(mux, mediaService, authService, cfg.AppOrigin, logger)
	api.RegisterProfiles(mux, authService, mediaService, cfg.AppOrigin, logger)
	api.RegisterImageGeneration(mux, imagegen.NewClient(cfg.OpenAIAPIKey, cfg.OpenAIImageModel), mediaService, authService, cfg.AppOrigin, logger)
	handler, err := api.WithAuth(mux, authService, cfg.AppOrigin, logger)
	if err != nil {
		return fmt.Errorf("configure frontend origin: %w", err)
	}

	server := &http.Server{
		Addr:              cfg.Address,
		Handler:           api.Middleware(logger, handler),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	stopped := make(chan struct{})
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown timed out", "error", err)
			_ = server.Close()
		}
		close(stopped)
	}()

	logger.Info("server started", "address", cfg.Address)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}
	<-stopped
	return nil
}
