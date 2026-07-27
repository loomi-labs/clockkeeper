package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/loomi-labs/clockkeeper/ent"
	"github.com/loomi-labs/clockkeeper/ent/spotifyconnection"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSpotifyClientID     = "test-spotify-client-id"
	testSpotifyClientSecret = "test-spotify-client-secret"
	testSpotifyRedirectURI  = "http://127.0.0.1:5173/auth/spotify/callback"
)

// fakeSpotify stands in for the Spotify accounts + web APIs. All response
// knobs are guarded by mu so they can be tweaked between calls.
type fakeSpotify struct {
	mu sync.Mutex

	tokenHits   int
	profileHits int

	lastTokenForm url.Values
	lastTokenAuth string

	// Token endpoint knobs.
	accessToken  string
	refreshToken string // empty => omitted from the response
	expiresIn    int
	tokenStatus  int    // non-zero => respond with this status and tokenErrBody
	tokenErrBody string

	// Profile endpoint knobs.
	profileID      string
	profileName    string
	profileProduct string
}

// startFakeSpotify boots an httptest server serving both the accounts token
// endpoint and /v1/me, and repoints the package base URLs at it.
func startFakeSpotify(t *testing.T) *fakeSpotify {
	t.Helper()

	f := &fakeSpotify{
		accessToken:    "access-token-1",
		refreshToken:   "refresh-token-1",
		expiresIn:      3600,
		profileID:      "spotify-user-1",
		profileName:    "Test Storyteller",
		profileProduct: "premium",
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/token", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))

		f.mu.Lock()
		f.tokenHits++
		f.lastTokenForm = form
		f.lastTokenAuth = r.Header.Get("Authorization")
		status, errBody := f.tokenStatus, f.tokenErrBody
		payload := map[string]any{
			"access_token": f.accessToken,
			"expires_in":   f.expiresIn,
			"scope":        "user-modify-playback-state",
		}
		if f.refreshToken != "" {
			payload["refresh_token"] = f.refreshToken
		}
		f.mu.Unlock()

		if status != 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = io.WriteString(w, errBody)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	})
	mux.HandleFunc("/v1/me", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.profileHits++
		payload := map[string]any{
			"id":           f.profileID,
			"display_name": f.profileName,
			"product":      f.profileProduct,
		}
		f.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	})

	srv := httptest.NewServer(mux)

	// Swapping package-level vars is process-global: no test in this package may call t.Parallel().
	prevAccounts, prevAPI := spotifyAccountsBaseURL, spotifyAPIBaseURL
	spotifyAccountsBaseURL = srv.URL
	spotifyAPIBaseURL = srv.URL
	t.Cleanup(func() {
		spotifyAccountsBaseURL = prevAccounts
		spotifyAPIBaseURL = prevAPI
		srv.Close()
	})

	return f
}

func (f *fakeSpotify) hits() (tokenHits, profileHits int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tokenHits, f.profileHits
}

func (f *fakeSpotify) resetHits() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokenHits, f.profileHits = 0, 0
}

func (f *fakeSpotify) set(fn func(f *fakeSpotify)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(f)
}

// spotifyHandler returns a test handler with Spotify OAuth configured.
func spotifyHandler(t *testing.T) *ClockKeeperServiceHandler {
	t.Helper()
	h := testHandler(t)
	h.config.SpotifyClientID = testSpotifyClientID
	h.config.SpotifyClientSecret = testSpotifyClientSecret
	h.config.SpotifyRedirectURI = testSpotifyRedirectURI
	return h
}

// connectSpotify links a user through the fake Spotify server.
func connectSpotify(t *testing.T, h *ClockKeeperServiceHandler, userID int) *clockkeeperv1.SpotifyStatus {
	t.Helper()
	resp, err := h.ConnectSpotify(authedCtx(userID), connect.NewRequest(&clockkeeperv1.ConnectSpotifyRequest{
		Code:        "auth-code",
		RedirectUri: testSpotifyRedirectURI,
	}))
	require.NoError(t, err)
	return resp.Msg.Status
}

