package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
	"github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1/clockkeeperv1connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// streamTimeout bounds every wait on a stream so a missing snapshot fails the
// test instead of hanging it.
const streamTimeout = 5 * time.Second

// watchClient serves the handler over HTTP with the real interceptor chain.
// Streams cannot be called as plain methods, and going through the chain is also
// what proves the public token bag procedures need no Authorization header.
func watchClient(t *testing.T, h *ClockKeeperServiceHandler) clockkeeperv1connect.ClockKeeperServiceClient {
	t.Helper()

	// Limits high enough that they never interfere; the point here is that the
	// interceptor is in the chain at all.
	rateLimiter := NewRateLimitInterceptor(6000, 6000)
	t.Cleanup(rateLimiter.Stop)

	path, rpcHandler := clockkeeperv1connect.NewClockKeeperServiceHandler(h, connect.WithInterceptors(h.auth, rateLimiter))
	mux := http.NewServeMux()
	mux.Handle(path, rpcHandler)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return clockkeeperv1connect.NewClockKeeperServiceClient(srv.Client(), srv.URL)
}

// watchReader pumps a watch stream into a channel so tests can wait on snapshots
// with a timeout instead of blocking on Receive.
type watchReader struct {
	t    *testing.T
	msgs chan *clockkeeperv1.WatchTokenBagResponse
	done chan struct{} // closed once the stream ended
	err  error         // stream error, set before done is closed
}

// startWatch dials WatchTokenBag. No Authorization header is ever set.
func startWatch(t *testing.T, client clockkeeperv1connect.ClockKeeperServiceClient, code, secret string) *watchReader {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	stream, err := client.WatchTokenBag(ctx, connect.NewRequest(&clockkeeperv1.WatchTokenBagRequest{
		Code:               code,
		RegistrationSecret: secret,
	}))
	require.NoError(t, err)

	w := &watchReader{
		t:    t,
		msgs: make(chan *clockkeeperv1.WatchTokenBagResponse, 64),
		done: make(chan struct{}),
	}
	go func() {
		defer close(w.done)
		for stream.Receive() {
			w.msgs <- stream.Msg()
		}
		w.err = stream.Err()
	}()

	// Wait for the stream to finish before the test server closes, otherwise
	// httptest.Server.Close blocks on this in-flight request.
	t.Cleanup(func() {
		cancel()
		_ = stream.Close()
		select {
		case <-w.done:
		case <-time.After(streamTimeout):
		}
	})

	return w
}

// next waits for the next snapshot.
func (w *watchReader) next() *clockkeeperv1.WatchTokenBagResponse {
	w.t.Helper()
	return w.nextMatching("a snapshot", func(*clockkeeperv1.WatchTokenBagResponse) bool { return true })
}

// nextMatching waits for the first snapshot satisfying want, skipping the ones
// that still describe an earlier state — publishes coalesce, so how many
// intermediate snapshots arrive is not something a test may pin down.
func (w *watchReader) nextMatching(what string, want func(*clockkeeperv1.WatchTokenBagResponse) bool) *clockkeeperv1.WatchTokenBagResponse {
	w.t.Helper()

	deadline := time.After(streamTimeout)
	for {
		select {
		case msg := <-w.msgs:
			if want(msg) {
				return msg
			}
		case <-w.done:
			// Everything the stream sent is buffered by now: the final snapshot
			// of an ending stream arrives together with the end itself.
			for {
				select {
				case msg := <-w.msgs:
					if want(msg) {
						return msg
					}
				default:
					w.t.Fatalf("stream ended before %s (err: %v)", what, w.err)
				}
			}
		case <-deadline:
			w.t.Fatalf("timed out waiting for %s", what)
		}
	}
}

// requireEnded asserts the server closed the stream itself, without an error.
func (w *watchReader) requireEnded() {
	w.t.Helper()
	select {
	case <-w.done:
		require.NoError(w.t, w.err, "the stream must end cleanly, not with an error")
	case <-time.After(streamTimeout):
		w.t.Fatal("timed out waiting for the stream to end")
	}
}

