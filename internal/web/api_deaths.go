package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/loomi-labs/clockkeeper/ent"
	"github.com/loomi-labs/clockkeeper/ent/death"
	"github.com/loomi-labs/clockkeeper/ent/game"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
)

func (h *ClockKeeperServiceHandler) RecordDeath(ctx context.Context, req *connect.Request[clockkeeperv1.RecordDeathRequest]) (*connect.Response[clockkeeperv1.RecordDeathResponse], error) {
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	if g.State != game.StateInProgress {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("game is not in progress"))
	}

	// Validate role exists in registry.
	if _, ok := h.registry.Character(req.Msg.RoleId); !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown character: %s", req.Msg.RoleId))
	}

	// Validate role is in the game.
	if !isRoleInGame(g, req.Msg.RoleId) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("character %s is not in this game", req.Msg.RoleId))
	}

	// Determine target phase.
	var targetPhaseID int
	if req.Msg.PhaseId != nil {
		targetPhaseID = int(*req.Msg.PhaseId)
		found := false
		for _, p := range g.Edges.Phases {
			if p.ID == targetPhaseID {
				found = true
				break
			}
		}
		if !found {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("phase not found"))
		}
	} else {
		activePhase, err := h.getActivePhase(ctx, g.ID)
		if err != nil {
			return nil, err
		}
		targetPhaseID = activePhase.ID
	}

	// Collect phases to create death records for.
	// Phases are ordered by ID (creation order) from getOwnedGame.
	targetPhases := phasesFromID(g.Edges.Phases, targetPhaseID, req.Msg.Propagate)

	// Build set of phases that already have a death for this role (idempotent).
	existingDeathPhases := make(map[int]bool)
	for _, p := range g.Edges.Phases {
		for _, d := range p.Edges.Deaths {
			if d.RoleID == req.Msg.RoleId {
				existingDeathPhases[p.ID] = true
			}
		}
	}

	tx, err := h.db.Tx(ctx)
	if err != nil {
		slog.Error("start transaction failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	for _, p := range targetPhases {
		if existingDeathPhases[p.ID] {
			continue
		}
		_, err = tx.Death.Create().
			SetPhaseID(p.ID).
			SetRoleID(req.Msg.RoleId).
			SetGhostVote(true).
			Save(ctx)
		if err != nil {
			_ = tx.Rollback()
			slog.Error("create death failed", "err", err)
			return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
		}
	}

	if err = tx.Commit(); err != nil {
		slog.Error("commit transaction failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	g, err = h.getOwnedGame(ctx, g.ID)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&clockkeeperv1.RecordDeathResponse{
		Game: entGameToProto(g, h.registry),
	}), nil
}

func (h *ClockKeeperServiceHandler) RemoveDeath(ctx context.Context, req *connect.Request[clockkeeperv1.RemoveDeathRequest]) (*connect.Response[clockkeeperv1.RemoveDeathResponse], error) {
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	if g.State != game.StateInProgress {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("game is not in progress"))
	}

	// Load the death and verify it belongs to this game.
	d, err := h.db.Death.Query().
		Where(death.ID(int(req.Msg.DeathId))).
		WithPhase().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("death not found"))
		}
		slog.Error("get death failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	if d.Edges.Phase.GameID != g.ID {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("death not found"))
	}

	if req.Msg.Propagate {
		// Delete death records for this role in this phase and all later phases.
		laterPhases := phasesFromID(g.Edges.Phases, d.Edges.Phase.ID, true)
		laterPhaseIDs := make([]int, 0, len(laterPhases))
		for _, p := range laterPhases {
			laterPhaseIDs = append(laterPhaseIDs, p.ID)
		}
		_, err = h.db.Death.Delete().
			Where(
				death.RoleID(d.RoleID),
				death.PhaseIDIn(laterPhaseIDs...),
			).
			Exec(ctx)
	} else {
		err = h.db.Death.DeleteOneID(d.ID).Exec(ctx)
	}
	if err != nil {
		slog.Error("delete death failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	g, err = h.getOwnedGame(ctx, g.ID)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&clockkeeperv1.RemoveDeathResponse{
		Game: entGameToProto(g, h.registry),
	}), nil
}

func (h *ClockKeeperServiceHandler) UseGhostVote(ctx context.Context, req *connect.Request[clockkeeperv1.UseGhostVoteRequest]) (*connect.Response[clockkeeperv1.UseGhostVoteResponse], error) {
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	if g.State != game.StateInProgress {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("game is not in progress"))
	}

	// Load the death and verify it belongs to this game.
	d, err := h.db.Death.Query().
		Where(death.ID(int(req.Msg.DeathId))).
		WithPhase().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("death not found"))
		}
		slog.Error("get death failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	if d.Edges.Phase.GameID != g.ID {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("death not found"))
	}

	if !d.GhostVote {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("ghost vote already used"))
	}

	// Set ghost_vote=false on ALL death records for this role across all phases.
	allPhaseIDs := make([]int, 0, len(g.Edges.Phases))
	for _, p := range g.Edges.Phases {
		allPhaseIDs = append(allPhaseIDs, p.ID)
	}
	_, err = h.db.Death.Update().
		Where(
			death.RoleID(d.RoleID),
			death.PhaseIDIn(allPhaseIDs...),
		).
		SetGhostVote(false).
		Save(ctx)
	if err != nil {
		slog.Error("update ghost vote failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	g, err = h.getOwnedGame(ctx, g.ID)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&clockkeeperv1.UseGhostVoteResponse{
		Game: entGameToProto(g, h.registry),
	}), nil
}

// phasesFromID returns the phase with the given ID and, if propagate is true,
// all phases that come after it. Phases must be pre-sorted by ID (creation order).
func phasesFromID(phases []*ent.Phase, fromID int, propagate bool) []*ent.Phase {
	for i, p := range phases {
		if p.ID == fromID {
			if propagate {
				return phases[i:]
			}
			return phases[i : i+1]
		}
	}
	return nil
}

// isRoleInGame checks if a role ID is in the game's selected roles, travellers, or extra characters.
func isRoleInGame(g *ent.Game, roleID string) bool {
	for _, id := range g.SelectedRoles {
		if id == roleID {
			return true
		}
	}
	for _, id := range g.SelectedTravellers {
		if id == roleID {
			return true
		}
	}
	for _, id := range g.ExtraCharacters {
		if id == roleID {
			return true
		}
	}
	return false
}
