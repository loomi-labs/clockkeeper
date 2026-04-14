package web

import (
	"github.com/loomi-labs/clockkeeper/ent"
	"github.com/loomi-labs/clockkeeper/internal/botc"
)

// ClockKeeperServiceHandler implements the ConnectRPC ClockKeeperService.
type ClockKeeperServiceHandler struct {
	config   *Config
	db       *ent.Client
	auth     *AuthInterceptor
	registry *botc.Registry
}
