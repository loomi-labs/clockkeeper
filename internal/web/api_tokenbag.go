package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sort"
	"strings"

	"connectrpc.com/connect"
	"github.com/loomi-labs/clockkeeper/ent"
	"github.com/loomi-labs/clockkeeper/ent/game"
	"github.com/loomi-labs/clockkeeper/ent/registration"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
)

// The digital token bag lets players register their names from their own phones
// (or via a shared device the storyteller passes around) and then see the
// character they were dealt. The storyteller drives the phases:
//
//	inactive -> open (players register) -> closed (players pick neighbors)
//	         -> revealed (players see their token)
//
// Both later phases can go back: closed re-opens registration, and revealed
// resets to closed (see ResetTokenBag) so the roles can be re-assigned. Nothing
// returns to inactive — once a bag has been opened, its codes are its codes for
// the rest of the game.
//
// Names are matched to roles through the grimoire's role_id -> player name map,
// which the storyteller fills in with the existing grimoire UI. Reveal snapshots
// that mapping onto the registrations, so later grimoire edits cannot change what
// a player already saw.

// --- Storyteller RPCs (owner-gated) ---

// OpenTokenBagRegistration opens (or re-opens) registration. Re-opening a closed
// bag keeps the players who already joined, and keeps the existing codes so QR
// codes handed out earlier keep working. Codes are minted once, on the first
// open of a game's bag, and never rotated afterwards.
//
// Opening also backfills the bag from the grimoire's player names — see
// backfillBagFromGrimoire.
func (h *ClockKeeperServiceHandler) OpenTokenBagRegistration(ctx context.Context, req *connect.Request[clockkeeperv1.OpenTokenBagRegistrationRequest]) (*connect.Response[clockkeeperv1.OpenTokenBagRegistrationResponse], error) {
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	if g.State == game.StateCompleted {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("game is completed"))
	}

	switch g.TokenBagPhase {
	case game.TokenBagPhaseRevealed:
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("tokens are already revealed — reset the reveal first"))
	case game.TokenBagPhaseOpen:
		// Already open: idempotent success.
	default:
		tx, err := h.db.Tx(ctx)
		if err != nil {
			slog.Error("start transaction failed", "err", err)
			return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
		}

		upd := tx.Game.UpdateOneID(g.ID).SetTokenBagPhase(game.TokenBagPhaseOpen)
		if g.TokenBagJoinCode == nil {
			code, err := newBagCode()
			if err != nil {
				_ = tx.Rollback()
				slog.Error("generate token bag join code failed", "err", err)
				return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
			}
			upd = upd.SetTokenBagJoinCode(code)
		}
		if g.TokenBagSharedCode == nil {
			code, err := newBagCode()
			if err != nil {
				_ = tx.Rollback()
				slog.Error("generate token bag shared code failed", "err", err)
				return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
			}
			upd = upd.SetTokenBagSharedCode(code)
		}
		if _, err := upd.Save(ctx); err != nil {
			_ = tx.Rollback()
			slog.Error("open token bag registration failed", "err", err)
			return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
		}

		// Same transaction as the phase change: an open bag that silently lost
		// half the table would be worse than not opening at all.
		if err := backfillBagFromGrimoire(ctx, tx.Registration, g); err != nil {
			_ = tx.Rollback()
			slog.Error("backfill token bag from grimoire names failed", "err", err)
			return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
		}

		if err := tx.Commit(); err != nil {
			slog.Error("commit failed", "err", err)
			return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
		}
	}

	h.publishTokenBag(g.ID)

	bag, err := h.ownerTokenBag(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&clockkeeperv1.OpenTokenBagRegistrationResponse{TokenBag: bag}), nil
}

