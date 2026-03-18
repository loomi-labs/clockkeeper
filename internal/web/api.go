package web

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/loomi-labs/clockkeeper/ent"
	"github.com/loomi-labs/clockkeeper/ent/user"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
	"github.com/loomi-labs/clockkeeper/internal/botc"
)

// ClockKeeperServiceHandler implements the ConnectRPC ClockKeeperService.
type ClockKeeperServiceHandler struct {
	config   *Config
	db       *ent.Client
	auth     *AuthInterceptor
	registry *botc.Registry
}

func (h *ClockKeeperServiceHandler) Login(ctx context.Context, req *connect.Request[clockkeeperv1.LoginRequest]) (*connect.Response[clockkeeperv1.LoginResponse], error) {
	u, err := h.db.User.Query().
		Where(user.Username(req.Msg.Username)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid credentials"))
		}
		slog.Error("login query failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}
	if !CheckPassword(req.Msg.Password, u.PasswordHash) {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid credentials"))
	}

	token, err := h.auth.IssueToken(req.Msg.Username)
	if err != nil {
		slog.Error("issue token failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return connect.NewResponse(&clockkeeperv1.LoginResponse{Token: token}), nil
}
