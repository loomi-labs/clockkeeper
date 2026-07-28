package web

import (
	"time"

	"github.com/loomi-labs/clockkeeper/internal/env"
)

// DiscordConfigured returns true if Discord OAuth is set up.
func (c *Config) DiscordConfigured() bool {
	return c.DiscordClientID != "" && c.DiscordClientSecret != "" && c.DiscordRedirectURI != ""
}

// SpotifyConfigured returns true if Spotify OAuth is set up.
func (c *Config) SpotifyConfigured() bool {
	return c.SpotifyClientID != "" && c.SpotifyClientSecret != "" && c.SpotifyRedirectURI != ""
}

// defaultRateLimitAnon is the anonymous request budget, per minute, per IP.
//
// Token bag players are unauthenticated, so every phone at one table shares this
// SINGLE budget — they all sit behind the same venue NAT. At 300/min the burst
// (see NewRateLimitInterceptor: max(rate/3, 5)) is 100 requests, which absorbs a
// full table of 15 phones scanning the QR code at the same moment, each spending
// a handful of requests on the way in. Several rooms behind one NAT need it
// raised; see .env.example.
const defaultRateLimitAnon = 300

// Config holds web server configuration.
type Config struct {
	Listen              string
	JWTSecretKey        string
	DiscordClientID     string
	DiscordClientSecret string
	DiscordRedirectURI  string
	SpotifyClientID     string
	SpotifyClientSecret string
	SpotifyRedirectURI  string
	RateLimitAnon       int
	RateLimitAuth       int
	AnonymousMaxAge     time.Duration
	DevSingleUser       bool
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

	spotifySecret, err := env.GetStringFromFile("SPOTIFY_CLIENT_SECRET_FILE")
	if err != nil {
		spotifySecret = env.GetString("SPOTIFY_CLIENT_SECRET", "")
	}

	return &Config{
		Listen:              env.GetString("WEB_LISTEN", ":8080"),
		JWTSecretKey:        jwtSecret,
		DiscordClientID:     env.GetString("DISCORD_CLIENT_ID", ""),
		DiscordClientSecret: discordSecret,
		DiscordRedirectURI:  env.GetString("DISCORD_REDIRECT_URI", ""),
		SpotifyClientID:     env.GetString("SPOTIFY_CLIENT_ID", ""),
		SpotifyClientSecret: spotifySecret,
		SpotifyRedirectURI:  env.GetString("SPOTIFY_REDIRECT_URI", ""),
		RateLimitAnon:       env.GetInt("RATE_LIMIT_ANON", defaultRateLimitAnon),
		RateLimitAuth:       env.GetInt("RATE_LIMIT_AUTH", 120),
		AnonymousMaxAge:     env.GetDuration("ANONYMOUS_MAX_AGE", "8760h"),
		DevSingleUser:       env.GetBool("DEV_SINGLE_USER", false),
	}
}
