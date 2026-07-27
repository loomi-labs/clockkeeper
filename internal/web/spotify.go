package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Base URLs for the Spotify APIs. Declared as vars so tests can point them at
// an httptest server.
var (
	spotifyAccountsBaseURL = "https://accounts.spotify.com"
	spotifyAPIBaseURL      = "https://api.spotify.com"
)

// spotifyHTTPClient is used for every Spotify call. It must have a timeout:
// token refreshes run while the per-user refresh mutex is held, and
// sync.Mutex.Lock is not context-aware, so a hung request to Spotify would
// block every subsequent token request for that user indefinitely.
var spotifyHTTPClient = &http.Client{Timeout: 10 * time.Second}

// errSpotifyInvalidGrant is returned when Spotify rejects a refresh token.
// The stored connection is unusable and must be re-established by the user.
var errSpotifyInvalidGrant = errors.New("spotify refresh token is no longer valid")

// spotifyTokenResponse is the payload of the Spotify token endpoint. The
// refresh token is only present on the initial exchange and whenever Spotify
// decides to rotate it.
type spotifyTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

// SpotifyProfile represents the current user as returned by GET /v1/me.
type SpotifyProfile struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Product     string `json:"product"`
}

// exchangeSpotifyCode exchanges an authorization code for an access/refresh token pair.
func (h *ClockKeeperServiceHandler) exchangeSpotifyCode(ctx context.Context, code, redirectURI string) (*spotifyTokenResponse, error) {
	data := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {redirectURI},
	}

	token, err := h.spotifyTokenRequest(ctx, data)
	if err != nil {
		return nil, err
	}
	if token.RefreshToken == "" {
		return nil, errors.New("empty refresh token from Spotify")
	}
	return token, nil
}

// refreshSpotifyToken exchanges a refresh token for a fresh access token.
// Spotify may rotate the refresh token, in which case the response carries a
// new one that must be persisted.
func (h *ClockKeeperServiceHandler) refreshSpotifyToken(ctx context.Context, refreshToken string) (*spotifyTokenResponse, error) {
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}
	return h.spotifyTokenRequest(ctx, data)
}

// spotifyTokenRequest posts a form-encoded body to the Spotify token endpoint
// using HTTP Basic client authentication.
func (h *ClockKeeperServiceHandler) spotifyTokenRequest(ctx context.Context, data url.Values) (*spotifyTokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, spotifyAccountsBaseURL+"/api/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	basic := base64.StdEncoding.EncodeToString([]byte(h.config.SpotifyClientID + ":" + h.config.SpotifyClientSecret))
	req.Header.Set("Authorization", "Basic "+basic)

	resp, err := spotifyHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusBadRequest && spotifyErrorCode(body) == "invalid_grant" {
			return nil, errSpotifyInvalidGrant
		}
		return nil, fmt.Errorf("token request failed (status %d): %s", resp.StatusCode, body)
	}

	var token spotifyTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	if token.AccessToken == "" {
		return nil, errors.New("empty access token from Spotify")
	}

	return &token, nil
}

// spotifyErrorCode extracts the top-level "error" field from a Spotify error
// body. Only the flat OAuth shape ({"error":"invalid_grant",...}) is parsed —
// that is the shape the token endpoint uses, and the nested Web API shape
// ({"error":{"status":...}}) never reaches this function. A body in the nested
// shape simply yields "".
func spotifyErrorCode(body []byte) string {
	var flat struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &flat); err == nil {
		return flat.Error
	}
	return ""
}

// fetchSpotifyProfile loads the profile of the user owning the access token.
func (h *ClockKeeperServiceHandler) fetchSpotifyProfile(ctx context.Context, accessToken string) (*SpotifyProfile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, spotifyAPIBaseURL+"/v1/me", nil)
	if err != nil {
		return nil, fmt.Errorf("create profile request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := spotifyHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("profile request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("profile request failed (status %d): %s", resp.StatusCode, body)
	}

	var profile SpotifyProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, fmt.Errorf("decode profile response: %w", err)
	}
	if profile.ID == "" {
		return nil, errors.New("empty user ID from Spotify")
	}

	return &profile, nil
}
