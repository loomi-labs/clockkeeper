package web

import (
	"strconv"
	"strings"
	"testing"

	"connectrpc.com/connect"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// happyPathRoles is a 7-role setup that includes the Drunk (caused_by seat) and
// the Washerwoman (a Townsfolk target with two reminder tokens). Order matters
// for the elementwise selected_roles assertion.
var happyPathRoles = []string{"imp", "poisoner", "drunk", "washerwoman", "chef", "empath", "monk"}

// setRoles updates a game's selected roles via the handler (which also
// reconciles bag substitutions for setup-affecting characters like the Drunk).
func setRoles(t *testing.T, h *ClockKeeperServiceHandler, ownerID int, gameID int64, roles []string) {
	t.Helper()
	_, err := h.UpdateGameRoles(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.UpdateGameRolesRequest{
		GameId:          gameID,
		SelectedRoleIds: roles,
	}))
	require.NoError(t, err)
}

// setBagSub sets the shown character for a caused-by seat during setup.
func setBagSub(t *testing.T, h *ClockKeeperServiceHandler, ownerID int, gameID int64, subs ...*clockkeeperv1.BagSubstitution) {
	t.Helper()
	_, err := h.UpdateBagSubstitutions(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.UpdateBagSubstitutionsRequest{
		GameId:           gameID,
		BagSubstitutions: subs,
	}))
	require.NoError(t, err)
}

func getGame(t *testing.T, h *ClockKeeperServiceHandler, ownerID int, gameID int64) *clockkeeperv1.Game {
	t.Helper()
	resp, err := h.GetGame(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.GetGameRequest{Id: gameID}))
	require.NoError(t, err)
	return resp.Msg.Game
}

func reassign(t *testing.T, h *ClockKeeperServiceHandler, ownerID int, gameID int64, causedBy, target string) (*clockkeeperv1.Game, error) {
	t.Helper()
	resp, err := h.ReassignBagSubstitution(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.ReassignBagSubstitutionRequest{
		GameId:       gameID,
		CausedById:   causedBy,
		TargetRoleId: target,
	}))
	if err != nil {
		return nil, err
	}
	return resp.Msg.Game, nil
}

// --- Authorization ---

