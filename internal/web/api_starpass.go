package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/loomi-labs/clockkeeper/ent"
	"github.com/loomi-labs/clockkeeper/ent/game"
	"github.com/loomi-labs/clockkeeper/ent/phase"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
	"github.com/loomi-labs/clockkeeper/internal/botc"
)

// StarPass handles the Imp "star pass": when the Imp kills himself at night the
// Storyteller promotes a Minion to become the new Imp. Seats are keyed by real
// role id (there is no Player entity), so promotion is a two-seat ROLE SWAP:
// {"imp" -> minionRoleID, minionRoleID -> "imp"} applied everywhere role ids key
// game state. The dead seat (the old Imp) ends up holding the minion's former
// role id; the promoted seat becomes "imp". This mirrors physically swapping the
// two character tokens in the grimoire.
func (h *ClockKeeperServiceHandler) StarPass(ctx context.Context, req *connect.Request[clockkeeperv1.StarPassRequest]) (*connect.Response[clockkeeperv1.StarPassResponse], error) {
	// Ownership check (CodeNotFound for non-owner/missing).
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	if g.State != game.StateInProgress {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("game is not in progress"))
	}

	inRoles := make(map[string]bool, len(g.SelectedRoles))
	for _, id := range g.SelectedRoles {
		inRoles[id] = true
	}
	if !inRoles["imp"] {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("imp is not in play"))
	}

	minionRoleID := req.Msg.MinionRoleId
	if minionRoleID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("minion_role_id is required"))
	}
	if minionRoleID == "imp" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("minion_role_id must not be the imp"))
	}
	if !inRoles[minionRoleID] {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s is not in play", minionRoleID))
	}
	// The promoted seat's REAL character must be a Minion.
	char, ok := h.registry.Character(minionRoleID)
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown character: %s", minionRoleID))
	}
	if char.Team != botc.TeamMinion {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s is not a minion", minionRoleID))
	}

	// Two-seat swap mapping, applied atomically by applySeatRename.
	mapping := map[string]string{
		"imp":        minionRoleID,
		minionRoleID: "imp",
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
			q.WithDeaths().Order(ent.Asc(phase.FieldID))
		}).
		Only(ctx)
	if err != nil {
		_ = tx.Rollback()
		slog.Error("re-fetch game in tx failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	// Both roles remain in play (they just swap seats), so no reminder tokens are
	// dropped — pass a nil drop predicate.
	if err = h.applySeatRename(ctx, tx, g, mapping, nil); err != nil {
		_ = tx.Rollback()
		slog.Error("apply seat rename for star pass failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	// completed_actions: intentionally NOT remapped. Night-entry ids are keyed by
	// the SHOWN character id; both tokens remain shown (they only swap seats), so
	// completed night actions stay valid as-is.
	//
	// selected_bluffs: left untouched, mirroring ReassignBagSubstitution.

	if err = tx.Commit(); err != nil {
		slog.Error("commit failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	// Re-fetch so the response carries eager-loaded PlayState (avoids blanking the
	// in-progress view — see the Save-then-serialize bug in UpdateDemonBluffs).
	g, err = h.getOwnedGame(ctx, g.ID)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&clockkeeperv1.StarPassResponse{
		Game: entGameToProto(g, h.registry),
	}), nil
}
