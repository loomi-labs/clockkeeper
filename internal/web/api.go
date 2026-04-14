package web

import (
	"github.com/shifty11/clockkeeper/ent"
	"github.com/shifty11/clockkeeper/internal/botc"
)

// ClockKeeperServiceHandler implements the ConnectRPC ClockKeeperService.
type ClockKeeperServiceHandler struct {
	config   *Config
	db       *ent.Client
	auth     *AuthInterceptor
	registry *botc.Registry
}
