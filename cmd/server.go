package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	clockkeeper "github.com/loomi-labs/clockkeeper"
	"github.com/loomi-labs/clockkeeper/internal/botc"
	"github.com/loomi-labs/clockkeeper/internal/database"
	"github.com/loomi-labs/clockkeeper/internal/logger"
	"github.com/loomi-labs/clockkeeper/internal/web"
)

// ServeCmd starts the Clock Keeper server.
type ServeCmd struct{}

func (s *ServeCmd) Run() error {
	logger.Setup()
	slog.Info("starting clockkeeper")

	dbConfig := database.LoadConfigFromEnv()
	webConfig := web.LoadConfigFromEnv()

	if webConfig.JWTSecretKey == "" {
		return fmt.Errorf("JWT_SECRET_KEY or JWT_SECRET_KEY_FILE must be set")
	}

	if !webConfig.DiscordConfigured() {
		slog.Warn("DISCORD_CLIENT_ID or DISCORD_CLIENT_SECRET not set — Discord login unavailable, anonymous-only mode")
	}

	db, sqlDB, err := database.NewClient(dbConfig)
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	defer db.Close()

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	slog.Info("database connected")

	registry, err := botc.NewRegistry(clockkeeper.RolesJSON, clockkeeper.JinxesJSON, clockkeeper.NightSheetJSON)
	if err != nil {
		return fmt.Errorf("loading game data: %w", err)
	}
	slog.Info("game data loaded", "characters", len(registry.AllCharacters()))

	staticFiles, err := fs.Sub(clockkeeper.StaticFiles, "web/build")
	if err != nil {
		return err
	}

	characterIcons, err := fs.Sub(clockkeeper.CharacterIcons, "data/characters")
	if err != nil {
		return err
	}

	server := web.NewServer(webConfig, db, registry, staticFiles, characterIcons)

	go func() {
		if listenErr := server.ListenAndServe(); listenErr != nil && listenErr != http.ErrServerClosed {
			slog.Error("server error", "error", listenErr)
		}
	}()

	slog.Info("clockkeeper is running", "listen", webConfig.Listen)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}

	slog.Info("clockkeeper stopped")
	return nil
}