func TestReassignBagSubstitution_BlocksOtherUser(t *testing.T) {
	handler := testHandler(t)
	ownerID, gameID := createTestGame(t, handler)
	setRoles(t, handler, ownerID, gameID, happyPathRoles)
	setBagSub(t, handler, ownerID, gameID, &clockkeeperv1.BagSubstitution{CausedById: "drunk", CharacterId: "fortuneteller"})

	attacker, err := handler.db.User.Create().Save(authedCtx(ownerID))
	require.NoError(t, err)

	_, err = reassign(t, handler, attacker.ID, gameID, "drunk", "washerwoman")
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// --- Error cases ---

func TestReassignBagSubstitution_RejectsCompletedGame(t *testing.T) {
	handler := testHandler(t)
	ownerID, gameID := createTestGame(t, handler)
	setRoles(t, handler, ownerID, gameID, happyPathRoles)
	setBagSub(t, handler, ownerID, gameID, &clockkeeperv1.BagSubstitution{CausedById: "drunk", CharacterId: "fortuneteller"})

	_, err := handler.StartGame(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.StartGameRequest{GameId: gameID}))
	require.NoError(t, err)
	_, err = handler.EndGame(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.EndGameRequest{GameId: gameID}))
	require.NoError(t, err)

	_, err = reassign(t, handler, ownerID, gameID, "drunk", "washerwoman")
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestReassignBagSubstitution_NoSubstitution(t *testing.T) {
	handler := testHandler(t)
	ownerID, gameID := createTestGame(t, handler)
	// No Drunk in play -> no bag substitution for "drunk".
	setRoles(t, handler, ownerID, gameID, []string{"imp", "poisoner", "washerwoman", "chef", "empath", "monk", "fortuneteller"})

	_, err := reassign(t, handler, ownerID, gameID, "drunk", "washerwoman")
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestReassignBagSubstitution_ShownCharacterUnpicked(t *testing.T) {
	handler := testHandler(t)
	ownerID, gameID := createTestGame(t, handler)
	// UpdateGameRoles creates an EMPTY bag sub for the Drunk (no character picked).
	setRoles(t, handler, ownerID, gameID, happyPathRoles)

	_, err := reassign(t, handler, ownerID, gameID, "drunk", "washerwoman")
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestReassignBagSubstitution_ShownCharacterAlreadyInPlay(t *testing.T) {
	handler := testHandler(t)
	ownerID, gameID := createTestGame(t, handler)
	setRoles(t, handler, ownerID, gameID, happyPathRoles)
	// "chef" is already a selected role.
	setBagSub(t, handler, ownerID, gameID, &clockkeeperv1.BagSubstitution{CausedById: "drunk", CharacterId: "chef"})

	_, err := reassign(t, handler, ownerID, gameID, "drunk", "washerwoman")
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestReassignBagSubstitution_TargetEqualsCausedBy(t *testing.T) {
	handler := testHandler(t)
	ownerID, gameID := createTestGame(t, handler)
	setRoles(t, handler, ownerID, gameID, happyPathRoles)
	setBagSub(t, handler, ownerID, gameID, &clockkeeperv1.BagSubstitution{CausedById: "drunk", CharacterId: "fortuneteller"})

	_, err := reassign(t, handler, ownerID, gameID, "drunk", "drunk")
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestReassignBagSubstitution_TargetNotInPlay(t *testing.T) {
	handler := testHandler(t)
	ownerID, gameID := createTestGame(t, handler)
	setRoles(t, handler, ownerID, gameID, happyPathRoles)
	setBagSub(t, handler, ownerID, gameID, &clockkeeperv1.BagSubstitution{CausedById: "drunk", CharacterId: "fortuneteller"})

	// "undertaker" is a valid townsfolk but not in selected_roles.
	_, err := reassign(t, handler, ownerID, gameID, "drunk", "undertaker")
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestReassignBagSubstitution_TargetWrongTeam(t *testing.T) {
	handler := testHandler(t)
	ownerID, gameID := createTestGame(t, handler)
	setRoles(t, handler, ownerID, gameID, happyPathRoles)
	setBagSub(t, handler, ownerID, gameID, &clockkeeperv1.BagSubstitution{CausedById: "drunk", CharacterId: "fortuneteller"})

	// "poisoner" is a Minion, in play, but the Drunk's bag team is Townsfolk.
	_, err := reassign(t, handler, ownerID, gameID, "drunk", "poisoner")
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestReassignBagSubstitution_TargetHasOwnSubstitution(t *testing.T) {
	handler := testHandler(t)
	ownerID, gameID := createTestGame(t, handler)
	setRoles(t, handler, ownerID, gameID, happyPathRoles)
	// The target (washerwoman) itself carries a bag substitution with a shown character.
	setBagSub(t, handler, ownerID, gameID,
		&clockkeeperv1.BagSubstitution{CausedById: "drunk", CharacterId: "fortuneteller"},
		&clockkeeperv1.BagSubstitution{CausedById: "washerwoman", CharacterId: "chef"},
	)

	_, err := reassign(t, handler, ownerID, gameID, "drunk", "washerwoman")
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// --- Setup-state happy path (no phases) ---

func TestReassignBagSubstitution_SetupHappyPath(t *testing.T) {
	handler := testHandler(t)
	ownerID, gameID := createTestGame(t, handler)
	setRoles(t, handler, ownerID, gameID, happyPathRoles)
	setBagSub(t, handler, ownerID, gameID, &clockkeeperv1.BagSubstitution{CausedById: "drunk", CharacterId: "fortuneteller"})

	// Seat positions + names in setup state.
	_, err := handler.UpdateGrimoireState(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.UpdateGrimoireStateRequest{
		GameId: gameID,
		Positions: map[string]*clockkeeperv1.Position{
			"drunk":       {X: 10, Y: 10},
			"washerwoman": {X: 20, Y: 20},
			"chef":        {X: 30, Y: 30},
		},
		PlayerNames: map[string]string{"drunk": "Alice", "washerwoman": "Bob"},
	}))
	require.NoError(t, err)

	g, err := reassign(t, handler, ownerID, gameID, "drunk", "washerwoman")
	require.NoError(t, err)

	// selected_roles: drunk -> fortuneteller, washerwoman -> drunk, order preserved.
	assert.Equal(t, []string{"imp", "poisoner", "fortuneteller", "drunk", "chef", "empath", "monk"}, g.SelectedRoleIds)

	// Bag sub now shows the target's former role.
	require.Len(t, g.BagSubstitutions, 1)
	assert.Equal(t, "drunk", g.BagSubstitutions[0].CausedById)
	assert.Equal(t, "washerwoman", g.BagSubstitutions[0].CharacterId)
	assert.Equal(t, "Washerwoman", g.BagSubstitutions[0].CharacterName)

	// Seat positions re-keyed.
	assert.Equal(t, float32(10), g.GrimoirePositions["fortuneteller"].X)
	assert.Equal(t, float32(20), g.GrimoirePositions["drunk"].X)
	assert.Equal(t, float32(30), g.GrimoirePositions["chef"].X)
	assert.NotContains(t, g.GrimoirePositions, "washerwoman")

	// Names re-keyed.
	assert.Equal(t, "Alice", g.GrimoirePlayerNames["fortuneteller"])
	assert.Equal(t, "Bob", g.GrimoirePlayerNames["drunk"])

	// Response equals a fresh GetGame.
	fresh := getGame(t, handler, ownerID, gameID)
	assert.True(t, proto.Equal(g, fresh), "reassign response should equal fresh GetGame")
}

// --- Full in-progress remap happy path ---

func TestReassignBagSubstitution_FullRemap(t *testing.T) {
	handler := testHandler(t)
	ownerID, gameID := createTestGame(t, handler)
	setRoles(t, handler, ownerID, gameID, happyPathRoles)
	setBagSub(t, handler, ownerID, gameID, &clockkeeperv1.BagSubstitution{CausedById: "drunk", CharacterId: "fortuneteller"})

	// Start and advance to round 2 (phases: r1 night, r1 day, r2 night, r2 day).
	// AdvancePhase is step-wise (night -> day -> next round's night), so
	// reaching round 2 takes two advances.
	_, err := handler.StartGame(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.StartGameRequest{GameId: gameID}))
	require.NoError(t, err)
	_, err = handler.AdvancePhase(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.AdvancePhaseRequest{GameId: gameID}))
	require.NoError(t, err)
	_, err = handler.AdvancePhase(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.AdvancePhaseRequest{GameId: gameID}))
	require.NoError(t, err)

	before := getGame(t, handler, ownerID, gameID)
	require.NotNil(t, before.PlayState)
	require.Len(t, before.PlayState.Phases, 4)
	firstPhaseID := before.PlayState.Phases[0].Id

	// Active night phase (round 2) for deaths + completed actions.
	var r2Night *clockkeeperv1.Phase
	for _, p := range before.PlayState.Phases {
		if p.IsActive && p.Type == clockkeeperv1.PhaseType_PHASE_TYPE_NIGHT {
			r2Night = p
			break
		}
	}
	require.NotNil(t, r2Night)

	// Alignments on ALL phases (propagate from the first phase).
	_, err = handler.UpdateCharacterAlignment(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.UpdateCharacterAlignmentRequest{
		GameId: gameID, PhaseId: firstPhaseID, RoleId: "drunk", Alignment: "evil", Propagate: true,
	}))
	require.NoError(t, err)
	_, err = handler.UpdateCharacterAlignment(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.UpdateCharacterAlignmentRequest{
		GameId: gameID, PhaseId: firstPhaseID, RoleId: "washerwoman", Alignment: "good", Propagate: true,
	}))
	require.NoError(t, err)

	// Both the Drunk seat and the target die in the SAME phase (unique-index path).
	_, err = handler.RecordDeath(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: gameID, RoleId: "drunk", PhaseId: &r2Night.Id, Cause: clockkeeperv1.DeathCause_DEATH_CAUSE_DEMON,
	}))
	require.NoError(t, err)
	_, err = handler.RecordDeath(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: gameID, RoleId: "washerwoman", PhaseId: &r2Night.Id, Cause: clockkeeperv1.DeathCause_DEATH_CAUSE_EXECUTION,
	}))
	require.NoError(t, err)

	// A completed night action (opaque, keyed by shown character id).
	_, err = handler.ToggleNightAction(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.ToggleNightActionRequest{
		GameId: gameID, PhaseId: r2Night.Id, ActionId: "fortuneteller", Done: true,
	}))
	require.NoError(t, err)

	// Compute OLD stable reminder ids to seed a legacy positional key.
	beforeTokens := getGame(t, handler, ownerID, gameID).ReminderTokens
	oldStableIDs := stableReminderIDs(beforeTokens)
	require.NotEmpty(t, oldStableIDs)
	legacyIdx := -1
	for i, id := range oldStableIDs {
		if id != "reminder-drunk-0" && !strings.HasPrefix(id, "reminder-washerwoman-") {
			legacyIdx = i
			break
		}
	}
	require.GreaterOrEqual(t, legacyIdx, 0, "expected a surviving reminder token to alias with a legacy key")
	legacyStableID := oldStableIDs[legacyIdx]

	// Grimoire state: seat keys, stable + legacy reminder keys, bagsub key, attachments.
	_, err = handler.UpdateGrimoireState(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.UpdateGrimoireStateRequest{
		GameId: gameID,
		Positions: map[string]*clockkeeperv1.Position{
			"drunk":                  {X: 10, Y: 10},
			"washerwoman":            {X: 20, Y: 20},
			"chef":                   {X: 30, Y: 30},
			"reminder-drunk-0":       {X: 1, Y: 1},
			"reminder-washerwoman-0": {X: 2, Y: 2},
			"reminder-washerwoman-1": {X: 3, Y: 3},
			"bagsub-reminder-drunk":  {X: 4, Y: 4},
			"reminder-" + strconv.Itoa(legacyIdx): {X: 99, Y: 99}, // legacy positional key
		},
		PlayerNames: map[string]string{"drunk": "Alice", "washerwoman": "Bob", "chef": "Carol"},
		GameNotes:   map[string]string{"drunk": "gn-drunk", "washerwoman": "gn-ww"},
		RoundNotes:  map[string]string{"1:drunk": "rn1", "2:washerwoman": "rn2"},
		ReminderAttachments: map[string]string{
			"reminder-monk-0":        "drunk:0.5",
			"reminder-drunk-0":       "washerwoman:1",
			"reminder-washerwoman-0": "chef:0.3",
			"bagsub-reminder-drunk":  "drunk:0.25",
		},
	}))
	require.NoError(t, err)

	g, err := reassign(t, handler, ownerID, gameID, "drunk", "washerwoman")
	require.NoError(t, err)

	// selected_roles order.
	assert.Equal(t, []string{"imp", "poisoner", "fortuneteller", "drunk", "chef", "empath", "monk"}, g.SelectedRoleIds)

	// Bag substitution.
	require.Len(t, g.BagSubstitutions, 1)
	assert.Equal(t, "drunk", g.BagSubstitutions[0].CausedById)
	assert.Equal(t, "Drunk", g.BagSubstitutions[0].CausedByName)
	assert.Equal(t, "washerwoman", g.BagSubstitutions[0].CharacterId)
	assert.Equal(t, "Washerwoman", g.BagSubstitutions[0].CharacterName)
	assert.Equal(t, "townsfolk", g.BagSubstitutions[0].Team)

	// Positions re-keyed; dropped keys absent; legacy canonicalized; drunk token kept.
	assert.Equal(t, float32(10), g.GrimoirePositions["fortuneteller"].X)
	assert.Equal(t, float32(20), g.GrimoirePositions["drunk"].X)
	assert.Equal(t, float32(30), g.GrimoirePositions["chef"].X)
	assert.NotContains(t, g.GrimoirePositions, "washerwoman")
	assert.NotContains(t, g.GrimoirePositions, "bagsub-reminder-drunk")
	assert.NotContains(t, g.GrimoirePositions, "reminder-washerwoman-0")
	assert.NotContains(t, g.GrimoirePositions, "reminder-washerwoman-1")
	assert.Equal(t, float32(1), g.GrimoirePositions["reminder-drunk-0"].X)
	require.Contains(t, g.GrimoirePositions, legacyStableID)
	assert.Equal(t, float32(99), g.GrimoirePositions[legacyStableID].X)

	// Attachment values remapped; dropped keys absent.
	assert.Equal(t, "fortuneteller:0.5", g.GrimoireReminderAttachments["reminder-monk-0"])
	assert.Equal(t, "drunk:1", g.GrimoireReminderAttachments["reminder-drunk-0"])
	assert.NotContains(t, g.GrimoireReminderAttachments, "reminder-washerwoman-0")
	assert.NotContains(t, g.GrimoireReminderAttachments, "bagsub-reminder-drunk")

	// Names / game notes / round notes re-keyed.
	assert.Equal(t, "Alice", g.GrimoirePlayerNames["fortuneteller"])
	assert.Equal(t, "Bob", g.GrimoirePlayerNames["drunk"])
	assert.Equal(t, "Carol", g.GrimoirePlayerNames["chef"])
	assert.Equal(t, "gn-drunk", g.GrimoireGameNotes["fortuneteller"])
	assert.Equal(t, "gn-ww", g.GrimoireGameNotes["drunk"])
	assert.Equal(t, "rn1", g.GrimoireRoundNotes["1:fortuneteller"])
	assert.Equal(t, "rn2", g.GrimoireRoundNotes["2:drunk"])
	assert.NotContains(t, g.GrimoireRoundNotes, "1:drunk")
	assert.NotContains(t, g.GrimoireRoundNotes, "2:washerwoman")

	// Alignments remapped on ALL phases.
	require.NotNil(t, g.PlayState)
	require.Len(t, g.PlayState.Phases, 4)
	for _, p := range g.PlayState.Phases {
		assert.Equal(t, "evil", p.CharacterAlignments["fortuneteller"], "phase %d", p.Id)
		assert.Equal(t, "good", p.CharacterAlignments["drunk"], "phase %d", p.Id)
		assert.NotContains(t, p.CharacterAlignments, "washerwoman", "phase %d", p.Id)
	}

	// Deaths remapped in the shared phase, ghost votes + causes preserved.
	var remapped []*clockkeeperv1.Death
	for _, p := range g.PlayState.Phases {
		if p.Id == r2Night.Id {
			remapped = p.Deaths
		}
	}
	require.Len(t, remapped, 2)
	byRole := map[string]*clockkeeperv1.Death{}
	for _, d := range remapped {
		byRole[d.RoleId] = d
	}
	require.Contains(t, byRole, "fortuneteller")
	require.Contains(t, byRole, "drunk")
	assert.NotContains(t, byRole, "washerwoman")
	assert.Equal(t, clockkeeperv1.DeathCause_DEATH_CAUSE_DEMON, byRole["fortuneteller"].Cause)
	assert.True(t, byRole["fortuneteller"].GhostVote)
	assert.Equal(t, clockkeeperv1.DeathCause_DEATH_CAUSE_EXECUTION, byRole["drunk"].Cause)
	assert.True(t, byRole["drunk"].GhostVote)

	// completed_actions unchanged (byte-identical).
	for _, p := range g.PlayState.Phases {
		if p.Id == r2Night.Id {
			assert.Equal(t, []string{"fortuneteller"}, p.CompletedActions)
		}
	}

	// Response equals a fresh GetGame.
	fresh := getGame(t, handler, ownerID, gameID)
	assert.True(t, proto.Equal(g, fresh), "reassign response should equal fresh GetGame")
}

