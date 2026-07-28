package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1/clockkeeperv1connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Spotify panel talks to the Web API straight from the browser and shows
// cover art from Spotify's CDNs — the CSP must allow both, or the feature is
// silently dead on deployed instances (dev bypasses these headers via Vite).
func TestSecurityHeadersAllowSpotify(t *testing.T) {
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	csp := rec.Header().Get("Content-Security-Policy")
	require.NotEmpty(t, csp)

	var connectSrc, imgSrc string
	for _, directive := range strings.Split(csp, ";") {
		directive = strings.TrimSpace(directive)
		switch {
		case strings.HasPrefix(directive, "connect-src "):
			connectSrc = directive
		case strings.HasPrefix(directive, "img-src "):
			imgSrc = directive
		}
	}

	assert.Contains(t, connectSrc, "'self'")
	assert.Contains(t, connectSrc, "https://api.spotify.com")
	assert.Contains(t, imgSrc, "https://*.scdn.co")
	assert.Contains(t, imgSrc, "https://*.spotifycdn.com")
}

// The watch stream stays open for the whole game, so it is the ONE route that
// must not get a write deadline — and every other route must, or the global
// WriteTimeout that used to protect them is simply gone.
func TestApplyWriteDeadline_ExemptsOnlyTheWatchStream(t *testing.T) {
	assert.False(t, applyWriteDeadline(watchTokenBagPath), "the watch stream must stay deadline-free")

	for _, path := range []string{
		"/",
		"/join/abc",
		"/clockkeeper.v1.ClockKeeperService/GetGame",
		"/clockkeeper.v1.ClockKeeperService/GetTokenBag",
		"/clockkeeper.v1.ClockKeeperService/JoinTokenBag",
		// Near misses: the exemption is an exact-path match on purpose.
		watchTokenBagPath + "/",
		strings.TrimSuffix(watchTokenBagPath, "Bag"),
	} {
		assert.True(t, applyWriteDeadline(path), "%s must get a write deadline", path)
	}
}

// The exemption is matched by exact path, so it is only correct as long as it is
// spelled by the generated procedure constant rather than by hand.
func TestWatchTokenBagPath_ComesFromTheGeneratedConstant(t *testing.T) {
	require.Equal(t, clockkeeperv1connect.ClockKeeperServiceWatchTokenBagProcedure, watchTokenBagPath)
	assert.True(t, strings.HasPrefix(watchTokenBagPath, "/"), "procedure paths are absolute: %q", watchTokenBagPath)
	assert.True(t, strings.HasSuffix(watchTokenBagPath, "/WatchTokenBag"), "unexpected procedure path: %q", watchTokenBagPath)
}

// The middleware must be transparent: it decides on a deadline and otherwise
// hands the request straight through, watch stream or not.
func TestPerRequestWriteDeadline_PassesEveryRequestThrough(t *testing.T) {
	var served []string
	handler := perRequestWriteDeadline(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served = append(served, r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, path := range []string{"/clockkeeper.v1.ClockKeeperService/GetGame", watchTokenBagPath} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, nil))
		assert.Equal(t, http.StatusNoContent, rec.Code, "%s", path)
	}
	assert.Equal(t, []string{"/clockkeeper.v1.ClockKeeperService/GetGame", watchTokenBagPath}, served)
}

// A whole table of token bag players shares one per-IP budget, so the default
// has to absorb them all arriving at once. Pins the sizing argument, not the
// number: if the default drops, this says why that hurts.
func TestDefaultAnonRateLimit_AbsorbsOneFullTable(t *testing.T) {
	const phones = 15        // the largest supported game
	const requestsOnJoin = 4 // fetch bag, join, open the stream, and slack

	rl := NewRateLimitInterceptor(defaultRateLimitAnon, 120)
	defer rl.Stop()

	assert.GreaterOrEqual(t, rl.anonBurst, phones*requestsOnJoin,
		"the anon burst must hold a full table scanning the QR code simultaneously")
}
