package web

import "github.com/loomi-labs/clockkeeper/internal/env"

// Config holds web server configuration.
type Config struct {
	Listen       string
	JWTSecretKey string
}

// LoadConfigFromEnv loads web configuration from environment variables.
func LoadConfigFromEnv() *Config {
	jwtSecret, err := env.GetStringFromFile("JWT_SECRET_KEY_FILE")
	if err != nil {
		jwtSecret = env.GetString("JWT_SECRET_KEY", "")
	}
	return &Config{
		Listen:       env.GetString("WEB_LISTEN", ":8080"),
		JWTSecretKey: jwtSecret,
	}
}