func revealedPhase(s *clockkeeperv1.WatchTokenBagResponse) bool {
	return s.Phase == clockkeeperv1.TokenBagPhase_TOKEN_BAG_PHASE_REVEALED
}

func closedPhase(s *clockkeeperv1.WatchTokenBagResponse) bool {
	return s.Phase == clockkeeperv1.TokenBagPhase_TOKEN_BAG_PHASE_CLOSED
}

func inactivePhase(s *clockkeeperv1.WatchTokenBagResponse) bool {
	return s.Phase == clockkeeperv1.TokenBagPhase_TOKEN_BAG_PHASE_INACTIVE
}

// The full player-facing lifecycle over three concurrent streams: an anonymous
// watcher, the player who joined (secret in hand) and a shared device.
func TestWatchTokenBag_StreamsLifecycleAndScopesTokens(t *testing.T) {
	h := testHandler(t)
	client := watchClient(t, h)
	ctx := context.Background()
	bag := createBagGame(t, h)

	watcher := startWatch(t, client, bag.joinCode, "")
	initial := watcher.next()
	assert.Equal(t, clockkeeperv1.TokenBagPhase_TOKEN_BAG_PHASE_OPEN, initial.Phase)
	assert.Empty(t, initial.Players, "nobody has joined yet")
	assert.Zero(t, initial.SelfRegistrationId, "a watcher without a secret has no self")

	// Joining is public too — no Authorization header on this call either.
	joined, err := client.JoinTokenBag(ctx, connect.NewRequest(&clockkeeperv1.JoinTokenBagRequest{
		JoinCode: bag.joinCode,
		Name:     "Alice",
	}))
	require.NoError(t, err)

	withAlice := watcher.nextMatching("Alice to appear", func(s *clockkeeperv1.WatchTokenBagResponse) bool {
		return len(s.Players) == 1
	})
	assert.Equal(t, "Alice", withAlice.Players[0].Name)

	alice := startWatch(t, client, bag.joinCode, joined.Msg.RegistrationSecret)
	aliceInitial := alice.next()
	assert.Equal(t, joined.Msg.RegistrationId, aliceInitial.SelfRegistrationId)
	assert.Nil(t, aliceInitial.SelfToken, "no token before the reveal")

	shared := startWatch(t, client, bag.sharedCode, "")
	require.NotEmpty(t, shared.next().Players)

	// A player who was removed keeps a secret that resolves to nothing.
	stranger := startWatch(t, client, bag.joinCode, "not-a-real-secret")
	assert.Zero(t, stranger.next().SelfRegistrationId, "an unknown secret must not error, just yield no self")

	setGrimoireNames(t, h, bag.ownerID, bag.gameID, map[string]string{"chef": "Alice"})
	closeBag(t, h, bag)
	revealBag(t, h, bag)

	aliceRevealed := alice.nextMatching("Alice's token", revealedPhase)
	require.NotNil(t, aliceRevealed.SelfToken, "the secret-bearing watcher must receive their token")
	assert.Equal(t, "chef", aliceRevealed.SelfToken.Id)

	sharedRevealed := shared.nextMatching("the reveal on the shared device", revealedPhase)
	assert.Nil(t, sharedRevealed.SelfToken, "a shared-code watcher must never receive a token")
	assert.Zero(t, sharedRevealed.SelfRegistrationId)

	strangerRevealed := stranger.nextMatching("the reveal with a bad secret", revealedPhase)
	assert.Nil(t, strangerRevealed.SelfToken, "a wrong secret must never yield a token")
	assert.Zero(t, strangerRevealed.SelfRegistrationId)

	// A reset undeals the tokens and keeps the codes, so every stream stays up and
	// simply goes back to the closed phase — nobody re-scans anything.
	_, err = h.ResetTokenBag(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.ResetTokenBagRequest{
		GameId: bag.gameID,
	}))
	require.NoError(t, err)

	for name, reader := range map[string]*watchReader{
		"anonymous watcher": watcher,
		"player":            alice,
		"shared device":     shared,
		"stranger":          stranger,
	} {
		afterReset := reader.nextMatching("the reset snapshot of the "+name, closedPhase)
		assert.Len(t, afterReset.Players, 1, "%s keeps seeing the registrants", name)
		assert.Nil(t, afterReset.SelfToken, "%s must not keep seeing a token", name)
		assert.False(t, afterReset.GameStarted, "%s: the game has not started", name)
	}

	// Revealed again, then the game starts: the token goes off the phones for
	// good, without the phase moving at all.
	revealBag(t, h, bag)
	require.NotNil(t, alice.nextMatching("Alice's second token", func(s *clockkeeperv1.WatchTokenBagResponse) bool {
		return s.SelfToken != nil
	}).SelfToken)

	_, err = h.StartGame(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.StartGameRequest{
		GameId: bag.gameID,
	}))
	require.NoError(t, err)

	for name, reader := range map[string]*watchReader{
		"anonymous watcher": watcher,
		"player":            alice,
		"shared device":     shared,
		"stranger":          stranger,
	} {
		started := reader.nextMatching("the started-game snapshot of the "+name, func(s *clockkeeperv1.WatchTokenBagResponse) bool {
			return s.GameStarted
		})
		assert.Equal(t, clockkeeperv1.TokenBagPhase_TOKEN_BAG_PHASE_REVEALED, started.Phase,
			"%s: starting the game does not move the bag phase", name)
		assert.Nil(t, started.SelfToken, "%s must not be shown a token once the game runs", name)
	}
}