// storedConnection loads a user's connection row directly from the DB.
func storedConnection(t *testing.T, h *ClockKeeperServiceHandler, userID int) *ent.SpotifyConnection {
	t.Helper()
	conn, err := h.db.SpotifyConnection.Query().
		Where(spotifyconnection.UserID(userID)).
		Only(context.Background())
	require.NoError(t, err)
	return conn
}

func newUser(t *testing.T, h *ClockKeeperServiceHandler) *ent.User {
	t.Helper()
	u, err := h.db.User.Create().Save(context.Background())
	require.NoError(t, err)
	return u
}

// --- ConnectSpotify ---

func TestConnectSpotify_NotConfigured(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := testHandler(t) // no Spotify config
	u := newUser(t, handler)

	_, err := handler.ConnectSpotify(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.ConnectSpotifyRequest{
		Code:        "auth-code",
		RedirectUri: testSpotifyRedirectURI,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestConnectSpotify_RejectsBadArguments(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := spotifyHandler(t)
	startFakeSpotify(t)
	u := newUser(t, handler)

	tests := []struct {
		name        string
		code        string
		redirectURI string
	}{
		{"empty code", "", testSpotifyRedirectURI},
		{"empty redirect uri", "auth-code", ""},
		{"redirect uri mismatch", "auth-code", "https://evil.example/callback"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := handler.ConnectSpotify(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.ConnectSpotifyRequest{
				Code:        tc.code,
				RedirectUri: tc.redirectURI,
			}))
			require.Error(t, err)
			assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
		})
	}
}

func TestConnectSpotify_StoresConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := spotifyHandler(t)
	fake := startFakeSpotify(t)
	u := newUser(t, handler)

	before := time.Now()
	status := connectSpotify(t, handler, u.ID)

	assert.True(t, status.Connected)
	assert.Equal(t, "Test Storyteller", status.DisplayName)
	assert.True(t, status.Premium)
	assert.Nil(t, status.Day)
	assert.Nil(t, status.Night)
	assert.Nil(t, status.Nominations)

	conn := storedConnection(t, handler, u.ID)
	assert.Equal(t, "spotify-user-1", conn.SpotifyUserID)
	assert.Equal(t, "refresh-token-1", conn.RefreshToken)
	assert.Equal(t, "access-token-1", conn.AccessToken)
	assert.Equal(t, u.ID, conn.UserID)
	require.NotNil(t, conn.AccessTokenExpiresAt)
	assert.WithinDuration(t, before.Add(3600*time.Second), *conn.AccessTokenExpiresAt, 30*time.Second)

	// The exchange used the authorization_code grant with Basic client auth.
	fake.mu.Lock()
	form, auth := fake.lastTokenForm, fake.lastTokenAuth
	fake.mu.Unlock()
	assert.Equal(t, "authorization_code", form.Get("grant_type"))
	assert.Equal(t, "auth-code", form.Get("code"))
	assert.Equal(t, testSpotifyRedirectURI, form.Get("redirect_uri"))
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(testSpotifyClientID+":"+testSpotifyClientSecret))
	assert.Equal(t, wantAuth, auth)

	tokenHits, profileHits := fake.hits()
	assert.Equal(t, 1, tokenHits)
	assert.Equal(t, 1, profileHits)
}

func TestConnectSpotify_NonPremiumAccount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := spotifyHandler(t)
	fake := startFakeSpotify(t)
	fake.set(func(f *fakeSpotify) { f.profileProduct = "free" })
	u := newUser(t, handler)

	status := connectSpotify(t, handler, u.ID)
	assert.True(t, status.Connected)
	assert.False(t, status.Premium)
}

