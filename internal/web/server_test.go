package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
