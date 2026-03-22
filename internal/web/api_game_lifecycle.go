package web

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/loomi-labs/clockkeeper/ent"
	"github.com/loomi-labs/clockkeeper/ent/game"
	"github.com/loomi-labs/clockkeeper/ent/phase"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
)

func (h *ClockKeeperServiceHandler) StartGame(ctx context.Context, req *connect.Request[clockkeeperv1.StartGameRequest]) (*connect.Response[clockkeeperv1.StartGameResponse], error) {
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	if g.State != game.StateSetup {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("game is not in setup state"))
	}
	if len(g.SelectedRoles) == 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no roles selected"))
	}

	tx, err := h.db.Tx(ctx)
	if err != nil {
		slog.Error("start transaction failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	_, err = tx.Game.UpdateOneID(g.ID).SetState(game.StateInProgress).Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		slog.Error("update game state failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	_, err = tx.Phase.Create().
		SetGameID(g.ID).
		SetRoundNumber(1).
		SetType(phase.TypeNight).
		SetIsActive(true).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		slog.Error("create first phase failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	if err := tx.Commit(); err != nil {
		slog.Error("commit failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	// Re-fetch with eager-loaded phases.
	g, err = h.getOwnedGame(ctx, g.ID)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&clockkeeperv1.StartGameResponse{
		Game: entGameToProto(g, h.registry),
	}), nil
}

func (h *ClockKeeperServiceHandler) AdvancePhase(ctx context.Context, req *connect.Request[clockkeeperv1.AdvancePhaseRequest]) (*connect.Response[clockkeeperv1.AdvancePhaseResponse], error) {
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	if g.State != game.StateInProgress {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("game is not in progress"))
	}

	activePhase, err := h.getActivePhase(ctx, g.ID)
	if err != nil {
		return nil, err
	}

	// Determine next phase.
	var nextType phase.Type
	var nextRound int
	if activePhase.Type == phase.TypeNight {
		nextType = phase.TypeDay
		nextRound = activePhase.RoundNumber
	} else {
		nextType = phase.TypeNight
		nextRound = activePhase.RoundNumber + 1
	}

	tx, err := h.db.Tx(ctx)
	if err != nil {
		slog.Error("start transaction failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	_, err = tx.Phase.UpdateOneID(activePhase.ID).SetIsActive(false).Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		slog.Error("deactivate phase failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	newPhase, err := tx.Phase.Create().
		SetGameID(g.ID).
		SetRoundNumber(nextRound).
		SetType(nextType).
		SetIsActive(true).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		slog.Error("create next phase failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	// Copy death records from the old phase to the new phase (auto-propagation).
	// Use the game's eagerly-loaded phase data (activePhase from getActivePhase has no edges).
	var oldPhaseDeaths []*ent.Death
	for _, p := range g.Edges.Phases {
		if p.ID == activePhase.ID {
			oldPhaseDeaths = p.Edges.Deaths
			break
		}
	}
	for _, d := range oldPhaseDeaths {
		_, err = tx.Death.Create().
			SetPhaseID(newPhase.ID).
			SetRoleID(d.RoleID).
			SetGhostVote(d.GhostVote).
			Save(ctx)
		if err != nil {
			_ = tx.Rollback()
			slog.Error("copy death to new phase failed", "err", err)
			return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
		}
	}

	if err := tx.Commit(); err != nil {
		slog.Error("commit failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	g, err = h.getOwnedGame(ctx, g.ID)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&clockkeeperv1.AdvancePhaseResponse{
		Game: entGameToProto(g, h.registry),
	}), nil
}

func (h *ClockKeeperServiceHandler) EndGame(ctx context.Context, req *connect.Request[clockkeeperv1.EndGameRequest]) (*connect.Response[clockkeeperv1.EndGameResponse], error) {
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	if g.State != game.StateInProgress {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("game is not in progress"))
	}

	tx, err := h.db.Tx(ctx)
	if err != nil {
		slog.Error("start transaction failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	// Deactivate any active phase.
	_, err = tx.Phase.Update().
		Where(phase.GameID(g.ID), phase.IsActive(true)).
		SetIsActive(false).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		slog.Error("deactivate phase failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	_, err = tx.Game.UpdateOneID(g.ID).SetState(game.StateCompleted).Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		slog.Error("update game state failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	if err := tx.Commit(); err != nil {
		slog.Error("commit failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	g, err = h.getOwnedGame(ctx, g.ID)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&clockkeeperv1.EndGameResponse{
		Game: entGameToProto(g, h.registry),
	}), nil
}

func (h *ClockKeeperServiceHandler) ToggleNightAction(ctx context.Context, req *connect.Request[clockkeeperv1.ToggleNightActionRequest]) (*connect.Response[clockkeeperv1.ToggleNightActionResponse], error) {
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	if g.State != game.StateInProgress {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("game is not in progress"))
	}

	// Load the target phase and validate it belongs to this game and is a night phase.
	targetPhase, err := h.db.Phase.Get(ctx, int(req.Msg.PhaseId))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("phase not found"))
		}
		slog.Error("get phase failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}
	if targetPhase.GameID != g.ID {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("phase not found"))
	}
	if targetPhase.Type != phase.TypeNight {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("can only toggle night actions on night phases"))
	}

	// Build updated completed actions list.
	actions := make([]string, 0, len(targetPhase.CompletedActions)+1)
	found := false
	for _, id := range targetPhase.CompletedActions {
		if id == req.Msg.ActionId {
			found = true
			if req.Msg.Done {
				actions = append(actions, id)
			}
		} else {
			actions = append(actions, id)
		}
	}
	if req.Msg.Done && !found {
		actions = append(actions, req.Msg.ActionId)
	}

	_, err = h.db.Phase.UpdateOneID(targetPhase.ID).SetCompletedActions(actions).Save(ctx)
	if err != nil {
		slog.Error("update completed actions failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	g, err = h.getOwnedGame(ctx, g.ID)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&clockkeeperv1.ToggleNightActionResponse{
		Game: entGameToProto(g, h.registry),
	}), nil
}

// getActivePhase finds the active phase for a game.
func (h *ClockKeeperServiceHandler) getActivePhase(ctx context.Context, gameID int) (*ent.Phase, error) {
	p, err := h.db.Phase.Query().
		Where(phase.GameID(gameID), phase.IsActive(true)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no active phase"))
		}
		slog.Error("get active phase failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}
	return p, nil
}