func TestConnectSpotify_ReplacesExistingConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := spotifyHandler(t)
	fake := startFakeSpotify(t)
	u := newUser(t, handler)

	connectSpotify(t, handler, u.ID)

	// Configure a playlist so we can prove the row was really replaced.
	_, err := handler.UpdateSpotifyPlaylists(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.UpdateSpotifyPlaylistsRequest{
		Day: &clockkeeperv1.SpotifyPlaylistSlot{Uri: "spotify:playlist:day", Name: "Day"},
	}))
	require.NoError(t, err)

	// Reconnect with a different Spotify account.
	fake.set(func(f *fakeSpotify) {
		f.accessToken = "access-token-2"
		f.refreshToken = "refresh-token-2"
		f.profileID = "spotify-user-2"
		f.profileName = "Second Account"
		f.profileProduct = "free"
	})
	status := connectSpotify(t, handler, u.ID)

	assert.Equal(t, "Second Account", status.DisplayName)
	assert.False(t, status.Premium)
	assert.Nil(t, status.Day, "playlists belong to the replaced connection")

	count, err := handler.db.SpotifyConnection.Query().Where(spotifyconnection.UserID(u.ID)).Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, count, "expected exactly one connection row per user")

	conn := storedConnection(t, handler, u.ID)
	assert.Equal(t, "spotify-user-2", conn.SpotifyUserID)
	assert.Equal(t, "refresh-token-2", conn.RefreshToken)
	assert.Equal(t, "access-token-2", conn.AccessToken)
}

// --- GetSpotifyStatus ---

func TestGetSpotifyStatus_NotConnected(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := spotifyHandler(t)
	u := newUser(t, handler)

	resp, err := handler.GetSpotifyStatus(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.GetSpotifyStatusRequest{}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Status)
	assert.False(t, resp.Msg.Status.Connected)
	assert.Empty(t, resp.Msg.Status.DisplayName)
}

// --- GetSpotifyAccessToken ---

func TestGetSpotifyAccessToken_NotConnected(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := spotifyHandler(t)
	u := newUser(t, handler)

	_, err := handler.GetSpotifyAccessToken(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.GetSpotifyAccessTokenRequest{}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	assert.Contains(t, err.Error(), "spotify not connected")
}

func TestGetSpotifyAccessToken_UsesCachedToken(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := spotifyHandler(t)
	fake := startFakeSpotify(t)
	u := newUser(t, handler)

	connectSpotify(t, handler, u.ID)
	fake.resetHits()

	resp, err := handler.GetSpotifyAccessToken(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.GetSpotifyAccessTokenRequest{}))
	require.NoError(t, err)
	assert.Equal(t, "access-token-1", resp.Msg.AccessToken)
	assert.Greater(t, resp.Msg.ExpiresAtUnix, time.Now().Unix())

	tokenHits, _ := fake.hits()
	assert.Equal(t, 0, tokenHits, "a fresh token must be served from the database")
}

