package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/loomi-labs/clockkeeper/internal/database"
	"github.com/loomi-labs/clockkeeper/internal/logger"
	"github.com/loomi-labs/clockkeeper/internal/web"
)

// CreateUserCmd creates a new user in the database.
type CreateUserCmd struct {
	Username string `arg:"" help:"Username for the new user."`
	Password string `arg:"" help:"Password for the new user."`
}

func (c *CreateUserCmd) Run() error {
	logger.Setup()

	dbConfig := database.LoadConfigFromEnv()
	db, sqlDB, err := database.NewClient(dbConfig)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer sqlDB.Close()
	defer db.Close()

	hash, err := web.HashPassword(c.Password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	ctx := context.Background()
	_, err = db.User.Create().
		SetUsername(c.Username).
		SetPasswordHash(hash).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	slog.Info("user created", "username", c.Username)
	return nil
}
