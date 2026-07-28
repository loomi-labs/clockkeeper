package web

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
)

// errTokenBagUnimplemented is returned by every token bag endpoint until the
// handlers land.
var errTokenBagUnimplemented = connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))

// TODO(token-bag): implemented in a later task
func (h *ClockKeeperServiceHandler) OpenTokenBagRegistration(_ context.Context, _ *connect.Request[clockkeeperv1.OpenTokenBagRegistrationRequest]) (*connect.Response[clockkeeperv1.OpenTokenBagRegistrationResponse], error) {
	return nil, errTokenBagUnimplemented
}

// TODO(token-bag): implemented in a later task
func (h *ClockKeeperServiceHandler) CloseTokenBagRegistration(_ context.Context, _ *connect.Request[clockkeeperv1.CloseTokenBagRegistrationRequest]) (*connect.Response[clockkeeperv1.CloseTokenBagRegistrationResponse], error) {
	return nil, errTokenBagUnimplemented
}

// TODO(token-bag): implemented in a later task
func (h *ClockKeeperServiceHandler) RemoveTokenBagRegistration(_ context.Context, _ *connect.Request[clockkeeperv1.RemoveTokenBagRegistrationRequest]) (*connect.Response[clockkeeperv1.RemoveTokenBagRegistrationResponse], error) {
	return nil, errTokenBagUnimplemented
}

// TODO(token-bag): implemented in a later task
func (h *ClockKeeperServiceHandler) RevealTokenBag(_ context.Context, _ *connect.Request[clockkeeperv1.RevealTokenBagRequest]) (*connect.Response[clockkeeperv1.RevealTokenBagResponse], error) {
	return nil, errTokenBagUnimplemented
}

// TODO(token-bag): implemented in a later task
func (h *ClockKeeperServiceHandler) ResetTokenBag(_ context.Context, _ *connect.Request[clockkeeperv1.ResetTokenBagRequest]) (*connect.Response[clockkeeperv1.ResetTokenBagResponse], error) {
	return nil, errTokenBagUnimplemented
}

// TODO(token-bag): implemented in a later task
func (h *ClockKeeperServiceHandler) GetTokenBag(_ context.Context, _ *connect.Request[clockkeeperv1.GetTokenBagRequest]) (*connect.Response[clockkeeperv1.GetTokenBagResponse], error) {
	return nil, errTokenBagUnimplemented
}

// TODO(token-bag): implemented in a later task
func (h *ClockKeeperServiceHandler) GetTokenBagSeating(_ context.Context, _ *connect.Request[clockkeeperv1.GetTokenBagSeatingRequest]) (*connect.Response[clockkeeperv1.GetTokenBagSeatingResponse], error) {
	return nil, errTokenBagUnimplemented
}

// TODO(token-bag): implemented in a later task
func (h *ClockKeeperServiceHandler) JoinTokenBag(_ context.Context, _ *connect.Request[clockkeeperv1.JoinTokenBagRequest]) (*connect.Response[clockkeeperv1.JoinTokenBagResponse], error) {
	return nil, errTokenBagUnimplemented
}

// TODO(token-bag): implemented in a later task
func (h *ClockKeeperServiceHandler) SetTokenBagNeighbors(_ context.Context, _ *connect.Request[clockkeeperv1.SetTokenBagNeighborsRequest]) (*connect.Response[clockkeeperv1.SetTokenBagNeighborsResponse], error) {
	return nil, errTokenBagUnimplemented
}

// TODO(token-bag): implemented in a later task
func (h *ClockKeeperServiceHandler) GetMyToken(_ context.Context, _ *connect.Request[clockkeeperv1.GetMyTokenRequest]) (*connect.Response[clockkeeperv1.GetMyTokenResponse], error) {
	return nil, errTokenBagUnimplemented
}

// TODO(token-bag): implemented in a later task
func (h *ClockKeeperServiceHandler) WatchTokenBag(_ context.Context, _ *connect.Request[clockkeeperv1.WatchTokenBagRequest], _ *connect.ServerStream[clockkeeperv1.WatchTokenBagResponse]) error {
	return errTokenBagUnimplemented
}

// TODO(token-bag): implemented in a later task
func (h *ClockKeeperServiceHandler) JoinTokenBagShared(_ context.Context, _ *connect.Request[clockkeeperv1.JoinTokenBagSharedRequest]) (*connect.Response[clockkeeperv1.JoinTokenBagSharedResponse], error) {
	return nil, errTokenBagUnimplemented
}

// TODO(token-bag): implemented in a later task
func (h *ClockKeeperServiceHandler) RevealTokenShared(_ context.Context, _ *connect.Request[clockkeeperv1.RevealTokenSharedRequest]) (*connect.Response[clockkeeperv1.RevealTokenSharedResponse], error) {
	return nil, errTokenBagUnimplemented
}