func TestGetSpotifyAccessToken_RefreshesExpiredToken(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := spotifyHandler(t)
	fake := startFakeSpotify(t)
	ctx := context.Background()
	u := newUser(t, handler)

	connectSpotify(t, handler, u.ID)
	conn := storedConnection(t, handler, u.ID)

	// Expire the cached token.
	_, err := handler.db.SpotifyConnection.UpdateOneID(conn.ID).
		SetAccessTokenExpiresAt(time.Now().Add(-time.Minute)).
		Save(ctx)
	require.NoError(t, err)

	fake.resetHits()
	fake.set(func(f *fakeSpotify) {
		f.accessToken = "access-token-refreshed"
		f.refreshToken = "" // no rotation this time
	})

	resp, err := handler.GetSpotifyAccessToken(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.GetSpotifyAccessTokenRequest{}))
	require.NoError(t, err)
	assert.Equal(t, "access-token-refreshed", resp.Msg.AccessToken)
	assert.Greater(t, resp.Msg.ExpiresAtUnix, time.Now().Unix())

	tokenHits, _ := fake.hits()
	assert.Equal(t, 1, tokenHits)

	fake.mu.Lock()
	form := fake.lastTokenForm
	fake.mu.Unlock()
	assert.Equal(t, "refresh_token", form.Get("grant_type"))
	assert.Equal(t, "refresh-token-1", form.Get("refresh_token"))

	// The new access token and expiry are persisted, the refresh token kept.
	updated := storedConnection(t, handler, u.ID)
	assert.Equal(t, "access-token-refreshed", updated.AccessToken)
	assert.Equal(t, "refresh-token-1", updated.RefreshToken)
	require.NotNil(t, updated.AccessTokenExpiresAt)
	assert.True(t, updated.AccessTokenExpiresAt.After(time.Now().Add(spotifyTokenRefreshSkew)))

	// The refreshed token is now cached — a second call must not hit Spotify.
	fake.resetHits()
	_, err = handler.GetSpotifyAccessToken(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.GetSpotifyAccessTokenRequest{}))
	require.NoError(t, err)
	tokenHits, _ = fake.hits()
	assert.Equal(t, 0, tokenHits)
}

func TestGetSpotifyAccessToken_PersistsRotatedRefreshToken(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := spotifyHandler(t)
	fake := startFakeSpotify(t)
	ctx := context.Background()
	u := newUser(t, handler)

	connectSpotify(t, handler, u.ID)
	conn := storedConnection(t, handler, u.ID)

	_, err := handler.db.SpotifyConnection.UpdateOneID(conn.ID).
		SetAccessTokenExpiresAt(time.Now().Add(-time.Minute)).
		Save(ctx)
	require.NoError(t, err)

	fake.set(func(f *fakeSpotify) {
		f.accessToken = "access-token-rotated"
		f.refreshToken = "refresh-token-rotated"
	})

	_, err = handler.GetSpotifyAccessToken(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.GetSpotifyAccessTokenRequest{}))
	require.NoError(t, err)

	updated := storedConnection(t, handler, u.ID)
	assert.Equal(t, "refresh-token-rotated", updated.RefreshToken, "rotated refresh token must be persisted")
	assert.Equal(t, "access-token-rotated", updated.AccessToken)
}

func TestGetSpotifyAccessToken_InvalidGrantDropsConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := spotifyHandler(t)
	fake := startFakeSpotify(t)
	ctx := context.Background()
	u := newUser(t, handler)

	connectSpotify(t, handler, u.ID)
	conn := storedConnection(t, handler, u.ID)

	_, err := handler.db.SpotifyConnection.UpdateOneID(conn.ID).
		SetAccessTokenExpiresAt(time.Now().Add(-time.Minute)).
		Save(ctx)
	require.NoError(t, err)

	fake.set(func(f *fakeSpotify) {
		f.tokenStatus = http.StatusBadRequest
		f.tokenErrBody = `{"error":"invalid_grant","error_description":"Refresh token revoked"}`
	})

	_, err = handler.GetSpotifyAccessToken(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.GetSpotifyAccessTokenRequest{}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))

	count, err := handler.db.SpotifyConnection.Query().Where(spotifyconnection.UserID(u.ID)).Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, count, "an invalid_grant must drop the stored connection")

	// Status reflects the dropped connection.
	statusResp, err := handler.GetSpotifyStatus(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.GetSpotifyStatusRequest{}))
	require.NoError(t, err)
	assert.False(t, statusResp.Msg.Status.Connected)
}

