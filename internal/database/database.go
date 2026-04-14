package database

import (
	"database/sql"
	"fmt"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/shifty11/clockkeeper/ent"
	"github.com/shifty11/clockkeeper/internal/env"

	_ "github.com/lib/pq"
)

// Config holds the database connection parameters.
type Config struct {
	Host     string
	Port     string
	Name     string
	User     string
	Password string
}

// LoadConfigFromEnv loads database configuration from environment variables.
func LoadConfigFromEnv() *Config {
	password, err := env.GetStringFromFile("DB_PASSWORD_FILE")
	if err != nil {
		password = env.GetString("DB_PASSWORD", "postgres")
	}
	return &Config{
		Host:     env.GetString("DB_HOST", "localhost"),
		Port:     env.GetString("DB_PORT", "5432"),
		Name:     env.GetString("DB_NAME", "clockkeeper"),
		User:     env.GetString("DB_USER", "postgres"),
		Password: password,
	}
}

// ConnectionString returns the PostgreSQL connection string.
func (c *Config) ConnectionString() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.User, c.Password, c.Host, c.Port, c.Name)
}

// NewClient creates a new Ent client connected to PostgreSQL.
func NewClient(config *Config) (*ent.Client, *sql.DB, error) {
	drv, err := entsql.Open(dialect.Postgres, config.ConnectionString())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open database connection: %w", err)
	}
	client := ent.NewClient(ent.Driver(drv))
	return client, drv.DB(), nil
}
