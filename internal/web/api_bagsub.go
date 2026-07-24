package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"github.com/loomi-labs/clockkeeper/ent"
	"github.com/loomi-labs/clockkeeper/ent/death"
	"github.com/loomi-labs/clockkeeper/ent/game"
	"github.com/loomi-labs/clockkeeper/ent/phase"
	"github.com/loomi-labs/clockkeeper/ent/schema"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
	"github.com/loomi-labs/clockkeeper/internal/botc"
)

// legacyReminderKeyRe matches the old positional reminder ids of the form
// `reminder-<n>` (digits only). These ids were index-based into the reminder
// token list and shift whenever selected_roles changes, so we canonicalize
// them to the stable form before applying any seat remap.
var legacyReminderKeyRe = regexp.MustCompile(`^reminder-(\d+)$`)

// stableReminderIDs returns a stable id for each reminder token in the given
// list, matching the frontend scheme `reminder-${characterId}-${n}` where n is
// the zero-based occurrence index of that characterId within the ordered token
// list (see buildReminderTokens for the ordering: per character, Reminders then
// RemindersGlobal). Because the id is derived from the character rather than the
// token's position, it is stable across changes to selected_roles.
func stableReminderIDs(tokens []*clockkeeperv1.ReminderToken) []string {
	counts := make(map[string]int, len(tokens))
	ids := make([]string, len(tokens))
	for i, t := range tokens {
		n := counts[t.CharacterId]
		counts[t.CharacterId] = n + 1
		ids[i] = fmt.Sprintf("reminder-%s-%d", t.CharacterId, n)
	}
	return ids
}

// canonicalizeReminderKey maps a legacy positional reminder key
// (`reminder-<n>`) to its stable id using the supplied (OLD) stable-id list.
// Non-legacy keys pass through unchanged. A legacy key whose index is out of
// range references a token that no longer exists and is dropped (ok=false).
func canonicalizeReminderKey(key string, stableIDs []string) (string, bool) {
	m := legacyReminderKeyRe.FindStringSubmatch(key)
	if m == nil {
		return key, true
	}
	i, err := strconv.Atoi(m[1])
	if err != nil || i < 0 || i >= len(stableIDs) {
		return "", false
	}
	return stableIDs[i], true
}

// remapAttachmentValue rewrites the playerId segment of a reminder-attachment
// value encoded as "playerId:angle" (split at the LAST colon so player ids that
// themselves contain colons survive). Values without a colon pass through.
func remapAttachmentValue(v string, f func(string) string) string {
	idx := strings.LastIndex(v, ":")
	if idx < 0 {
		return v
	}
	return f(v[:idx]) + v[idx:]
}