func TestWatchTokenBag_EndsWhenGameIsDeleted(t *testing.T) {
	h := testHandler(t)
	client := watchClient(t, h)
	bag := createBagGame(t, h)

	watcher := startWatch(t, client, bag.joinCode, "")
	require.Equal(t, clockkeeperv1.TokenBagPhase_TOKEN_BAG_PHASE_OPEN, watcher.next().Phase)

	_, err := h.DeleteGame(authedCtx(bag.ownerID), connect.NewRequest(&clockkeeperv1.DeleteGameRequest{
		Id: bag.gameID,
	}))
	require.NoError(t, err)

	final := watcher.nextMatching("the final snapshot", inactivePhase)
	assert.Empty(t, final.Players)
	watcher.requireEnded()
}

func TestWatchTokenBag_UnknownCodeIsNotFound(t *testing.T) {
	h := testHandler(t)
	client := watchClient(t, h)

	stream, err := client.WatchTokenBag(context.Background(), connect.NewRequest(&clockkeeperv1.WatchTokenBagRequest{
		Code: "not-a-code",
	}))
	require.NoError(t, err, "connect reports stream errors on the first receive")
	t.Cleanup(func() { _ = stream.Close() })

	require.False(t, stream.Receive())
	require.Error(t, stream.Err())
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(stream.Err()))
}

// Without the heartbeat an idle stream sends nothing for hours, and neither a NAT
// nor the server would notice a client that vanished.
func TestWatchTokenBag_HeartbeatResendsSnapshot(t *testing.T) {
	h := testHandler(t)
	h.watchHeartbeatInterval = 50 * time.Millisecond
	client := watchClient(t, h)
	bag := createBagGame(t, h)

	watcher := startWatch(t, client, bag.joinCode, "")
	first := watcher.next()

	// Nothing changed in between, so this can only be the heartbeat.
	second := watcher.next()
	assert.Equal(t, first.Phase, second.Phase)
	assert.Equal(t, first.GameName, second.GameName)
}

func TestWatchTokenBag_EndsOnServerShutdown(t *testing.T) {
	h := testHandler(t)
	client := watchClient(t, h)
	bag := createBagGame(t, h)

	watcher := startWatch(t, client, bag.joinCode, "")
	require.NotNil(t, watcher.next())

	// What Server.Shutdown does before it waits for in-flight requests.
	h.hub.Close()

	watcher.requireEnded()
}

// --- Self resolution ---

