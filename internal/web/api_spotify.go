package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/loomi-labs/clockkeeper/ent"
	"github.com/loomi-labs/clockkeeper/ent/schema"
	"github.com/loomi-labs/clockkeeper/ent/spotifyconnection"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
)

const (
	// spotifyPlaylistURIPrefix is the only URI shape the panel can play.
	spotifyPlaylistURIPrefix = "spotify:playlist:"
	// maxSpotifyPlaylistNameLen caps the snapshotted playlist name.
	maxSpotifyPlaylistNameLen = 200
	// maxSpotifyPlaylistURLLen caps the stored uri and image url.
	maxSpotifyPlaylistURLLen = 512
	// spotifyTokenRefreshSkew is how long before expiry a cached access token
	// is considered stale and gets refreshed.
	spotifyTokenRefreshSkew = 5 * time.Minute
	// spotifyTokenFreshMintWindow is how much validity a stored token must have
	// left to count as "just minted". A forced refresh that reloads such a token
	// assumes a concurrent forced refresh already did the work and reuses it.
	spotifyTokenFreshMintWindow = 55 * time.Minute
)

var errSpotifyNotConnected = errors.New("spotify not connected")

// spotifyConnectionFor loads the current user's Spotify connection, or nil if
// none exists.
func (h *ClockKeeperServiceHandler) spotifyConnectionFor(ctx context.Context, userID int) (*ent.SpotifyConnection, error) {
	conn, err := h.db.SpotifyConnection.Query().
		Where(spotifyconnection.UserID(userID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		slog.Error("spotify connection lookup failed", "user_id", userID, "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}
	return conn, nil
}

func (h *ClockKeeperServiceHandler) ConnectSpotify(ctx context.Context, req *connect.Request[clockkeeperv1.ConnectSpotifyRequest]) (*connect.Response[clockkeeperv1.ConnectSpotifyResponse], error) {
	if !h.config.SpotifyConfigured() {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("Spotify is not configured"))
	}

	if req.Msg.Code == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("code is required"))
	}
	if req.Msg.RedirectUri == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("redirect_uri is required"))
	}
	if req.Msg.RedirectUri != h.config.SpotifyRedirectURI {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("redirect_uri mismatch"))
	}

	u, err := h.currentUser(ctx)
	if err != nil {
		return nil, err
	}

	token, err := h.exchangeSpotifyCode(ctx, req.Msg.Code, req.Msg.RedirectUri)
	if err != nil {
		slog.Error("spotify code exchange failed", "err", err)
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("Spotify authentication failed"))
	}

	profile, err := h.fetchSpotifyProfile(ctx, token.AccessToken)
	if err != nil {
		slog.Error("spotify profile fetch failed", "err", err)
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("Spotify authentication failed"))
	}

	// Replace any existing connection — a re-connect always wins, and the
	// playlist slots are re-configured against the newly linked account. The
	// delete and the create share a transaction so a failure can't leave the
	// user with no connection at all.
	tx, err := h.db.Tx(ctx)
	if err != nil {
		slog.Error("start transaction failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	if _, err := tx.SpotifyConnection.Delete().
		Where(spotifyconnection.UserID(u.ID)).
		Exec(ctx); err != nil {
		_ = tx.Rollback()
		slog.Error("delete existing spotify connection failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	conn, err := tx.SpotifyConnection.Create().
		SetUserID(u.ID).
		SetSpotifyUserID(profile.ID).
		SetDisplayName(profile.DisplayName).
		SetPremium(profile.Product == "premium").
		SetRefreshToken(token.RefreshToken).
		SetAccessToken(token.AccessToken).
		SetAccessTokenExpiresAt(time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		slog.Error("create spotify connection failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	if err := tx.Commit(); err != nil {
		slog.Error("commit spotify connection failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.ConnectSpotifyResponse{
		Status: spotifyStatusToProto(conn),
	}), nil
}

func (h *ClockKeeperServiceHandler) DisconnectSpotify(ctx context.Context, req *connect.Request[clockkeeperv1.DisconnectSpotifyRequest]) (*connect.Response[clockkeeperv1.DisconnectSpotifyResponse], error) {
	u, err := h.currentUser(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := h.db.SpotifyConnection.Delete().
		Where(spotifyconnection.UserID(u.ID)).
		Exec(ctx); err != nil {
		slog.Error("delete spotify connection failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.DisconnectSpotifyResponse{}), nil
}

func (h *ClockKeeperServiceHandler) GetSpotifyStatus(ctx context.Context, req *connect.Request[clockkeeperv1.GetSpotifyStatusRequest]) (*connect.Response[clockkeeperv1.GetSpotifyStatusResponse], error) {
	u, err := h.currentUser(ctx)
	if err != nil {
		return nil, err
	}

	conn, err := h.spotifyConnectionFor(ctx, u.ID)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&clockkeeperv1.GetSpotifyStatusResponse{
		Status: spotifyStatusToProto(conn),
	}), nil
}

func (h *ClockKeeperServiceHandler) UpdateSpotifyPlaylists(ctx context.Context, req *connect.Request[clockkeeperv1.UpdateSpotifyPlaylistsRequest]) (*connect.Response[clockkeeperv1.UpdateSpotifyPlaylistsResponse], error) {
	u, err := h.currentUser(ctx)
	if err != nil {
		return nil, err
	}

	conn, err := h.spotifyConnectionFor(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errSpotifyNotConnected)
	}

	day, err := validateSpotifyPlaylistSlot(req.Msg.Day, "day")
	if err != nil {
		return nil, err
	}
	night, err := validateSpotifyPlaylistSlot(req.Msg.Night, "night")
	if err != nil {
		return nil, err
	}
	nominations, err := validateSpotifyPlaylistSlot(req.Msg.Nominations, "nominations")
	if err != nil {
		return nil, err
	}

	// The request replaces all three slots; an absent message clears one.
	update := h.db.SpotifyConnection.UpdateOneID(conn.ID)
	if day != nil {
		update = update.SetDayPlaylist(day)
	} else {
		update = update.ClearDayPlaylist()
	}
	if night != nil {
		update = update.SetNightPlaylist(night)
	} else {
		update = update.ClearNightPlaylist()
	}
	if nominations != nil {
		update = update.SetNominationsPlaylist(nominations)
	} else {
		update = update.ClearNominationsPlaylist()
	}

	conn, err = update.Save(ctx)
	if err != nil {
		slog.Error("update spotify playlists failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.UpdateSpotifyPlaylistsResponse{
		Status: spotifyStatusToProto(conn),
	}), nil
}

func (h *ClockKeeperServiceHandler) GetSpotifyAccessToken(ctx context.Context, req *connect.Request[clockkeeperv1.GetSpotifyAccessTokenRequest]) (*connect.Response[clockkeeperv1.GetSpotifyAccessTokenResponse], error) {
	u, err := h.currentUser(ctx)
	if err != nil {
		return nil, err
	}

	conn, err := h.spotifyConnectionFor(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errSpotifyNotConnected)
	}

	token, expiresAt, err := h.ensureSpotifyAccessToken(ctx, conn, req.Msg.ForceRefresh)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&clockkeeperv1.GetSpotifyAccessTokenResponse{
		AccessToken:   token,
		ExpiresAtUnix: expiresAt.Unix(),
	}), nil
}

// ensureSpotifyAccessToken returns a usable access token for the connection,
// refreshing it when the cached one is missing or about to expire.
//
// force skips the freshness check: the caller has seen Spotify reject the
// cached token with a 401, so the token must be re-minted even though the
// stored expiry still looks fine. This is also how a revoked grant is
// discovered — the refresh fails with invalid_grant and the connection is
// dropped, instead of the client dead-ending on a token nobody re-validates.
func (h *ClockKeeperServiceHandler) ensureSpotifyAccessToken(ctx context.Context, conn *ent.SpotifyConnection, force bool) (string, time.Time, error) {
	if !force {
		if token, expiresAt, ok := cachedSpotifyToken(conn); ok {
			return token, expiresAt, nil
		}
	}

	// Serialize refreshes per user: Spotify rotates refresh tokens, so two
	// concurrent refreshes would race and one of them could persist a token
	// that has already been invalidated.
	mu := h.spotifyUserLock(conn.UserID)
	mu.Lock()
	defer mu.Unlock()

	// Another request may have refreshed while we waited for the lock. The
	// re-load also guarantees the refresh below uses the latest stored refresh
	// token, so a concurrent forced refresh is safe rotation-wise.
	fresh, err := h.spotifyConnectionFor(ctx, conn.UserID)
	if err != nil {
		return "", time.Time{}, err
	}
	if fresh == nil {
		return "", time.Time{}, connect.NewError(connect.CodeFailedPrecondition, errSpotifyNotConnected)
	}
	if force {
		// Dedup concurrent forced refreshes: a token with nearly its full
		// lifetime left was just minted by whoever held the lock before us.
		if token, expiresAt, ok := freshlyMintedSpotifyToken(fresh); ok {
			return token, expiresAt, nil
		}
	} else if token, expiresAt, ok := cachedSpotifyToken(fresh); ok {
		return token, expiresAt, nil
	}

	token, err := h.refreshSpotifyToken(ctx, fresh.RefreshToken)
	if err != nil {
		if errors.Is(err, errSpotifyInvalidGrant) {
			// The refresh token is dead — drop the connection so the UI can
			// prompt for a fresh authorization. Scoping the delete to the
			// refresh token that just failed keeps it from wiping a connection
			// a concurrent re-connect has already replaced.
			if _, delErr := h.db.SpotifyConnection.Delete().
				Where(
					spotifyconnection.UserID(fresh.UserID),
					spotifyconnection.RefreshToken(fresh.RefreshToken),
				).
				Exec(ctx); delErr != nil {
				slog.Error("delete invalid spotify connection failed", "err", delErr)
			}
			return "", time.Time{}, connect.NewError(connect.CodeFailedPrecondition, errSpotifyNotConnected)
		}
		slog.Error("spotify token refresh failed", "err", err)
		return "", time.Time{}, connect.NewError(connect.CodeUnavailable, errors.New("Spotify is unavailable"))
	}

	expiresAt := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	update := h.db.SpotifyConnection.UpdateOneID(fresh.ID).
		SetAccessToken(token.AccessToken).
		SetAccessTokenExpiresAt(expiresAt)
	// Spotify may rotate the refresh token; losing the rotated one would break
	// the connection permanently.
	if token.RefreshToken != "" {
		update = update.SetRefreshToken(token.RefreshToken)
	}
	if _, err := update.Save(ctx); err != nil {
		slog.Error("persist refreshed spotify token failed", "err", err)
		return "", time.Time{}, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return token.AccessToken, expiresAt, nil
}

// cachedSpotifyToken returns the stored access token if it is still valid for
// longer than the refresh skew.
func cachedSpotifyToken(conn *ent.SpotifyConnection) (string, time.Time, bool) {
	if conn.AccessToken == "" || conn.AccessTokenExpiresAt == nil {
		return "", time.Time{}, false
	}
	expiresAt := *conn.AccessTokenExpiresAt
	if time.Until(expiresAt) <= spotifyTokenRefreshSkew {
		return "", time.Time{}, false
	}
	return conn.AccessToken, expiresAt, true
}

// freshlyMintedSpotifyToken returns the stored access token if it still has
// nearly its full lifetime left, meaning a concurrent forced refresh minted it
// moments ago.
func freshlyMintedSpotifyToken(conn *ent.SpotifyConnection) (string, time.Time, bool) {
	if conn.AccessToken == "" || conn.AccessTokenExpiresAt == nil {
		return "", time.Time{}, false
	}
	expiresAt := *conn.AccessTokenExpiresAt
	if time.Until(expiresAt) <= spotifyTokenFreshMintWindow {
		return "", time.Time{}, false
	}
	return conn.AccessToken, expiresAt, true
}

// validateSpotifyPlaylistSlot converts a proto slot to its stored form. A nil
// message means "clear this slot" and yields a nil result.
func validateSpotifyPlaylistSlot(slot *clockkeeperv1.SpotifyPlaylistSlot, name string) (*schema.SpotifyPlaylistSlot, error) {
	if slot == nil {
		return nil, nil
	}
	// The prefix alone is not a playlist — an id must follow it.
	if !strings.HasPrefix(slot.Uri, spotifyPlaylistURIPrefix) || slot.Uri == spotifyPlaylistURIPrefix {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s playlist uri must start with %q and include a playlist id", name, spotifyPlaylistURIPrefix))
	}
	if len([]rune(slot.Uri)) > maxSpotifyPlaylistURLLen {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s playlist uri must be at most %d characters", name, maxSpotifyPlaylistURLLen))
	}
	if len([]rune(slot.Name)) > maxSpotifyPlaylistNameLen {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s playlist name must be at most %d characters", name, maxSpotifyPlaylistNameLen))
	}
	if len([]rune(slot.ImageUrl)) > maxSpotifyPlaylistURLLen {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s playlist image url must be at most %d characters", name, maxSpotifyPlaylistURLLen))
	}
	return &schema.SpotifyPlaylistSlot{
		URI:      slot.Uri,
		Name:     slot.Name,
		ImageURL: slot.ImageUrl,
	}, nil
}
