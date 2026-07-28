package web

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tickTimeout bounds every wait on a hub tick. Generous enough for a loaded CI
// machine, short enough that a missing tick fails the test instead of hanging it.
const tickTimeout = 2 * time.Second

// requireTick asserts a tick arrives.
func requireTick(t *testing.T, tick <-chan struct{}) {
	t.Helper()
	select {
	case <-tick:
	case <-time.After(tickTimeout):
		t.Fatal("expected a tick, got none")
	}
}

// requireNoTick asserts no tick is pending right now.
func requireNoTick(t *testing.T, tick <-chan struct{}) {
	t.Helper()
	select {
	case <-tick:
		t.Fatal("expected no tick")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestTokenBagHub_PublishDeliversTick(t *testing.T) {
	hub := NewTokenBagHub()
	defer hub.Close()

	tick, cancel := hub.Subscribe(7)
	defer cancel()

	hub.Publish(7)
	requireTick(t, tick)
}

func TestTokenBagHub_PublishOnlyReachesItsOwnGame(t *testing.T) {
	hub := NewTokenBagHub()
	defer hub.Close()

	tick, cancel := hub.Subscribe(7)
	defer cancel()

	// No subscribers on game 8 at all — must not panic, must not cross over.
	hub.Publish(8)
	requireNoTick(t, tick)
}

func TestTokenBagHub_BurstsCoalesceAndNeverBlock(t *testing.T) {
	hub := NewTokenBagHub()
	defer hub.Close()

	tick, cancel := hub.Subscribe(7)
	defer cancel()

	// A subscriber that has not drained yet must not stall the publisher. If
	// Publish blocked, this loop would never finish and the test would time out.
	done := make(chan struct{})
	go func() {
		for range 1000 {
			hub.Publish(7)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(tickTimeout):
		t.Fatal("Publish blocked on a subscriber that had not drained")
	}

	// The burst collapsed into a single pending tick: the subscriber learns
	// "something changed", not how often.
	requireTick(t, tick)
	requireNoTick(t, tick)
}

func TestTokenBagHub_CancelRemovesSubscription(t *testing.T) {
	hub := NewTokenBagHub()
	defer hub.Close()

	tick, cancel := hub.Subscribe(7)
	_, cancelOther := hub.Subscribe(7)
	require.Equal(t, 2, hub.subscriberCount(7))

	cancel()
	cancel() // idempotent
	assert.Equal(t, 1, hub.subscriberCount(7))
	assert.Equal(t, 1, hub.gameCount(), "the game still has one subscriber")

	// A cancelled subscriber is no longer published to.
	hub.Publish(7)
	requireNoTick(t, tick)

	cancelOther()
	assert.Equal(t, 0, hub.subscriberCount(7))
	assert.Equal(t, 0, hub.gameCount(), "the empty per-game map must be dropped")
}

func TestTokenBagHub_CloseWakesSubscribers(t *testing.T) {
	hub := NewTokenBagHub()

	tick, cancel := hub.Subscribe(7)
	defer cancel()

	// A stream blocked on its select must be woken by Close, or graceful
	// shutdown would wait for it forever.
	woken := make(chan struct{})
	go func() {
		select {
		case <-tick:
		case <-hub.Done():
		}
		close(woken)
	}()

	hub.Close()
	hub.Close() // idempotent

	select {
	case <-woken:
	case <-time.After(tickTimeout):
		t.Fatal("Close did not wake the blocked subscriber")
	}
}