// --- Both dead in the same phase (unique-index safety) ---

func TestReassignBagSubstitution_BothDeadSamePhase(t *testing.T) {
	handler := testHandler(t)
	ownerID, gameID := createTestGame(t, handler)
	setRoles(t, handler, ownerID, gameID, happyPathRoles)
	setBagSub(t, handler, ownerID, gameID, &clockkeeperv1.BagSubstitution{CausedById: "drunk", CharacterId: "fortuneteller"})

	_, err := handler.StartGame(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.StartGameRequest{GameId: gameID}))
	require.NoError(t, err)

	before := getGame(t, handler, ownerID, gameID)
	var night *clockkeeperv1.Phase
	for _, p := range before.PlayState.Phases {
		if p.Type == clockkeeperv1.PhaseType_PHASE_TYPE_NIGHT {
			night = p
			break
		}
	}
	require.NotNil(t, night)

	// Both the Drunk (causedBy) and the Washerwoman (target) die in the same phase.
	_, err = handler.RecordDeath(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: gameID, RoleId: "drunk", PhaseId: &night.Id, Cause: clockkeeperv1.DeathCause_DEATH_CAUSE_OTHER,
	}))
	require.NoError(t, err)
	_, err = handler.RecordDeath(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: gameID, RoleId: "washerwoman", PhaseId: &night.Id, Cause: clockkeeperv1.DeathCause_DEATH_CAUSE_EXECUTION,
	}))
	require.NoError(t, err)

	g, err := reassign(t, handler, ownerID, gameID, "drunk", "washerwoman")
	require.NoError(t, err)

	var deaths []*clockkeeperv1.Death
	for _, p := range g.PlayState.Phases {
		if p.Id == night.Id {
			deaths = p.Deaths
		}
	}
	require.Len(t, deaths, 2)
	roles := map[string]bool{}
	for _, d := range deaths {
		roles[d.RoleId] = true
	}
	assert.True(t, roles["fortuneteller"])
	assert.True(t, roles["drunk"])
	assert.False(t, roles["washerwoman"])
}
