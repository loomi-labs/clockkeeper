package web

import (
	"sync"
	"time"

	"github.com/loomi-labs/clockkeeper/ent"
	"github.com/loomi-labs/clockkeeper/internal/botc"
)

// ClockKeeperServiceHandler implements the ConnectRPC ClockKeeperService.
type ClockKeeperServiceHandler struct {
	config   *Config
	db       *ent.Client
	auth     *AuthInterceptor
	registry *botc.Registry

	// hub wakes the WatchTokenBag streams of a game after every mutation.
	hub *TokenBagHub

	// watchHeartbeatInterval overrides defaultWatchHeartbeat. Test hook only —
	// zero means the default.
	watchHeartbeatInterval time.Duration

	// spotifyMu guards spotifyLocks. Each per-user mutex serializes Spotify
	// token refreshes so concurrent requests can't race on the rotating
	// refresh token.
	spotifyMu    sync.Mutex
	spotifyLocks map[int]*sync.Mutex
}

// spotifyUserLock returns the refresh lock for a user, creating it on first use.
func (h *ClockKeeperServiceHandler) spotifyUserLock(userID int) *sync.Mutex {
	h.spotifyMu.Lock()
	defer h.spotifyMu.Unlock()

	if h.spotifyLocks == nil {
		h.spotifyLocks = make(map[int]*sync.Mutex)
	}
	mu, ok := h.spotifyLocks[userID]
	if !ok {
		mu = &sync.Mutex{}
		h.spotifyLocks[userID] = mu
	}
	return mu
}
