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

	// The game leaving setup takes the role display off the players' phones (see
	// buildWatchSnapshot's game_started). Their devices only learn about it
	// through the token bag stream, so it has to be woken here.
	h.publishTokenBag(g.ID)

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

	// Find the single active phase inside the transaction. Both Night and Day
	// are valid active phases now — BotC alternates Night N -> Day N -> Night N+1
	// and AdvancePhase steps through them one at a time.
	activePhase, err := tx.Phase.Query().
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

	if activePhase.Type == phase.TypeNight {
		// Night N -> Day N: deactivate the night and activate the companion day
		// of the same round. No new round, no death/alignment propagation — night
		// deaths were already propagated into Day N at record time (RecordDeath
		// copies a death forward into all later phases, including Day N, when the
		// caller passes propagate=true), so Day N already reflects them.
		if err := h.advanceNightToDay(ctx, tx, g, activePhase); err != nil {
			_ = tx.Rollback()
			return nil, err
		}
	} else {
		// Day N -> Night N+1: deactivate the day, then create the next
		// Night N+1 (active) + Day N+1 (inactive) pair, propagating accumulated
		// deaths and character alignments from Day N into both.
		if err := h.advanceDayToNextRound(ctx, tx, g, activePhase); err != nil {
			_ = tx.Rollback()
			return nil, err
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

// advanceNightToDay handles Night N -> Day N: deactivate the active night and
// activate the companion Day phase of the same round. Callers roll back the tx
// on error. No death/alignment propagation happens here — night deaths already
// landed in Day N at record time via RecordDeath's propagate flag.
func (h *ClockKeeperServiceHandler) advanceNightToDay(ctx context.Context, tx *ent.Tx, g *ent.Game, activeNight *ent.Phase) error {
	// Locate the companion day phase of the current round (created in pairs).
	var dayPhase *ent.Phase
	for _, p := range g.Edges.Phases {
		if p.RoundNumber == activeNight.RoundNumber && p.Type == phase.TypeDay {
			dayPhase = p
			break
		}
	}

	// Deactivate the current night phase.
	if _, err := tx.Phase.UpdateOneID(activeNight.ID).SetIsActive(false).Save(ctx); err != nil {
		slog.Error("deactivate night phase failed", "err", err)
		return connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	// Defensively create the day phase if it is somehow missing (phases are
	// normally created in Night+Day pairs, so this should not happen).
	if dayPhase == nil {
		created, err := tx.Phase.Create().
			SetGameID(g.ID).
			SetRoundNumber(activeNight.RoundNumber).
			SetType(phase.TypeDay).
			SetIsActive(true).
			Save(ctx)
		if err != nil {
			slog.Error("create missing day phase failed", "err", err)
			return connect.NewError(connect.CodeInternal, errors.New("internal server error"))
		}
		_ = created
		return nil
	}

	// Activate the existing companion day phase.
	if _, err := tx.Phase.UpdateOneID(dayPhase.ID).SetIsActive(true).Save(ctx); err != nil {
		slog.Error("activate day phase failed", "err", err)
		return connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}
	return nil
}

// advanceDayToNextRound handles Day N -> Night N+1: deactivate the active day,
// then create the next round's Night (active) + Day (inactive) pair, propagating
// accumulated deaths and character alignments from Day N into both new phases.
// Callers roll back the tx on error.
func (h *ClockKeeperServiceHandler) advanceDayToNextRound(ctx context.Context, tx *ent.Tx, g *ent.Game, activeDay *ent.Phase) error {
	// Day N carries all accumulated deaths and the latest alignments. Deaths are
	// read from the eager-loaded game phases (activeDay was queried without its
	// deaths edge); alignments live on the phase row itself.
	var currentDayDeaths []*ent.Death
	for _, p := range g.Edges.Phases {
		if p.ID == activeDay.ID {
			currentDayDeaths = p.Edges.Deaths
			break
		}
	}
	currentDayAlignments := activeDay.CharacterAlignments
	nextRound := activeDay.RoundNumber + 1

	// Deactivate the current day phase.
	if _, err := tx.Phase.UpdateOneID(activeDay.ID).SetIsActive(false).Save(ctx); err != nil {
		slog.Error("deactivate day phase failed", "err", err)
		return connect.NewError(connect.CodeInternal, errors.New("internal server error"))
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
		slog.Error("create next night phase failed", "err", err)
		return connect.NewError(connect.CodeInternal, errors.New("internal server error"))
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
		slog.Error("create next day phase failed", "err", err)
		return connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	// Propagate deaths from Day N (has all accumulated deaths) into both new
	// phases. These carried-forward copies deliberately keep cause UNSPECIFIED:
	// they only mean "already dead", not that a new death occurred this round.
	for _, d := range currentDayDeaths {
		if _, err := tx.Death.Create().
			SetPhaseID(newNight.ID).
			SetRoleID(d.RoleID).
			SetGhostVote(d.GhostVote).
			Save(ctx); err != nil {
			slog.Error("copy death to new night failed", "err", err)
			return connect.NewError(connect.CodeInternal, errors.New("internal server error"))
		}
		if _, err := tx.Death.Create().
			SetPhaseID(newDay.ID).
			SetRoleID(d.RoleID).
			SetGhostVote(d.GhostVote).
			Save(ctx); err != nil {
			slog.Error("copy death to new day failed", "err", err)
			return connect.NewError(connect.CodeInternal, errors.New("internal server error"))
		}
	}
	return nil
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

	// Same as StartGame: a state change out of setup is a change the watching
	// player devices have to see.
	h.publishTokenBag(g.ID)

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