func TestGetSpotifyAccessToken_ForceRefreshesFreshToken(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := spotifyHandler(t)
	fake := startFakeSpotify(t)
	ctx := context.Background()
	u := newUser(t, handler)

	connectSpotify(t, handler, u.ID)
	conn := storedConnection(t, handler, u.ID)

	// Half the lifetime left: comfortably "cached" for a normal call, and old
	// enough that a forced call must not treat it as just minted.
	_, err := handler.db.SpotifyConnection.UpdateOneID(conn.ID).
		SetAccessTokenExpiresAt(time.Now().Add(30 * time.Minute)).
		Save(ctx)
	require.NoError(t, err)

	fake.resetHits()
	fake.set(func(f *fakeSpotify) {
		f.accessToken = "access-token-forced"
		f.refreshToken = ""
	})

	// Sanity check: without force the cached token is served untouched.
	resp, err := handler.GetSpotifyAccessToken(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.GetSpotifyAccessTokenRequest{}))
	require.NoError(t, err)
	assert.Equal(t, "access-token-1", resp.Msg.AccessToken)
	tokenHits, _ := fake.hits()
	assert.Equal(t, 0, tokenHits)

	before := time.Now()
	resp, err = handler.GetSpotifyAccessToken(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.GetSpotifyAccessTokenRequest{
		ForceRefresh: true,
	}))
	require.NoError(t, err)
	assert.Equal(t, "access-token-forced", resp.Msg.AccessToken)

	tokenHits, _ = fake.hits()
	assert.Equal(t, 1, tokenHits, "force must refresh exactly once even with a fresh cached token")

	fake.mu.Lock()
	form := fake.lastTokenForm
	fake.mu.Unlock()
	assert.Equal(t, "refresh_token", form.Get("grant_type"))
	assert.Equal(t, "refresh-token-1", form.Get("refresh_token"))

	updated := storedConnection(t, handler, u.ID)
	assert.Equal(t, "access-token-forced", updated.AccessToken)
	require.NotNil(t, updated.AccessTokenExpiresAt)
	assert.WithinDuration(t, before.Add(3600*time.Second), *updated.AccessTokenExpiresAt, 30*time.Second)
	assert.Equal(t, updated.AccessTokenExpiresAt.Unix(), resp.Msg.ExpiresAtUnix)
}

func TestGetSpotifyAccessToken_ForceInvalidGrantDropsConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := spotifyHandler(t)
	fake := startFakeSpotify(t)
	ctx := context.Background()
	u := newUser(t, handler)

	connectSpotify(t, handler, u.ID)
	conn := storedConnection(t, handler, u.ID)

	// The stored token still looks usable — only the forced refresh can
	// discover that the grant was revoked on Spotify's side.
	_, err := handler.db.SpotifyConnection.UpdateOneID(conn.ID).
		SetAccessTokenExpiresAt(time.Now().Add(30 * time.Minute)).
		Save(ctx)
	require.NoError(t, err)

	fake.resetHits()
	fake.set(func(f *fakeSpotify) {
		f.tokenStatus = http.StatusBadRequest
		f.tokenErrBody = `{"error":"invalid_grant","error_description":"Refresh token revoked"}`
	})

	_, err = handler.GetSpotifyAccessToken(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.GetSpotifyAccessTokenRequest{
		ForceRefresh: true,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))

	tokenHits, _ := fake.hits()
	assert.Equal(t, 1, tokenHits)

	count, err := handler.db.SpotifyConnection.Query().Where(spotifyconnection.UserID(u.ID)).Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, count, "a revoked grant must drop the stored connection")

	statusResp, err := handler.GetSpotifyStatus(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.GetSpotifyStatusRequest{}))
	require.NoError(t, err)
	assert.False(t, statusResp.Msg.Status.Connected)
}

func TestGetSpotifyAccessToken_ForceReusesJustMintedToken(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := spotifyHandler(t)
	fake := startFakeSpotify(t)
	u := newUser(t, handler)

	// Straight after ConnectSpotify the stored token expires in ~60 minutes,
	// i.e. it was just minted — a forced refresh must dedup onto it.
	connectSpotify(t, handler, u.ID)
	fake.resetHits()

	resp, err := handler.GetSpotifyAccessToken(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.GetSpotifyAccessTokenRequest{
		ForceRefresh: true,
	}))
	require.NoError(t, err)
	assert.Equal(t, "access-token-1", resp.Msg.AccessToken)

	tokenHits, _ := fake.hits()
	assert.Equal(t, 0, tokenHits, "a just-minted token means a concurrent force already refreshed")
}