// ReassignBagSubstitution reassigns a bag substitution (today only the Drunk)
// from its current seat onto a different in-play seat, swapping the two real
// roles atomically. Seats are keyed by real role id; the shown character tokens
// are invariant. If the Drunk's seat currently shows character X and the target
// seat holds real role R_B, the seat rename f is:
//
//	f(causedBy) = X       // old Drunk seat becomes the real X
//	f(R_B)      = causedBy // target seat becomes the real Drunk
//	f(other)    = other
//
// f is applied to every seat-keyed piece of state (roles, positions, names,
// notes, alignments, deaths). The synthesized "Is the Drunk" token re-attaches
// to the new Drunk seat via its derived default, so its stored keys are dropped.
func (h *ClockKeeperServiceHandler) ReassignBagSubstitution(ctx context.Context, req *connect.Request[clockkeeperv1.ReassignBagSubstitutionRequest]) (*connect.Response[clockkeeperv1.ReassignBagSubstitutionResponse], error) {
	// Ownership check before the transaction (auth gate only).
	g, err := h.getOwnedGame(ctx, int(req.Msg.GameId))
	if err != nil {
		return nil, err
	}

	causedBy := req.Msg.CausedById
	targetRoleID := req.Msg.TargetRoleId

	// State gating: allow setup + in_progress, reject only completed (mirrors UpdateGrimoireState).
	if g.State == game.StateCompleted {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("game is completed"))
	}

	// Find the bag substitution for caused_by_id.
	var sub *schema.GameBagSubstitution
	for i := range g.BagSubstitutions {
		if g.BagSubstitutions[i].CausedByID == causedBy {
			sub = &g.BagSubstitutions[i]
			break
		}
	}
	if sub == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("no bag substitution for %s", causedBy))
	}

	// X = the shown character on the current Drunk seat. Must be picked.
	shownChar := sub.CharacterID
	if shownChar == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("bag substitution has no shown character picked"))
	}

	inRoles := make(map[string]bool, len(g.SelectedRoles))
	for _, id := range g.SelectedRoles {
		inRoles[id] = true
	}

	// After the swap the shown character becomes a real role; it must not already be in play.
	if inRoles[shownChar] {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("shown character %s is already in play", shownChar))
	}

	// Target validation (InvalidArgument).
	if targetRoleID == causedBy {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("target must differ from the substitution's own seat"))
	}
	if !inRoles[targetRoleID] {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("target %s is not in play", targetRoleID))
	}
	targetChar, ok := h.registry.Character(targetRoleID)
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown character: %s", targetRoleID))
	}
	wantTeam := botc.BagTeamForCharacter(causedBy)
	if targetChar.Team != wantTeam {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("target %s is not on the %s team", targetRoleID, wantTeam))
	}
	// The target seat must not itself be a bag substitution with a shown character.
	for i := range g.BagSubstitutions {
		if g.BagSubstitutions[i].CausedByID == targetRoleID && g.BagSubstitutions[i].CharacterID != "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("target %s itself has a bag substitution", targetRoleID))
		}
	}

	// Seat rename mapping: causedBy -> shownChar, targetRoleID -> causedBy.
	mapping := map[string]string{
		causedBy:     shownChar,
		targetRoleID: causedBy,
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

	// The synthesized "Is the Drunk" reminder token re-attaches to the new Drunk
	// seat via its derived default, so its stored keys are dropped. The target
	// role stops being in play after the swap (its seat becomes the real Drunk),
	// so its reminder tokens vanish too.
	bagsubKey := "bagsub-reminder-" + causedBy
	droppedReminderPrefix := "reminder-" + targetRoleID + "-"
	dropReminderKey := func(ck string) bool {
		return ck == bagsubKey || strings.HasPrefix(ck, droppedReminderPrefix)
	}

	// Bag substitution: the seat now shows the target's (former) role. Computed
	// from the OLD subs; applySeatRename never touches bag_substitutions.
	newSubs := make([]schema.GameBagSubstitution, len(g.BagSubstitutions))
	copy(newSubs, g.BagSubstitutions)
	for i := range newSubs {
		if newSubs[i].CausedByID == causedBy {
			newSubs[i].CharacterID = targetRoleID
			newSubs[i].CharacterName = targetChar.Name
		}
	}

	// Apply the seat rename across all seat-keyed state (roles, grimoire,
	// alignments, deaths).
	if err = h.applySeatRename(ctx, tx, g, mapping, dropReminderKey); err != nil {
		_ = tx.Rollback()
		slog.Error("apply seat rename failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	// completed_actions: intentionally NOT remapped. Night-entry ids are keyed by
	// the SHOWN character id, and the set of shown characters is invariant under
	// the swap (the shown X moves seats, the target's shown role moves seats, but
	// both remain shown), so completed night actions stay valid as-is.
	//
	// selected_bluffs: left untouched. If X (now a real role) was also a bluff,
	// the existing "bluff in play" warning surfaces it; no rewrite is needed.

	if _, err = tx.Game.UpdateOneID(g.ID).SetBagSubstitutions(newSubs).Save(ctx); err != nil {
		_ = tx.Rollback()
		slog.Error("update bag substitutions failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	if err = tx.Commit(); err != nil {
		slog.Error("commit failed", "err", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal server error"))
	}

	g, err = h.getOwnedGame(ctx, g.ID)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&clockkeeperv1.ReassignBagSubstitutionResponse{
		Game: entGameToProto(g, h.registry),
	}), nil
}

// seatRenameTmpPrefix namespaces the transient role ids used while remapping
// death rows. No real character id starts with this, so temp ids never collide
// with in-play roles during the two-phase swap.
const seatRenameTmpPrefix = "__seatrename_tmp__"

// applySeatRename applies a seat-key rename (mapping used as the function f: an
// id present in the map becomes its value, ids absent are unchanged) atomically
// across every seat-keyed piece of game state: selected_roles, grimoire
// positions/player-names/game-notes/round-notes, reminder-attachment values,
// phase character_alignments on ALL phases, and death rows. It does NOT touch
// bag_substitutions, completed_actions or selected_bluffs — callers own those.
//
// Reminder-attachment KEYS and reminder-typed grimoire_position KEYS are
// canonicalized (legacy positional `reminder-<n>` -> stable `reminder-<charId>-<n>`
// against the OLD token list) but are never remapped by f, since they are
// reminder ids rather than player keys; the playerId segment of an attachment
// VALUE ("playerId:angle", split at the last colon) IS remapped. round-notes
// keys ("round:roleId", split at the first colon) have only the roleId remapped.
//
// dropReminderKey, when non-nil, is consulted with each canonicalized reminder
// key found in grimoire_positions / grimoire_reminder_attachments; matching keys
// are dropped.
//
// The rename is computed from the OLD state (a simultaneous transformation), so
// swaps/cycles like {imp: X, X: imp} apply atomically without intermediate
// collisions — including on the unique (role_id, phase) death index, which is
// kept collision-free by staging affected rows through unique temp ids.
//
// `g` must have been queried within `tx` with phases + deaths eager-loaded.
func (h *ClockKeeperServiceHandler) applySeatRename(ctx context.Context, tx *ent.Tx, g *ent.Game, mapping map[string]string, dropReminderKey func(canonicalKey string) bool) error {
	f := func(id string) string {
		if nid, ok := mapping[id]; ok {
			return nid
		}
		return id
	}

	// Compute stable reminder ids from the OLD token list (before mutating roles)
	// so legacy positional reminder keys can be canonicalized correctly.
	oldStableIDs := stableReminderIDs(buildReminderTokens(g, h.registry))

	// selected_roles: elementwise f, preserving order.
	newRoles := make([]string, len(g.SelectedRoles))
	for i, id := range g.SelectedRoles {
		newRoles[i] = f(id)
	}

	// grimoire_positions: canonicalize legacy keys, apply drop predicate, then
	// f-remap keys. Player keys (real role ids) are relabelled by f; reminder
	// keys are left as they are (f is identity on them).
	// Two passes so collision semantics match the TS canonicalizeReminderKeys:
	// native (non-legacy) keys are authoritative; a canonicalized legacy key
	// only fills a slot no native key already claimed.
	newPositions := make(map[string]schema.GrimoirePosition, len(g.GrimoirePositions))
	for pass := 0; pass < 2; pass++ {
		for k, v := range g.GrimoirePositions {
			isLegacy := legacyReminderKeyRe.MatchString(k)
			if (pass == 0) == isLegacy {
				continue
			}
			ck, keep := canonicalizeReminderKey(k, oldStableIDs)
			if !keep {
				continue
			}
			if dropReminderKey != nil && dropReminderKey(ck) {
				continue
			}
			nk := f(ck)
			if _, taken := newPositions[nk]; isLegacy && taken {
				continue
			}
			newPositions[nk] = v
		}
	}

	// grimoire_reminder_attachments: keys are reminder ids only (never player
	// keys), so f is not applied to keys. The playerId segment of each value is
	// f-remapped. Same canonicalization / drops as positions.
	newAttachments := make(map[string]string, len(g.GrimoireReminderAttachments))
	for pass := 0; pass < 2; pass++ {
		for k, v := range g.GrimoireReminderAttachments {
			isLegacy := legacyReminderKeyRe.MatchString(k)
			if (pass == 0) == isLegacy {
				continue
			}
			ck, keep := canonicalizeReminderKey(k, oldStableIDs)
			if !keep {
				continue
			}
			if dropReminderKey != nil && dropReminderKey(ck) {
				continue
			}
			if _, taken := newAttachments[ck]; isLegacy && taken {
				continue
			}
			newAttachments[ck] = remapAttachmentValue(v, f)
		}
	}

	// grimoire_player_names / grimoire_game_notes: keys are real role ids.
	newNames := make(map[string]string, len(g.GrimoirePlayerNames))
	for k, v := range g.GrimoirePlayerNames {
		newNames[f(k)] = v
	}
	newGameNotes := make(map[string]string, len(g.GrimoireGameNotes))
	for k, v := range g.GrimoireGameNotes {
		newGameNotes[f(k)] = v
	}

	// grimoire_round_notes: keys are "round:roleId" (split at the FIRST colon so
	// the round survives); f-remap the roleId segment.
	newRoundNotes := make(map[string]string, len(g.GrimoireRoundNotes))
	for k, v := range g.GrimoireRoundNotes {
		idx := strings.Index(k, ":")
		if idx < 0 {
			newRoundNotes[k] = v
			continue
		}
		newRoundNotes[k[:idx+1]+f(k[idx+1:])] = v
	}

	if _, err := tx.Game.UpdateOneID(g.ID).
		SetSelectedRoles(newRoles).
		SetGrimoirePositions(newPositions).
		SetGrimoirePlayerNames(newNames).
		SetGrimoireGameNotes(newGameNotes).
		SetGrimoireRoundNotes(newRoundNotes).
		SetGrimoireReminderAttachments(newAttachments).
		Save(ctx); err != nil {
		return fmt.Errorf("update game for seat rename: %w", err)
	}

	// Phase.character_alignments keys on ALL phases.
	phaseIDs := make([]int, 0, len(g.Edges.Phases))
	for _, p := range g.Edges.Phases {
		phaseIDs = append(phaseIDs, p.ID)
		if len(p.CharacterAlignments) == 0 {
			continue
		}
		na := make(map[string]string, len(p.CharacterAlignments))
		for k, v := range p.CharacterAlignments {
			na[f(k)] = v
		}
		if _, err := tx.Phase.UpdateOneID(p.ID).SetCharacterAlignments(na).Save(ctx); err != nil {
			return fmt.Errorf("update phase alignments: %w", err)
		}
	}

	// Death.role_id rows. To stay collision-free on the unique (role_id, phase)
	// index for an arbitrary bijective mapping (renames AND swaps/cycles), stage
	// every affected row through a unique temp id first, then move each temp to
	// its final id. After the temp pass every real target slot is free.
	if len(phaseIDs) > 0 {
		sources := make([]string, 0, len(mapping))
		for old, nid := range mapping {
			if old != nid {
				sources = append(sources, old)
			}
		}
		for _, old := range sources {
			if _, err := tx.Death.Update().
				Where(death.PhaseIDIn(phaseIDs...), death.RoleIDEQ(old)).
				SetRoleID(seatRenameTmpPrefix + old).
				Save(ctx); err != nil {
				return fmt.Errorf("stage death rename: %w", err)
			}
		}
		for _, old := range sources {
			if _, err := tx.Death.Update().
				Where(death.PhaseIDIn(phaseIDs...), death.RoleIDEQ(seatRenameTmpPrefix+old)).
				SetRoleID(mapping[old]).
				Save(ctx); err != nil {
				return fmt.Errorf("finalize death rename: %w", err)
			}
		}
	}

	return nil
}
