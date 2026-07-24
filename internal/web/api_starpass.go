package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/loomi-labs/clockkeeper/ent/game"
	"github.com/loomi-labs/clockkeeper/ent/schema"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
	"github.com/loomi-labs/clockkeeper/internal/botc"
)

// StarPass handles the Imp "star pass": when the demon kills himself at night the
// Storyteller promotes a Minion to act as the new demon. Seats keep their REAL
// role id — promotion is recorded as a revertible overlay entry
// {role_id: minionRoleID, acts_as_role_id: demonRoleID} appended to the game's
// role_promotions list. Nothing else in game state (deaths, grimoire maps,
// selected roles, alignments) is mutated: the marker alone tells the UI that the
// minion's seat now acts/displays as the demon. UndoStarPass removes the entry.
func (h *ClockKeeperServiceHandler) StarPass(ctx context.Context, req *connect.Request[clockkeeperv1.StarPassRequest]) (*connect.Response[clockkeeperv1.StarPassResponse], error) {
	// Ownership check (CodeNotFound for non-owner/missing).
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	if g.State != game.StateInProgress {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("game is not in progress"))
	}

	minionRoleID := req.Msg.MinionRoleId
	if minionRoleID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("minion_role_id is required"))
	}

	inRoles := make(map[string]bool, len(g.SelectedRoles))
	for _, id := range g.SelectedRoles {
		inRoles[id] = true
	}
	if !inRoles[minionRoleID] {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s is not in play", minionRoleID))
	}
	// The promoted seat's REAL character must be a Minion (this also rejects
	// targeting the demon itself, which is never a Minion).
	char, ok := h.registry.Character(minionRoleID)
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown character: %s", minionRoleID))
	}
	if char.Team != botc.TeamMinion {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s is not a minion", minionRoleID))
	}

	// A demon must be in play to hand off. Collect selected demon roles; the minion
	// acts as "imp" when present (standard star pass), otherwise as the single demon
	// role on the script (single-demon scripts).
	var demonRoleID string
	for _, id := range g.SelectedRoles {
		if c, found := h.registry.Character(id); found && c.Team == botc.TeamDemon {
			demonRoleID = id
			break
		}
	}
	if demonRoleID == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no demon in play"))
	}
	actsAs := demonRoleID
	if inRoles["imp"] {
		actsAs = "imp"
	}

	// Reject double promotion of the same seat.
	for _, p := range g.RolePromotions {
		if p.RoleID == minionRoleID {
			return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("%s is already promoted", minionRoleID))
		}
	}

	// A dead Minion cannot be promoted — the star pass hands the demon to a LIVING
	// Minion. Deaths propagate forward, so the active phase's death list is the
	// authoritative "currently dead" set. getOwnedGame eager-loads phases+deaths
	// (the client also filters dead Minions, but the RPC must not rely on it).
	for _, p := range g.Edges.Phases {
		if !p.IsActive {
			continue
		}
		for _, d := range p.Edges.Deaths {
			if d.RoleID == minionRoleID {
				return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("%s is dead — only a living minion can become the demon", minionRoleID))
			}
		}
	}

	promotions := append(g.RolePromotions, schema.GameRolePromotion{RoleID: minionRoleID, ActsAsRoleID: actsAs})
	if _, err = g.Update().SetRolePromotions(promotions).Save(ctx); err != nil {
		slog.Error("save role promotion failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	// Re-fetch so the response carries eager-loaded PlayState (Save() drops eager
	// edges — see the Save-then-serialize bug in UpdateDemonBluffs).
	g, err = h.getOwnedGame(ctx, g.ID)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&clockkeeperv1.StarPassResponse{
		Game: entGameToProto(g, h.registry),
	}), nil
}

// UndoStarPass reverts a star-pass promotion by removing the overlay entry whose
// real role id matches req.role_id. The seat resumes acting as its real role.
func (h *ClockKeeperServiceHandler) UndoStarPass(ctx context.Context, req *connect.Request[clockkeeperv1.UndoStarPassRequest]) (*connect.Response[clockkeeperv1.UndoStarPassResponse], error) {
	// Ownership check (CodeNotFound for non-owner/missing).
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	if g.State != game.StateInProgress {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("game is not in progress"))
	}

	roleID := req.Msg.RoleId
	if roleID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("role_id is required"))
	}

	remaining := make([]schema.GameRolePromotion, 0, len(g.RolePromotions))
	found := false
	for _, p := range g.RolePromotions {
		if p.RoleID == roleID {
			found = true
			continue
		}
		remaining = append(remaining, p)
	}
	if !found {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("%s is not promoted", roleID))
	}

	if _, err = g.Update().SetRolePromotions(remaining).Save(ctx); err != nil {
		slog.Error("save role promotion removal failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	// Re-fetch so the response carries eager-loaded PlayState (Save() drops eager edges).
	g, err = h.getOwnedGame(ctx, g.ID)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&clockkeeperv1.UndoStarPassResponse{
		Game: entGameToProto(g, h.registry),
	}), nil
}
