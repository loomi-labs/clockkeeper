package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
// Names are matched to roles through the grimoire's role_id -> player name map,
// which the storyteller fills in with the existing grimoire UI. Reveal snapshots
// that mapping onto the registrations, so later grimoire edits cannot change what
// a player already saw.

// --- Storyteller RPCs (owner-gated) ---

// OpenTokenBagRegistration opens (or re-opens) registration. Re-opening a closed
// bag keeps the players who already joined, and keeps the existing codes so QR
// codes handed out earlier keep working.
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
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("tokens are already revealed — reset the token bag first"))
	case game.TokenBagPhaseOpen:
		// Already open: idempotent success, no code rotation.
	default:
		upd := g.Update().SetTokenBagPhase(game.TokenBagPhaseOpen)
		if g.TokenBagJoinCode == nil {
			code, err := newBagCode()
			if err != nil {
				slog.Error("generate token bag join code failed", "err", err)
				return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
			}
			upd = upd.SetTokenBagJoinCode(code)
		}
		if g.TokenBagSharedCode == nil {
			code, err := newBagCode()
			if err != nil {
				slog.Error("generate token bag shared code failed", "err", err)
				return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
			}
			upd = upd.SetTokenBagSharedCode(code)
		}
		if _, err := upd.Save(ctx); err != nil {
			slog.Error("open token bag registration failed", "err", err)
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
func (h *ClockKeeperServiceHandler) RevealTokenBag(ctx context.Context, req *connect.Request[clockkeeperv1.RevealTokenBagRequest]) (*connect.Response[clockkeeperv1.RevealTokenBagResponse], error) {
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	if g.State == game.StateCompleted {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("game is completed"))
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

// ResetTokenBag wipes the bag back to its untouched state: all registrations are
// dropped and both codes are cleared, so the next open hands out fresh ones and
// old QR codes stop working.
func (h *ClockKeeperServiceHandler) ResetTokenBag(ctx context.Context, req *connect.Request[clockkeeperv1.ResetTokenBagRequest]) (*connect.Response[clockkeeperv1.ResetTokenBagResponse], error) {
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	tx, err := h.db.Tx(ctx)
	if err != nil {
		slog.Error("start transaction failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	if _, err := tx.Registration.Delete().Where(registration.GameID(g.ID)).Exec(ctx); err != nil {
		_ = tx.Rollback()
		slog.Error("delete registrations failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	if _, err := tx.Game.UpdateOneID(g.ID).
		SetTokenBagPhase(game.TokenBagPhaseInactive).
		ClearTokenBagJoinCode().
		ClearTokenBagSharedCode().
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
// their device stores to identify itself later.
func (h *ClockKeeperServiceHandler) JoinTokenBag(ctx context.Context, req *connect.Request[clockkeeperv1.JoinTokenBagRequest]) (*connect.Response[clockkeeperv1.JoinTokenBagResponse], error) {
	g, isShared, err := h.gameByBagCode(ctx, req.Msg.JoinCode)
	if err != nil {
		return nil, err
	}
	if isShared {
		// The shared-device code is not a player join code.
		return nil, connect.NewError(connect.CodeNotFound, errors.New("token bag not found"))
	}

	reg, err := h.createRegistration(ctx, g, req.Msg.Name, false)
	if err != nil {
		return nil, err
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
// revealed the bag.
func (h *ClockKeeperServiceHandler) GetMyToken(ctx context.Context, req *connect.Request[clockkeeperv1.GetMyTokenRequest]) (*connect.Response[clockkeeperv1.GetMyTokenResponse], error) {
	r, g, err := h.registrationBySecret(ctx, req.Msg.RegistrationSecret)
	if err != nil {
		return nil, err
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

	reg, err := h.createRegistration(ctx, g, req.Msg.Name, true)
	if err != nil {
		return nil, err
	}
	h.publishTokenBag(g.ID)

	return connect.NewResponse(&clockkeeperv1.JoinTokenBagSharedResponse{
		RegistrationId: int64(reg.entity.ID),
	}), nil
}

// RevealTokenShared reveals exactly one player's token on the shared device.
func (h *ClockKeeperServiceHandler) RevealTokenShared(ctx context.Context, req *connect.Request[clockkeeperv1.RevealTokenSharedRequest]) (*connect.Response[clockkeeperv1.RevealTokenSharedResponse], error) {
	g, err := h.sharedCodeGame(ctx, req.Msg.SharedCode)
	if err != nil {
		return nil, err
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

// createRegistration validates the name and adds the player to an open bag. A
// secret is always generated (the schema requires its hash); shared-device joins
// discard the raw value.
func (h *ClockKeeperServiceHandler) createRegistration(ctx context.Context, g *ent.Game, rawName string, viaSharedDevice bool) (*newRegistration, error) {
	if g.State == game.StateCompleted {
		// A finished game's QR codes are still floating around on paper; joining
		// one must not silently work. Covers both JoinTokenBag and
		// JoinTokenBagShared, which are the only ways in.
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("game is completed"))
	}

	if g.TokenBagPhase != game.TokenBagPhaseOpen {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("token bag registration is not open"))
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