// CloseTokenBagRegistration stops new players from joining. Neighbor picks
// happen after this point.
func (h *ClockKeeperServiceHandler) CloseTokenBagRegistration(ctx context.Context, req *connect.Request[clockkeeperv1.CloseTokenBagRegistrationRequest]) (*connect.Response[clockkeeperv1.CloseTokenBagRegistrationResponse], error) {
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	if g.State == game.StateCompleted {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("game is completed"))
	}

	if g.TokenBagPhase != game.TokenBagPhaseOpen {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("token bag registration is not open"))
	}

	if _, err := g.Update().SetTokenBagPhase(game.TokenBagPhaseClosed).Save(ctx); err != nil {
		slog.Error("close token bag registration failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	h.publishTokenBag(g.ID)

	bag, err := h.ownerTokenBag(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&clockkeeperv1.CloseTokenBagRegistrationResponse{TokenBag: bag}), nil
}

// AddTokenBagRegistration puts a player in the bag on the storyteller's behalf,
// for the players at the table who never touched a phone or a shared device.
// Such a registration is indistinguishable from a shared-device one: it carries
// no reachable secret, so its token is revealed by tapping the name on the
// shared device.
func (h *ClockKeeperServiceHandler) AddTokenBagRegistration(ctx context.Context, req *connect.Request[clockkeeperv1.AddTokenBagRegistrationRequest]) (*connect.Response[clockkeeperv1.AddTokenBagRegistrationResponse], error) {
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	if g.State == game.StateCompleted {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("game is completed"))
	}

	// Unlike the players themselves, the storyteller may still add someone after
	// closing registration — that is the point of this RPC.
	if g.TokenBagPhase != game.TokenBagPhaseOpen && g.TokenBagPhase != game.TokenBagPhaseClosed {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("players can only be added while the token bag is open or closed"))
	}

	// The secret is of no use to anyone: nobody is holding a device to store it.
	if _, err := h.createRegistration(ctx, g, req.Msg.Name, true, game.TokenBagPhaseOpen, game.TokenBagPhaseClosed); err != nil {
		return nil, err
	}

	h.publishTokenBag(g.ID)

	bag, err := h.ownerTokenBag(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&clockkeeperv1.AddTokenBagRegistrationResponse{TokenBag: bag}), nil
}