func TestGetSpotifyAccessToken_PerUserIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := spotifyHandler(t)
	fake := startFakeSpotify(t)
	userA := newUser(t, handler)
	userB := newUser(t, handler)

	connectSpotify(t, handler, userA.ID)

	// User B is not connected yet.
	_, err := handler.GetSpotifyAccessToken(authedCtx(userB.ID), connect.NewRequest(&clockkeeperv1.GetSpotifyAccessTokenRequest{}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))

	// B links a different account.
	fake.set(func(f *fakeSpotify) {
		f.accessToken = "access-token-b"
		f.refreshToken = "refresh-token-b"
		f.profileID = "spotify-user-b"
		f.profileName = "User B"
	})
	connectSpotify(t, handler, userB.ID)

	respA, err := handler.GetSpotifyAccessToken(authedCtx(userA.ID), connect.NewRequest(&clockkeeperv1.GetSpotifyAccessTokenRequest{}))
	require.NoError(t, err)
	assert.Equal(t, "access-token-1", respA.Msg.AccessToken)

	respB, err := handler.GetSpotifyAccessToken(authedCtx(userB.ID), connect.NewRequest(&clockkeeperv1.GetSpotifyAccessTokenRequest{}))
	require.NoError(t, err)
	assert.Equal(t, "access-token-b", respB.Msg.AccessToken)

	statusA, err := handler.GetSpotifyStatus(authedCtx(userA.ID), connect.NewRequest(&clockkeeperv1.GetSpotifyStatusRequest{}))
	require.NoError(t, err)
	assert.Equal(t, "Test Storyteller", statusA.Msg.Status.DisplayName)
}

// --- UpdateSpotifyPlaylists ---

func TestUpdateSpotifyPlaylists_NotConnected(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := spotifyHandler(t)
	u := newUser(t, handler)

	_, err := handler.UpdateSpotifyPlaylists(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.UpdateSpotifyPlaylistsRequest{
		Day: &clockkeeperv1.SpotifyPlaylistSlot{Uri: "spotify:playlist:abc", Name: "Day"},
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestUpdateSpotifyPlaylists_SetsAndClearsSlots(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := spotifyHandler(t)
	startFakeSpotify(t)
	u := newUser(t, handler)
	connectSpotify(t, handler, u.ID)

	resp, err := handler.UpdateSpotifyPlaylists(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.UpdateSpotifyPlaylistsRequest{
		Day:         &clockkeeperv1.SpotifyPlaylistSlot{Uri: "spotify:playlist:day1", Name: "Daytime", ImageUrl: "https://img.example/day.jpg"},
		Night:       &clockkeeperv1.SpotifyPlaylistSlot{Uri: "spotify:playlist:night1", Name: "Nighttime"},
		Nominations: &clockkeeperv1.SpotifyPlaylistSlot{Uri: "spotify:playlist:nom1", Name: "Nominations"},
	}))
	require.NoError(t, err)

	status := resp.Msg.Status
	require.NotNil(t, status.Day)
	assert.Equal(t, "spotify:playlist:day1", status.Day.Uri)
	assert.Equal(t, "Daytime", status.Day.Name)
	assert.Equal(t, "https://img.example/day.jpg", status.Day.ImageUrl)
	require.NotNil(t, status.Night)
	assert.Equal(t, "spotify:playlist:night1", status.Night.Uri)
	require.NotNil(t, status.Nominations)
	assert.Equal(t, "spotify:playlist:nom1", status.Nominations.Uri)

	// Persisted across reads.
	statusResp, err := handler.GetSpotifyStatus(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.GetSpotifyStatusRequest{}))
	require.NoError(t, err)
	require.NotNil(t, statusResp.Msg.Status.Night)
	assert.Equal(t, "Nighttime", statusResp.Msg.Status.Night.Name)

	// A request that only sets night clears the other two slots.
	resp, err = handler.UpdateSpotifyPlaylists(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.UpdateSpotifyPlaylistsRequest{
		Night: &clockkeeperv1.SpotifyPlaylistSlot{Uri: "spotify:playlist:night2", Name: "Nighttime 2"},
	}))
	require.NoError(t, err)
	assert.Nil(t, resp.Msg.Status.Day)
	assert.Nil(t, resp.Msg.Status.Nominations)
	require.NotNil(t, resp.Msg.Status.Night)
	assert.Equal(t, "spotify:playlist:night2", resp.Msg.Status.Night.Uri)

	// An empty request clears everything.
	resp, err = handler.UpdateSpotifyPlaylists(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.UpdateSpotifyPlaylistsRequest{}))
	require.NoError(t, err)
	assert.Nil(t, resp.Msg.Status.Day)
	assert.Nil(t, resp.Msg.Status.Night)
	assert.Nil(t, resp.Msg.Status.Nominations)
	assert.True(t, resp.Msg.Status.Connected)
}

