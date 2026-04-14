package web

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/shifty11/clockkeeper/ent"
	"github.com/shifty11/clockkeeper/ent/user"
	clockkeeperv1 "github.com/shifty11/clockkeeper/gen/clockkeeper/v1"
)

// LoginWithDiscord exchanges a Discord OAuth code for a ClockKeeper JWT.
// If the caller has an anonymous session (JWT in Authorization header), the
// anonymous account is upgraded to a Discord-linked account.
func (h *ClockKeeperServiceHandler) LoginWithDiscord(ctx context.Context, req *connect.Request[clockkeeperv1.LoginWithDiscordRequest]) (*connect.Response[clockkeeperv1.LoginWithDiscordResponse], error) {
	if !h.config.DiscordConfigured() {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("Discord login is not configured"))
	}

	if req.Msg.Code == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("code is required"))
	}
	if req.Msg.RedirectUri == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("redirect_uri is required"))
	}
	if req.Msg.RedirectUri != h.config.DiscordRedirectURI {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("redirect_uri mismatch"))
	}

	discordUser, err := h.exchangeDiscordCode(ctx, req.Msg.Code, req.Msg.RedirectUri)
	if err != nil {
		slog.Error("discord code exchange failed", "err", err)
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("Discord authentication failed"))
	}

	// Check if a user with this Discord ID already exists.
	existingUser, err := h.db.User.Query().
		Where(user.DiscordID(discordUser.ID)).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		slog.Error("discord user lookup failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	if existingUser != nil {
		// Returning user — update Discord profile info.
		update := existingUser.Update().
			SetDiscordUsername(discordUser.DisplayName())
		if discordUser.Avatar != "" {
			update = update.SetDiscordAvatar(discordUser.Avatar)
		} else {
			update = update.ClearDiscordAvatar()
		}
		existingUser, err = update.Save(ctx)
		if err != nil {
			slog.Error("update discord user failed", "err", err)
			return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
		}

		token, err := h.auth.IssueToken(existingUser.ID, false)
		if err != nil {
			slog.Error("issue token failed", "err", err)
			return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
		}

		return connect.NewResponse(&clockkeeperv1.LoginWithDiscordResponse{
			Token:     token,
			IsNewUser: false,
		}), nil
	}

	// No existing Discord user — check if caller is an anonymous user to upgrade.
	if authHeader := req.Header().Get("Authorization"); authHeader != "" {
		if anonUserID, isAnon, err := h.auth.validate(authHeader); err == nil && isAnon {
			anonUser, err := h.db.User.Get(ctx, anonUserID)
			if err == nil && anonUser.IsAnonymous {
				// Upgrade anonymous account.
				update := anonUser.Update().
					SetDiscordID(discordUser.ID).
					SetDiscordUsername(discordUser.DisplayName()).
					SetIsAnonymous(false)
				if discordUser.Avatar != "" {
					update = update.SetDiscordAvatar(discordUser.Avatar)
				} else {
					update = update.ClearDiscordAvatar()
				}
				upgraded, err := update.Save(ctx)
				if err != nil {
					// Possible race: another request linked this Discord ID concurrently.
					if ent.IsConstraintError(err) {
						return h.loginExistingDiscordUser(ctx, discordUser.ID)
					}
					slog.Error("upgrade anonymous user failed", "err", err)
					return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
				}

				token, err := h.auth.IssueToken(upgraded.ID, false)
				if err != nil {
					slog.Error("issue token failed", "err", err)
					return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
				}

				return connect.NewResponse(&clockkeeperv1.LoginWithDiscordResponse{
					Token:     token,
					IsNewUser: false,
				}), nil
			}
		}
	}

	// Brand new user.
	newUser, err := h.db.User.Create().
		SetDiscordID(discordUser.ID).
		SetDiscordUsername(discordUser.DisplayName()).
		SetNillableDiscordAvatar(nilIfEmpty(discordUser.Avatar)).
		Save(ctx)
	if err != nil {
		// Possible race: another request created this Discord user concurrently.
		if ent.IsConstraintError(err) {
			return h.loginExistingDiscordUser(ctx, discordUser.ID)
		}
		slog.Error("create discord user failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	token, err := h.auth.IssueToken(newUser.ID, false)
	if err != nil {
		slog.Error("issue token failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.LoginWithDiscordResponse{
		Token:     token,
		IsNewUser: true,
	}), nil
}

// loginExistingDiscordUser handles the race condition where a Discord user was
// created concurrently. It re-queries and returns a token for the existing user.
func (h *ClockKeeperServiceHandler) loginExistingDiscordUser(ctx context.Context, discordID string) (*connect.Response[clockkeeperv1.LoginWithDiscordResponse], error) {
	u, err := h.db.User.Query().Where(user.DiscordID(discordID)).Only(ctx)
	if err != nil {
		slog.Error("re-query discord user after race failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}
	token, err := h.auth.IssueToken(u.ID, false)
	if err != nil {
		slog.Error("issue token failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}
	return connect.NewResponse(&clockkeeperv1.LoginWithDiscordResponse{
		Token:     token,
		IsNewUser: false,
	}), nil
}

// CreateAnonymousSession creates a new anonymous user and returns a JWT.
func (h *ClockKeeperServiceHandler) CreateAnonymousSession(ctx context.Context, req *connect.Request[clockkeeperv1.CreateAnonymousSessionRequest]) (*connect.Response[clockkeeperv1.CreateAnonymousSessionResponse], error) {
	u, err := h.db.User.Create().
		SetIsAnonymous(true).
		Save(ctx)
	if err != nil {
		slog.Error("create anonymous user failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	token, err := h.auth.IssueToken(u.ID, true)
	if err != nil {
		slog.Error("issue token failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.CreateAnonymousSessionResponse{
		Token: token,
	}), nil
}

// GetAuthConfig returns authentication configuration for the frontend.
func (h *ClockKeeperServiceHandler) GetAuthConfig(ctx context.Context, req *connect.Request[clockkeeperv1.GetAuthConfigRequest]) (*connect.Response[clockkeeperv1.GetAuthConfigResponse], error) {
	return connect.NewResponse(&clockkeeperv1.GetAuthConfigResponse{
		DiscordClientId:  h.config.DiscordClientID,
		AnonymousEnabled: true,
	}), nil
}

// nilIfEmpty returns a pointer to s if non-empty, or nil.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
