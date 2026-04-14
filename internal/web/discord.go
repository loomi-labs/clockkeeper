package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	discordTokenURL = "https://discord.com/api/oauth2/token"
	discordUserURL  = "https://discord.com/api/users/@me"
)

// DiscordUser represents the user info returned by Discord's API.
type DiscordUser struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	Avatar     string `json:"avatar"`
	GlobalName string `json:"global_name"`
}

// DisplayName returns the best display name for the Discord user.
func (d *DiscordUser) DisplayName() string {
	if d.GlobalName != "" {
		return d.GlobalName
	}
	return d.Username
}

// exchangeDiscordCode exchanges an authorization code for a Discord user.
func (h *ClockKeeperServiceHandler) exchangeDiscordCode(ctx context.Context, code, redirectURI string) (*DiscordUser, error) {
	// Exchange code for access token.
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {h.config.DiscordClientID},
		"client_secret": {h.config.DiscordClientSecret},
	}

	tokenReq, err := http.NewRequestWithContext(ctx, http.MethodPost, discordTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	tokenResp, err := http.DefaultClient.Do(tokenReq)
	if err != nil {
		return nil, fmt.Errorf("token exchange request: %w", err)
	}
	defer tokenResp.Body.Close()

	if tokenResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(tokenResp.Body)
		return nil, fmt.Errorf("token exchange failed (status %d): %s", tokenResp.StatusCode, body)
	}

	var tokenData struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.NewDecoder(tokenResp.Body).Decode(&tokenData); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	if tokenData.AccessToken == "" {
		return nil, errors.New("empty access token from Discord")
	}

	// Fetch user info.
	userReq, err := http.NewRequestWithContext(ctx, http.MethodGet, discordUserURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create user request: %w", err)
	}
	userReq.Header.Set("Authorization", "Bearer "+tokenData.AccessToken)

	userResp, err := http.DefaultClient.Do(userReq)
	if err != nil {
		return nil, fmt.Errorf("user info request: %w", err)
	}
	defer userResp.Body.Close()

	if userResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(userResp.Body)
		return nil, fmt.Errorf("user info failed (status %d): %s", userResp.StatusCode, body)
	}

	var discordUser DiscordUser
	if err := json.NewDecoder(userResp.Body).Decode(&discordUser); err != nil {
		return nil, fmt.Errorf("decode user response: %w", err)
	}
	if discordUser.ID == "" {
		return nil, errors.New("empty user ID from Discord")
	}

	return &discordUser, nil
}