func TestUpdateSpotifyPlaylists_RejectsInvalidInput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := spotifyHandler(t)
	startFakeSpotify(t)
	u := newUser(t, handler)
	connectSpotify(t, handler, u.ID)

	tests := []struct {
		name string
		req  *clockkeeperv1.UpdateSpotifyPlaylistsRequest
	}{
		{
			name: "album uri",
			req: &clockkeeperv1.UpdateSpotifyPlaylistsRequest{
				Day: &clockkeeperv1.SpotifyPlaylistSlot{Uri: "spotify:album:abc", Name: "Album"},
			},
		},
		{
			name: "http url",
			req: &clockkeeperv1.UpdateSpotifyPlaylistsRequest{
				Night: &clockkeeperv1.SpotifyPlaylistSlot{Uri: "https://open.spotify.com/playlist/abc", Name: "Link"},
			},
		},
		{
			name: "empty uri",
			req: &clockkeeperv1.UpdateSpotifyPlaylistsRequest{
				Nominations: &clockkeeperv1.SpotifyPlaylistSlot{Uri: "", Name: "Nothing"},
			},
		},
		{
			name: "bare prefix without id",
			req: &clockkeeperv1.UpdateSpotifyPlaylistsRequest{
				Day: &clockkeeperv1.SpotifyPlaylistSlot{Uri: spotifyPlaylistURIPrefix, Name: "Bare"},
			},
		},
		{
			name: "name too long",
			req: &clockkeeperv1.UpdateSpotifyPlaylistsRequest{
				Day: &clockkeeperv1.SpotifyPlaylistSlot{
					Uri:  "spotify:playlist:abc",
					Name: strings.Repeat("a", maxSpotifyPlaylistNameLen+1),
				},
			},
		},
		{
			name: "uri too long",
			req: &clockkeeperv1.UpdateSpotifyPlaylistsRequest{
				Night: &clockkeeperv1.SpotifyPlaylistSlot{
					Uri:  spotifyPlaylistURIPrefix + strings.Repeat("a", maxSpotifyPlaylistURLLen),
					Name: "Long uri",
				},
			},
		},
		{
			name: "image url too long",
			req: &clockkeeperv1.UpdateSpotifyPlaylistsRequest{
				Nominations: &clockkeeperv1.SpotifyPlaylistSlot{
					Uri:      "spotify:playlist:abc",
					Name:     "Long image",
					ImageUrl: "https://img/" + strings.Repeat("a", maxSpotifyPlaylistURLLen),
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := handler.UpdateSpotifyPlaylists(authedCtx(u.ID), connect.NewRequest(tc.req))
			require.Error(t, err)
			assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
		})
	}

	// Nothing was written by the rejected requests.
	statusResp, err := handler.GetSpotifyStatus(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.GetSpotifyStatusRequest{}))
	require.NoError(t, err)
	assert.Nil(t, statusResp.Msg.Status.Day)
	assert.Nil(t, statusResp.Msg.Status.Night)
	assert.Nil(t, statusResp.Msg.Status.Nominations)
}

