package web

import "sync"

// TokenBagHub fans out "something changed" ticks per game. It carries no
// payloads: each subscriber re-reads the DB and builds its own personalized
// snapshot, so slow clients only coalesce, never block publishers.
//
// The hub is in-memory, which means it only reaches watchers connected to this
// process. That is fine for a self-hosted single-binary deployment; a multi-
// replica setup would need a shared bus behind the same interface.
type TokenBagHub struct {
	mu   sync.Mutex
	subs map[int]map[chan struct{}]struct{} // gameID -> subscriber tick channels

	closed    chan struct{}
	closeOnce sync.Once
}

// NewTokenBagHub creates an empty hub.
func NewTokenBagHub() *TokenBagHub {
	return &TokenBagHub{
		subs:   make(map[int]map[chan struct{}]struct{}),
		closed: make(chan struct{}),
	}
}

// Subscribe registers interest in a game's token bag. The returned channel
// receives a tick whenever the game changes; cancel must be called (once or
// more, it is idempotent) to release the subscription.
func (h *TokenBagHub) Subscribe(gameID int) (tick <-chan struct{}, cancel func()) {
	// Buffered(1) so Publish never blocks and bursts collapse into one tick.
	ch := make(chan struct{}, 1)

	h.mu.Lock()
	chans, ok := h.subs[gameID]
	if !ok {
		chans = make(map[chan struct{}]struct{})
		h.subs[gameID] = chans
	}
	chans[ch] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	return ch, func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			if chans, ok := h.subs[gameID]; ok {
				delete(chans, ch)
				if len(chans) == 0 {
					delete(h.subs, gameID)
				}
			}
		})
	}
}

// Publish wakes every subscriber of a game. It never blocks: a subscriber that
// has not drained its previous tick already knows it is behind.
func (h *TokenBagHub) Publish(gameID int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for ch := range h.subs[gameID] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// Close wakes every stream so they can end before the HTTP server shuts down.
// It is idempotent.
func (h *TokenBagHub) Close() {
	h.closeOnce.Do(func() { close(h.closed) })
}

// Done is closed when the hub is closed, signalling every stream to end.
func (h *TokenBagHub) Done() <-chan struct{} {
	return h.closed
}

// subscriberCount reports how many subscribers a game has. Test-only.
func (h *TokenBagHub) subscriberCount(gameID int) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs[gameID])
}

// gameCount reports how many games have at least one subscriber. Test-only.
func (h *TokenBagHub) gameCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}