// "No self" and "could not tell" must not look the same: zeroing the self fields
// on a transient failure would make a revealed player's token vanish from their
// screen, exactly as if the storyteller had kicked them.
func TestWatchSelfRegistration_AbsentVersusFailed(t *testing.T) {
	h := testHandler(t)
	ctx := context.Background()
	bag := createBagGame(t, h)
	regID, secret := joinBag(t, h, bag.joinCode, "Alice")

	g, _, err := h.gameByBagCode(ctx, bag.joinCode)
	require.NoError(t, err)

	// A secret from a different game's bag.
	other := createBagGame(t, h)
	_, otherSecret := joinBag(t, h, other.joinCode, "Bob")

	t.Run("own secret resolves", func(t *testing.T) {
		r, err := h.watchSelfRegistration(ctx, g, secret)
		require.NoError(t, err)
		require.NotNil(t, r)
		assert.Equal(t, int(regID), r.ID)
	})

	absent := map[string]string{
		"no secret":           "",
		"unknown secret":      "not-a-real-secret",
		"another game secret": otherSecret,
	}
	for name, s := range absent {
		t.Run(name+" is absent, not an error", func(t *testing.T) {
			r, err := h.watchSelfRegistration(ctx, g, s)
			require.NoError(t, err)
			assert.Nil(t, r)
		})
	}

	t.Run("a cancelled request errors instead of zeroing self", func(t *testing.T) {
		cancelled, cancel := context.WithCancel(ctx)
		cancel()

		r, err := h.watchSelfRegistration(cancelled, g, secret)
		require.Error(t, err, "a watcher that hung up must not look like a kicked player")
		assert.Nil(t, r)
		assert.Equal(t, connect.CodeCanceled, connect.CodeOf(err))
	})

	t.Run("an expired deadline is reported as such", func(t *testing.T) {
		expired, cancel := context.WithDeadline(ctx, time.Now().Add(-time.Second))
		defer cancel()

		_, err := h.watchSelfRegistration(expired, g, secret)
		require.Error(t, err)
		assert.Equal(t, connect.CodeDeadlineExceeded, connect.CodeOf(err))
	})
}

// --- Interceptor chain: which token bag RPCs are public ---

func TestTokenBagRPCs_PublicWithoutAuthHeader(t *testing.T) {
	h := testHandler(t)
	client := watchClient(t, h)
	ctx := context.Background()
	bag := createBagGame(t, h)

	joined, err := client.JoinTokenBag(ctx, connect.NewRequest(&clockkeeperv1.JoinTokenBagRequest{
		JoinCode: bag.joinCode,
		Name:     "Alice",
	}))
	require.NoError(t, err, "joining must work without an Authorization header")

	setGrimoireNames(t, h, bag.ownerID, bag.gameID, map[string]string{"chef": "Alice"})
	closeBag(t, h, bag)
	revealBag(t, h, bag)

	token, err := client.GetMyToken(ctx, connect.NewRequest(&clockkeeperv1.GetMyTokenRequest{
		RegistrationSecret: joined.Msg.RegistrationSecret,
	}))
	require.NoError(t, err, "GetMyToken must work without an Authorization header")
	assert.Equal(t, "chef", token.Msg.Character.Id)

	sharedReveal, err := client.RevealTokenShared(ctx, connect.NewRequest(&clockkeeperv1.RevealTokenSharedRequest{
		SharedCode:     bag.sharedCode,
		RegistrationId: joined.Msg.RegistrationId,
	}))
	require.NoError(t, err, "RevealTokenShared must work without an Authorization header")
	assert.Equal(t, "Alice", sharedReveal.Msg.Name)
}

func TestTokenBagRPCs_StorytellerStillNeedsAuth(t *testing.T) {
	h := testHandler(t)
	client := watchClient(t, h)
	bag := createBagGame(t, h)

	_, err := client.GetTokenBag(context.Background(), connect.NewRequest(&clockkeeperv1.GetTokenBagRequest{
		GameId: bag.gameID,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