// --- DisconnectSpotify ---

func TestDisconnectSpotify_DeletesRowAndIsIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := spotifyHandler(t)
	startFakeSpotify(t)
	ctx := context.Background()
	u := newUser(t, handler)
	connectSpotify(t, handler, u.ID)

	_, err := handler.DisconnectSpotify(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.DisconnectSpotifyRequest{}))
	require.NoError(t, err)

	count, err := handler.db.SpotifyConnection.Query().Where(spotifyconnection.UserID(u.ID)).Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, count)

	// Second call is a no-op, not an error.
	_, err = handler.DisconnectSpotify(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.DisconnectSpotifyRequest{}))
	require.NoError(t, err)

	statusResp, err := handler.GetSpotifyStatus(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.GetSpotifyStatusRequest{}))
	require.NoError(t, err)
	assert.False(t, statusResp.Msg.Status.Connected)
}

func TestDisconnectSpotify_OnlyAffectsCaller(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := spotifyHandler(t)
	startFakeSpotify(t)
	userA := newUser(t, handler)
	userB := newUser(t, handler)
	connectSpotify(t, handler, userA.ID)
	connectSpotify(t, handler, userB.ID)

	_, err := handler.DisconnectSpotify(authedCtx(userB.ID), connect.NewRequest(&clockkeeperv1.DisconnectSpotifyRequest{}))
	require.NoError(t, err)

	statusA, err := handler.GetSpotifyStatus(authedCtx(userA.ID), connect.NewRequest(&clockkeeperv1.GetSpotifyStatusRequest{}))
	require.NoError(t, err)
	assert.True(t, statusA.Msg.Status.Connected)
}

// --- GetAuthConfig ---

func TestGetAuthConfig_SpotifyClientIDOnlyWhenConfigured(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := testHandler(t)
	handler.config.SpotifyClientID = testSpotifyClientID // secret + redirect still missing

	resp, err := handler.GetAuthConfig(context.Background(), connect.NewRequest(&clockkeeperv1.GetAuthConfigRequest{}))
	require.NoError(t, err)
	assert.Empty(t, resp.Msg.SpotifyClientId, "partial config must not advertise the client id")

	handler.config.SpotifyClientSecret = testSpotifyClientSecret
	handler.config.SpotifyRedirectURI = testSpotifyRedirectURI

	resp, err = handler.GetAuthConfig(context.Background(), connect.NewRequest(&clockkeeperv1.GetAuthConfigRequest{}))
	require.NoError(t, err)
	assert.Equal(t, testSpotifyClientID, resp.Msg.SpotifyClientId)
}

// --- Cleanup cascade ---

func TestDeleteUserCascade_DeletesSpotifyConnectionAndInfoCards(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := spotifyHandler(t)
	startFakeSpotify(t)
	ctx := context.Background()
	u := newUser(t, handler)

	connectSpotify(t, handler, u.ID)
	_, err := handler.CreateInfoCard(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.CreateInfoCardRequest{
		Title: "Card to clean up",
	}))
	require.NoError(t, err)

	require.NoError(t, deleteUserCascade(ctx, handler.db, u.ID))

	_, err = handler.db.User.Get(ctx, u.ID)
	require.Error(t, err)
	assert.True(t, ent.IsNotFound(err))

	connCount, err := handler.db.SpotifyConnection.Query().Where(spotifyconnection.UserID(u.ID)).Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, connCount)

	cardCount, err := handler.db.InfoCard.Query().Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, cardCount)
}
