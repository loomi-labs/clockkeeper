package web

import (
	"time"

	"github.com/shifty11/clockkeeper/internal/env"
)

// DiscordConfigured returns true if Discord OAuth is set up.
func (c *Config) DiscordConfigured() bool {
	return c.DiscordClientID != "" && c.DiscordClientSecret != "" && c.DiscordRedirectURI != ""
}

// Config holds web server configuration.
type Config struct {
	Listen              string
	JWTSecretKey        string
	DiscordClientID     string
	DiscordClientSecret string
	DiscordRedirectURI  string
	RateLimitAnon       int
	RateLimitAuth       int
	AnonymousMaxAge     time.Duration
}

// LoadConfigFromEnv loads web configuration from environment variables.
func LoadConfigFromEnv() *Config {
	jwtSecret, err := env.GetStringFromFile("JWT_SECRET_KEY_FILE")
	if err != nil {
		jwtSecret = env.GetString("JWT_SECRET_KEY", "")
	}

	discordSecret, err := env.GetStringFromFile("DISCORD_CLIENT_SECRET_FILE")
	if err != nil {
		discordSecret = env.GetString("DISCORD_CLIENT_SECRET", "")
	}

	return &Config{
		Listen:              env.GetString("WEB_LISTEN", ":8080"),
		JWTSecretKey:        jwtSecret,
		DiscordClientID:     env.GetString("DISCORD_CLIENT_ID", ""),
		DiscordClientSecret: discordSecret,
		DiscordRedirectURI:  env.GetString("DISCORD_REDIRECT_URI", ""),
		RateLimitAnon:       env.GetInt("RATE_LIMIT_ANON", 120),
		RateLimitAuth:       env.GetInt("RATE_LIMIT_AUTH", 120),
		AnonymousMaxAge:     env.GetDuration("ANONYMOUS_MAX_AGE", "8760h"),
	}
}
