package web

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/loomi-labs/clockkeeper/ent"
	"github.com/loomi-labs/clockkeeper/ent/game"
	"github.com/loomi-labs/clockkeeper/ent/phase"
	"github.com/loomi-labs/clockkeeper/ent/schema"
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

	n, err := tx.Game.Update().
		Where(game.IDEQ(g.ID), game.StateEQ(game.StateSetup)).
		SetState(game.StateInProgress).
		Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		slog.Error("update game state failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}
	if n == 0 {
		_ = tx.Rollback()
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("game is not in setup state"))
	}

	// Seed initial character alignments from traveller setup alignments.
	initialAlignments := make(map[string]string)
	for id, align := range g.TravellerAlignments {
		switch align {
		case schema.AlignmentGood:
			initialAlignments[id] = "good"
		case schema.AlignmentEvil:
			initialAlignments[id] = "evil"
		}
	}

	// Create Night+Day pair for round 1.
	nightCreate := tx.Phase.Create().
		SetGameID(g.ID).
		SetRoundNumber(1).
		SetType(phase.TypeNight).
		SetIsActive(true)
	if len(initialAlignments) > 0 {
		nightCreate = nightCreate.SetCharacterAlignments(initialAlignments)
	}
	_, err = nightCreate.Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		slog.Error("create first night phase failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	dayCreate := tx.Phase.Create().
		SetGameID(g.ID).
		SetRoundNumber(1).
		SetType(phase.TypeDay).
		SetIsActive(false)
	if len(initialAlignments) > 0 {
		dayCreate = dayCreate.SetCharacterAlignments(initialAlignments)
	}
	_, err = dayCreate.Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		slog.Error("create first day phase failed", "err", err)
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
	// Ownership check before the transaction (auth gate only).
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

	// Re-query the game inside the transaction so all reads are consistent with writes.
	g, err = tx.Game.Query().
		Where(game.ID(g.ID)).
		WithPhases(func(q *ent.PhaseQuery) {
			q.WithDeaths().
				Order(ent.Asc(phase.FieldID))
		}).
		Only(ctx)
	if err != nil {
		_ = tx.Rollback()
		slog.Error("re-fetch game in tx failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	// Find the active night phase inside the transaction.
	activeNight, err := tx.Phase.Query().
		Where(phase.GameID(g.ID), phase.IsActive(true)).
		Only(ctx)
	if err != nil {
		_ = tx.Rollback()
		if ent.IsNotFound(err) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no active phase"))
		}
		slog.Error("get active phase failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	if activeNight.Type != phase.TypeNight {
		_ = tx.Rollback()
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("active phase is not a night phase"))
	}

	// Find the companion day phase of the current round.
	var currentDayDeaths []*ent.Death
	var currentDayAlignments map[string]string
	for _, p := range g.Edges.Phases {
		if p.RoundNumber == activeNight.RoundNumber && p.Type == phase.TypeDay {
			currentDayDeaths = p.Edges.Deaths
			currentDayAlignments = p.CharacterAlignments
			break
		}
	}

	nextRound := activeNight.RoundNumber + 1

	// Deactivate current night phase.
	_, err = tx.Phase.UpdateOneID(activeNight.ID).SetIsActive(false).Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		slog.Error("deactivate phase failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	// Create next Night+Day pair (with propagated alignments).
	nightCreate := tx.Phase.Create().
		SetGameID(g.ID).
		SetRoundNumber(nextRound).
		SetType(phase.TypeNight).
		SetIsActive(true)
	if len(currentDayAlignments) > 0 {
		nightCreate = nightCreate.SetCharacterAlignments(currentDayAlignments)
	}
	newNight, err := nightCreate.Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		slog.Error("create next night phase failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	dayCreate := tx.Phase.Create().
		SetGameID(g.ID).
		SetRoundNumber(nextRound).
		SetType(phase.TypeDay).
		SetIsActive(false)
	if len(currentDayAlignments) > 0 {
		dayCreate = dayCreate.SetCharacterAlignments(currentDayAlignments)
	}
	newDay, err := dayCreate.Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		slog.Error("create next day phase failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	// Propagate deaths from Day N (has all accumulated deaths) into both new phases.
	for _, d := range currentDayDeaths {
		_, err = tx.Death.Create().
			SetPhaseID(newNight.ID).
			SetRoleID(d.RoleID).
			SetGhostVote(d.GhostVote).
			Save(ctx)
		if err != nil {
			_ = tx.Rollback()
			slog.Error("copy death to new night failed", "err", err)
			return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
		}
		_, err = tx.Death.Create().
			SetPhaseID(newDay.ID).
			SetRoleID(d.RoleID).
			SetGhostVote(d.GhostVote).
			Save(ctx)
		if err != nil {
			_ = tx.Rollback()
			slog.Error("copy death to new day failed", "err", err)
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