// RemoveTokenBagRegistration drops a player from the bag (duplicate join, someone
// who left) and clears the neighbor picks that pointed at them.
func (h *ClockKeeperServiceHandler) RemoveTokenBagRegistration(ctx context.Context, req *connect.Request[clockkeeperv1.RemoveTokenBagRegistrationRequest]) (*connect.Response[clockkeeperv1.RemoveTokenBagRegistrationResponse], error) {
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	if g.TokenBagPhase != game.TokenBagPhaseOpen && g.TokenBagPhase != game.TokenBagPhaseClosed {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("players can only be removed while the token bag is open or closed"))
	}

	regID := int(req.Msg.RegistrationId)
	notFound := connect.NewError(connect.CodeNotFound, errors.New("registration not found"))
	r, err := h.db.Registration.Get(ctx, regID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, notFound
		}
		slog.Error("get registration failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}
	if r.GameID != g.ID {
		return nil, notFound
	}

	tx, err := h.db.Tx(ctx)
	if err != nil {
		slog.Error("start transaction failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	if err := tx.Registration.DeleteOneID(regID).Exec(ctx); err != nil {
		_ = tx.Rollback()
		slog.Error("delete registration failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	// Drop the dangling neighbor references the removed player left behind.
	if _, err := tx.Registration.Update().
		Where(registration.GameID(g.ID), registration.LeftNeighborID(regID)).
		ClearLeftNeighborID().
		Save(ctx); err != nil {
		_ = tx.Rollback()
		slog.Error("clear left neighbor references failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}
	if _, err := tx.Registration.Update().
		Where(registration.GameID(g.ID), registration.RightNeighborID(regID)).
		ClearRightNeighborID().
		Save(ctx); err != nil {
		_ = tx.Rollback()
		slog.Error("clear right neighbor references failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	if err := tx.Commit(); err != nil {
		slog.Error("commit failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	h.publishTokenBag(g.ID)

	bag, err := h.ownerTokenBag(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&clockkeeperv1.RemoveTokenBagRegistrationResponse{TokenBag: bag}), nil
}

// RevealTokenBag snapshots the grimoire's role_id -> player name map onto the
// registrations so every player can fetch their own token. Names are matched
// case- and whitespace-insensitively; every registered player must have exactly
// one role, while grimoire names nobody registered under are ignored (the
// storyteller may have typed names for players sitting without a phone).
//
// Only roles ACTUALLY IN PLAY (selected_roles + selected_travellers) can be
// dealt out. grimoire_player_names is never pruned when the storyteller swaps a
// character out of the script, so it keeps stale role_id keys around; handing a
// player a token off one of those would reveal a character nobody holds.
//
// Dealing tokens is part of setting a game up, so a running game refuses it: the
// grimoire has moved on (deaths, star passes), and the token every player was
// handed at the start of the evening must not be re-dealt off it.
func (h *ClockKeeperServiceHandler) RevealTokenBag(ctx context.Context, req *connect.Request[clockkeeperv1.RevealTokenBagRequest]) (*connect.Response[clockkeeperv1.RevealTokenBagResponse], error) {
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	// Completed is a state too, checked first for the message it deserves.
	if g.State == game.StateCompleted {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("game is completed"))
	}
	if g.State != game.StateSetup {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("tokens can only be revealed during setup"))
	}

	if g.TokenBagPhase != game.TokenBagPhaseClosed {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("close token bag registration before revealing"))
	}

	// Seats that exist: the roles and travellers the storyteller put in play.
	inPlay := make(map[string]bool, len(g.SelectedRoles)+len(g.SelectedTravellers))
	var unknownRoles []string
	for _, roleID := range slices.Concat(g.SelectedRoles, g.SelectedTravellers) {
		if inPlay[roleID] {
			continue
		}
		inPlay[roleID] = true
		if _, ok := h.registry.Character(roleID); !ok {
			// A seat the registry cannot resolve would blow up as a CodeInternal
			// the moment its player asked for their token. Better to refuse the
			// reveal and tell the storyteller which id is broken.
			unknownRoles = append(unknownRoles, roleID)
		}
	}
	if len(unknownRoles) > 0 {
		sort.Strings(unknownRoles)
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("these roles are in play but unknown: %s — fix the script selection before revealing", strings.Join(unknownRoles, ", ")))
	}

	// Invert the grimoire map: normalized player name -> role ids.
	rolesByName := make(map[string][]string, len(g.GrimoirePlayerNames))
	for roleID, name := range g.GrimoirePlayerNames {
		if !inPlay[roleID] {
			continue // stale key from a character that left the script
		}
		_, normalized, err := normalizeName(name)
		if err != nil {
			continue // blank or oversized grimoire entry — nothing to match
		}
		rolesByName[normalized] = append(rolesByName[normalized], roleID)
	}

	tx, err := h.db.Tx(ctx)
	if err != nil {
		slog.Error("start transaction failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	regs, err := tx.Registration.Query().
		Where(registration.GameID(g.ID)).
		Order(ent.Asc(registration.FieldID)).
		All(ctx)
	if err != nil {
		_ = tx.Rollback()
		slog.Error("list registrations for reveal failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}
	if len(regs) == 0 {
		_ = tx.Rollback()
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no registered players"))
	}

	assignments := make(map[int]string, len(regs))
	var unassigned, ambiguous []string
	for _, r := range regs {
		roleIDs := rolesByName[r.NameNormalized]
		switch len(roleIDs) {
		case 0:
			unassigned = append(unassigned, r.Name)
		case 1:
			assignments[r.ID] = roleIDs[0]
		default:
			ambiguous = append(ambiguous, r.Name)
		}
	}
	if len(unassigned) > 0 {
		_ = tx.Rollback()
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("no role assigned to: %s", strings.Join(unassigned, ", ")))
	}
	if len(ambiguous) > 0 {
		_ = tx.Rollback()
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("several roles are assigned to the same player name: %s", strings.Join(ambiguous, ", ")))
	}

	for regID, roleID := range assignments {
		if _, err := tx.Registration.UpdateOneID(regID).SetAssignedRoleID(roleID).Save(ctx); err != nil {
			_ = tx.Rollback()
			slog.Error("assign role to registration failed", "err", err)
			return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
		}
	}

	if _, err := tx.Game.UpdateOneID(g.ID).SetTokenBagPhase(game.TokenBagPhaseRevealed).Save(ctx); err != nil {
		_ = tx.Rollback()
		slog.Error("set token bag phase revealed failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	if err := tx.Commit(); err != nil {
		slog.Error("commit failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	h.publishTokenBag(g.ID)

	bag, err := h.ownerTokenBag(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&clockkeeperv1.RevealTokenBagResponse{TokenBag: bag}), nil
}

// ResetTokenBag undeals the tokens: every registration loses its assigned role
// and the bag drops back to closed, from where the storyteller can re-assign the
// roles and reveal again (or re-open registration).
//
// Deliberately a SOFT reset. The use case is a setup that went wrong, and making
// fifteen people re-scan a QR code to fix it is worse than the mistake: the
// registrations survive with their neighbor picks and claimed secrets, both codes
// survive, and every phone stays connected and simply goes back to waiting.
//
// Nothing takes a bag back to inactive or rotates its codes any more, which is
// why Open only ever mints a code that is missing.
func (h *ClockKeeperServiceHandler) ResetTokenBag(ctx context.Context, req *connect.Request[clockkeeperv1.ResetTokenBagRequest]) (*connect.Response[clockkeeperv1.ResetTokenBagResponse], error) {
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	// A bag that was never opened has nothing to reset, and resetting it would
	// invent a closed phase for a game with no codes and no players.
	if g.TokenBagPhase == game.TokenBagPhaseInactive {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the token bag has not been opened yet"))
	}

	tx, err := h.db.Tx(ctx)
	if err != nil {
		slog.Error("start transaction failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	if _, err := tx.Registration.Update().
		Where(registration.GameID(g.ID)).
		ClearAssignedRoleID().
		Save(ctx); err != nil {
		_ = tx.Rollback()
		slog.Error("clear assigned roles failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	if _, err := tx.Game.UpdateOneID(g.ID).
		SetTokenBagPhase(game.TokenBagPhaseClosed).
		Save(ctx); err != nil {
		_ = tx.Rollback()
		slog.Error("reset token bag failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	if err := tx.Commit(); err != nil {
		slog.Error("commit failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	h.publishTokenBag(g.ID)

	bag, err := h.ownerTokenBag(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&clockkeeperv1.ResetTokenBagResponse{TokenBag: bag}), nil
}

func (h *ClockKeeperServiceHandler) GetTokenBag(ctx context.Context, req *connect.Request[clockkeeperv1.GetTokenBagRequest]) (*connect.Response[clockkeeperv1.GetTokenBagResponse], error) {
	bag, err := h.ownerTokenBag(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&clockkeeperv1.GetTokenBagResponse{TokenBag: bag}), nil
}

// GetTokenBagSeating derives the seating circle from the players' neighbor picks.
func (h *ClockKeeperServiceHandler) GetTokenBagSeating(ctx context.Context, req *connect.Request[clockkeeperv1.GetTokenBagSeatingRequest]) (*connect.Response[clockkeeperv1.GetTokenBagSeatingResponse], error) {
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	regs, err := h.bagRegistrations(ctx, g.ID)
	if err != nil {
		return nil, err
	}

	claims := make([]seatingClaim, len(regs))
	names := make(map[int]string, len(regs))
	for i, r := range regs {
		claims[i] = seatingClaim{ID: r.ID, LeftID: r.LeftNeighborID, RightID: r.RightNeighborID}
		names[r.ID] = r.Name
	}

	order, complete, conflicts := computeSeating(claims)

	orderedIDs := make([]int64, len(order))
	for i, id := range order {
		orderedIDs[i] = int64(id)
	}

	return connect.NewResponse(&clockkeeperv1.GetTokenBagSeatingResponse{
		OrderedRegistrationIds: orderedIDs,
		Complete:               complete,
		Conflicts:              nameSeatingConflicts(conflicts, names),
	}), nil
}

// --- Player RPCs (public — the registration secret in the payload is the credential) ---

// JoinTokenBag registers a player from their own device and returns the secret
// their device stores to identify itself later. A name that is already in the bag
// but unclaimed is taken over instead of refused — see claimRegistration.
func (h *ClockKeeperServiceHandler) JoinTokenBag(ctx context.Context, req *connect.Request[clockkeeperv1.JoinTokenBagRequest]) (*connect.Response[clockkeeperv1.JoinTokenBagResponse], error) {
	g, isShared, err := h.gameByBagCode(ctx, req.Msg.JoinCode)
	if err != nil {
		return nil, err
	}
	if isShared {
		// The shared-device code is not a player join code.
		return nil, connect.NewError(connect.CodeNotFound, errors.New("token bag not found"))
	}

	reg, err := h.claimRegistration(ctx, g, req.Msg.Name)
	if err != nil {
		return nil, err
	}
	if reg == nil {
		// Nobody holds that name yet: an ordinary new player.
		reg, err = h.createRegistration(ctx, g, req.Msg.Name, false, game.TokenBagPhaseOpen)
		if err != nil {
			return nil, err
		}
	}
	h.publishTokenBag(g.ID)

	regs, err := h.bagRegistrations(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	snapshot, err := h.buildWatchSnapshot(g, regs, reg.entity)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&clockkeeperv1.JoinTokenBagResponse{
		RegistrationId:     int64(reg.entity.ID),
		RegistrationSecret: reg.secret,
		Snapshot:           snapshot,
	}), nil
}

// SetTokenBagNeighbors records who a player sits between. Picking the same player
// on both sides is allowed — tiny circles have a single other player.
func (h *ClockKeeperServiceHandler) SetTokenBagNeighbors(ctx context.Context, req *connect.Request[clockkeeperv1.SetTokenBagNeighborsRequest]) (*connect.Response[clockkeeperv1.SetTokenBagNeighborsResponse], error) {
	r, g, err := h.registrationBySecret(ctx, req.Msg.RegistrationSecret)
	if err != nil {
		return nil, err
	}

	if g.TokenBagPhase != game.TokenBagPhaseClosed {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("neighbors can only be picked after registration closes and before the reveal"))
	}

	leftID := int(req.Msg.LeftRegistrationId)
	rightID := int(req.Msg.RightRegistrationId)
	for _, id := range []int{leftID, rightID} {
		if id == 0 {
			continue // 0 clears the pick
		}
		if err := h.validateNeighbor(ctx, r, id); err != nil {
			return nil, err
		}
	}

	upd := r.Update()
	if leftID == 0 {
		upd = upd.ClearLeftNeighborID()
	} else {
		upd = upd.SetLeftNeighborID(leftID)
	}
	if rightID == 0 {
		upd = upd.ClearRightNeighborID()
	} else {
		upd = upd.SetRightNeighborID(rightID)
	}
	r, err = upd.Save(ctx)
	if err != nil {
		slog.Error("save neighbor picks failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}
	h.publishTokenBag(g.ID)

	regs, err := h.bagRegistrations(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	snapshot, err := h.buildWatchSnapshot(g, regs, r)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&clockkeeperv1.SetTokenBagNeighborsResponse{Snapshot: snapshot}), nil
}

// GetMyToken returns the character a player was dealt, once the storyteller has
// revealed the bag — and only while the game is still being set up. The re-show
// window closes when the game starts: from then on the token is a physical one
// on the table, and the bag must not hand out a second copy of it.
func (h *ClockKeeperServiceHandler) GetMyToken(ctx context.Context, req *connect.Request[clockkeeperv1.GetMyTokenRequest]) (*connect.Response[clockkeeperv1.GetMyTokenResponse], error) {
	r, g, err := h.registrationBySecret(ctx, req.Msg.RegistrationSecret)
	if err != nil {
		return nil, err
	}

	if g.State != game.StateSetup {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the game has started"))
	}

	if g.TokenBagPhase != game.TokenBagPhaseRevealed || r.AssignedRoleID == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("tokens have not been revealed yet"))
	}

	c, err := resolveTokenCharacter(g, r.AssignedRoleID, h.registry)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&clockkeeperv1.GetMyTokenResponse{Character: characterToProto(c)}), nil
}

// --- Shared device RPCs (public — the shared code is the credential) ---

// JoinTokenBagShared registers a player from the storyteller's shared device, for
// players without a phone. No secret is handed out: their token is revealed by
// tapping their name on the shared device.
func (h *ClockKeeperServiceHandler) JoinTokenBagShared(ctx context.Context, req *connect.Request[clockkeeperv1.JoinTokenBagSharedRequest]) (*connect.Response[clockkeeperv1.JoinTokenBagSharedResponse], error) {
	g, err := h.sharedCodeGame(ctx, req.Msg.SharedCode)
	if err != nil {
		return nil, err
	}

	reg, err := h.createRegistration(ctx, g, req.Msg.Name, true, game.TokenBagPhaseOpen)
	if err != nil {
		return nil, err
	}
	h.publishTokenBag(g.ID)

	return connect.NewResponse(&clockkeeperv1.JoinTokenBagSharedResponse{
		RegistrationId: int64(reg.entity.ID),
	}), nil
}

// RevealTokenShared reveals exactly one player's token on the shared device.
// Setup only, for the same reason as GetMyToken: once the game is running, the
// tablet passed around the table must not show anybody's character again.
func (h *ClockKeeperServiceHandler) RevealTokenShared(ctx context.Context, req *connect.Request[clockkeeperv1.RevealTokenSharedRequest]) (*connect.Response[clockkeeperv1.RevealTokenSharedResponse], error) {
	g, err := h.sharedCodeGame(ctx, req.Msg.SharedCode)
	if err != nil {
		return nil, err
	}

	if g.State != game.StateSetup {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the game has started"))
	}

	if g.TokenBagPhase != game.TokenBagPhaseRevealed {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("tokens have not been revealed yet"))
	}

	notFound := connect.NewError(connect.CodeNotFound, errors.New("registration not found"))
	r, err := h.db.Registration.Get(ctx, int(req.Msg.RegistrationId))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, notFound
		}
		slog.Error("get registration for shared reveal failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}
	if r.GameID != g.ID {
		return nil, notFound
	}

	c, err := resolveTokenCharacter(g, r.AssignedRoleID, h.registry)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&clockkeeperv1.RevealTokenSharedResponse{
		Name:      r.Name,
		Character: characterToProto(c),
	}), nil
}

// --- Shared internals ---

// ownerTokenBag re-reads the game (Save() drops eager-loaded edges) plus its
// registrations and renders the storyteller's view. It repeats the ownership
// check, so callers must already hold it.
func (h *ClockKeeperServiceHandler) ownerTokenBag(ctx context.Context, gameID int) (*clockkeeperv1.TokenBag, error) {
	g, err := h.getOwnedGame(ctx, gameID)
	if err != nil {
		return nil, err
	}
	regs, err := h.bagRegistrations(ctx, gameID)
	if err != nil {
		return nil, err
	}
	return tokenBagToProto(g, regs), nil
}

// sharedCodeGame resolves a shared-device code. Player join codes are rejected
// with the same CodeNotFound as unknown codes.
func (h *ClockKeeperServiceHandler) sharedCodeGame(ctx context.Context, code string) (*ent.Game, error) {
	g, isShared, err := h.gameByBagCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if !isShared {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("token bag not found"))
	}
	return g, nil
}

// newRegistration carries a created registration together with its raw secret,
// which exists only for the duration of the joining request.
type newRegistration struct {
	entity *ent.Registration
	secret string
}

// createRegistration validates the name and adds the player to the bag. A
// secret is always generated (the schema requires its hash); shared-device joins
// discard the raw value. allowedPhases is passed on to bagJoinable.
func (h *ClockKeeperServiceHandler) createRegistration(ctx context.Context, g *ent.Game, rawName string, viaSharedDevice bool, allowedPhases ...game.TokenBagPhase) (*newRegistration, error) {
	if err := bagJoinable(g, allowedPhases...); err != nil {
		return nil, err
	}

	display, normalized, err := normalizeName(rawName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	count, err := h.db.Registration.Query().Where(registration.GameID(g.ID)).Count(ctx)
	if err != nil {
		slog.Error("count registrations failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}
	if count >= maxTokenBagRegistrations {
		return nil, connect.NewError(connect.CodeResourceExhausted, errors.New("the token bag is full"))
	}

	secret, err := newRegistrationSecret()
	if err != nil {
		slog.Error("generate registration secret failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	r, err := h.db.Registration.Create().
		SetGameID(g.ID).
		SetName(display).
		SetNameNormalized(normalized).
		SetSecretHash(hashSecret(secret)).
		SetViaSharedDevice(viaSharedDevice).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("name already taken"))
		}
		if ent.IsValidationError(err) {
			// Defensive: normalizeName already enforces the schema's bounds, so
			// this only fires if the two ever drift apart. Still a bad request,
			// never a 500 — this endpoint is reachable without authentication.
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is not valid"))
		}
		slog.Error("create registration failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return &newRegistration{entity: r, secret: secret}, nil
}

// bagJoinable reports whether the bag accepts a name at all right now.
// allowedPhases are the token bag phases the caller accepts — the public join
// paths only ever allow OPEN, while the storyteller may also add players to a
// CLOSED bag.
func bagJoinable(g *ent.Game, allowedPhases ...game.TokenBagPhase) error {
	if g.State == game.StateCompleted {
		// A finished game's QR codes are still floating around on paper; joining
		// one must not silently work. Covers every way into the bag.
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("game is completed"))
	}

	if !slices.Contains(allowedPhases, g.TokenBagPhase) {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("token bag registration is not open"))
	}
	return nil
}

// claimRegistration hands a registration that carries no reachable secret — one
// the storyteller typed in, one backfilled from the grimoire, one made on the
// shared device — to the player who joins under its name from their own phone.
// Everything else on the row survives, neighbor picks included; only the secret
// is replaced, so the player's device can identify itself from now on.
//
// Returns (nil, nil) when the name is free: the caller registers a new player
// instead. A name already claimed by a device stays taken.
func (h *ClockKeeperServiceHandler) claimRegistration(ctx context.Context, g *ent.Game, rawName string) (*newRegistration, error) {
	if err := bagJoinable(g, game.TokenBagPhaseOpen); err != nil {
		return nil, err
	}

	_, normalized, err := normalizeName(rawName)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Scoped to this game: a claim never reaches another bag's rows.
	r, err := h.db.Registration.Query().
		Where(registration.GameID(g.ID), registration.NameNormalized(normalized)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		slog.Error("look up registration by name failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	taken := connect.NewError(connect.CodeAlreadyExists, errors.New("name already taken"))
	if !r.ViaSharedDevice {
		return nil, taken
	}

	secret, err := newRegistrationSecret()
	if err != nil {
		slog.Error("generate registration secret failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	// Conditional on the row still being unclaimed: two phones racing for the
	// same name must not both walk away with a working secret.
	claimed, err := h.db.Registration.UpdateOneID(r.ID).
		Where(registration.ViaSharedDevice(true)).
		SetSecretHash(hashSecret(secret)).
		SetViaSharedDevice(false).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, taken // another device claimed it a moment ago
		}
		slog.Error("claim registration failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	return &newRegistration{entity: claimed, secret: secret}, nil
}

// backfillBagFromGrimoire adds a registration for every in-play grimoire seat
// name that has none yet. A storyteller who typed the table's names into the
// grimoire before opening the bag — or duplicated last week's game, which copies
// the names but not the bag — would otherwise have to type them all again.
//
// The backfilled registrations are shared-device ones, exactly like the ones the
// storyteller adds by hand: a secret is generated because the schema requires its
// hash, and discarded, because no device is holding it. The player it belongs to
// either taps their name on the shared device or claims it from their own phone
// (see claimRegistration).
//
// Silent about everything it skips — unusable names, names already in the bag,
// everything past the cap. Opening the bag must not fail over a stale grimoire
// entry.
func backfillBagFromGrimoire(ctx context.Context, regs *ent.RegistrationClient, g *ent.Game) error {
	if len(g.GrimoirePlayerNames) == 0 {
		return nil
	}

	// Same filter as the reveal: only the seats actually in play are players.
	inPlay := make(map[string]bool, len(g.SelectedRoles)+len(g.SelectedTravellers))
	for _, roleID := range slices.Concat(g.SelectedRoles, g.SelectedTravellers) {
		inPlay[roleID] = true
	}

	existing, err := regs.Query().Where(registration.GameID(g.ID)).All(ctx)
	if err != nil {
		return err
	}
	taken := make(map[string]bool, len(existing))
	for _, r := range existing {
		taken[r.NameNormalized] = true
	}
	count := len(existing)

	// Sorted by role id: which names still fit under the cap must not depend on
	// Go's map iteration order.
	for _, roleID := range slices.Sorted(maps.Keys(g.GrimoirePlayerNames)) {
		if count >= maxTokenBagRegistrations {
			return nil
		}
		if !inPlay[roleID] {
			continue // stale key from a character that left the script
		}
		display, normalized, err := normalizeName(g.GrimoirePlayerNames[roleID])
		if err != nil || taken[normalized] {
			continue
		}

		secret, err := newRegistrationSecret()
		if err != nil {
			return err
		}
		if _, err := regs.Create().
			SetGameID(g.ID).
			SetName(display).
			SetNameNormalized(normalized).
			SetSecretHash(hashSecret(secret)).
			SetViaSharedDevice(true).
			Save(ctx); err != nil {
			return err
		}
		taken[normalized] = true
		count++
	}
	return nil
}

// validateNeighbor checks that a picked neighbor is another player in the same
// game.
func (h *ClockKeeperServiceHandler) validateNeighbor(ctx context.Context, self *ent.Registration, neighborID int) error {
	if neighborID == self.ID {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("cannot pick yourself as a neighbor"))
	}

	invalid := connect.NewError(connect.CodeInvalidArgument, errors.New("neighbor is not a player in this game"))
	n, err := h.db.Registration.Get(ctx, neighborID)
	if err != nil {
		if ent.IsNotFound(err) {
			return invalid
		}
		slog.Error("get neighbor registration failed", "err", err)
		return connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}
	if n.GameID != self.GameID {
		return invalid
	}
	return nil
}
