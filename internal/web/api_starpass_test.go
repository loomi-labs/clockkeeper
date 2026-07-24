package web

import (
	"testing"

	"connectrpc.com/connect"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func starPass(t *testing.T, h *ClockKeeperServiceHandler, ownerID int, gameID int64, minion string) (*clockkeeperv1.Game, error) {
	t.Helper()
	resp, err := h.StarPass(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.StarPassRequest{
		GameId:       gameID,
		MinionRoleId: minion,
	}))
	if err != nil {
		return nil, err
	}
	return resp.Msg.Game, nil
}

// nightPhase returns the first (round 1) night phase of an in-progress game.
func nightPhase(t *testing.T, g *clockkeeperv1.Game) *clockkeeperv1.Phase {
	t.Helper()
	require.NotNil(t, g.PlayState)
	for _, p := range g.PlayState.Phases {
		if p.Type == clockkeeperv1.PhaseType_PHASE_TYPE_NIGHT {
			return p
		}
	}
	t.Fatal("no night phase found")
	return nil
}

// --- Happy path ---

func TestStarPass_FullSwap(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := testHandler(t)
	ownerID, gameID := createTestGame(t, handler)
	// happyPathRoles = [imp, poisoner, drunk, washerwoman, chef, empath, monk].
	setRoles(t, handler, ownerID, gameID, happyPathRoles)

	_, err := handler.StartGame(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.StartGameRequest{GameId: gameID}))
	require.NoError(t, err)

	before := getGame(t, handler, ownerID, gameID)
	night := nightPhase(t, before)
	firstPhaseID := before.PlayState.Phases[0].Id

	// Alignments on all phases: imp evil, poisoner good (distinct so the remap is
	// observable even though both are evil in a real game).
	_, err = handler.UpdateCharacterAlignment(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.UpdateCharacterAlignmentRequest{
		GameId: gameID, PhaseId: firstPhaseID, RoleId: "imp", Alignment: "evil", Propagate: true,
	}))
	require.NoError(t, err)
	_, err = handler.UpdateCharacterAlignment(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.UpdateCharacterAlignmentRequest{
		GameId: gameID, PhaseId: firstPhaseID, RoleId: "poisoner", Alignment: "good", Propagate: true,
	}))
	require.NoError(t, err)

	// The Imp kills himself: record a death for "imp" on the night phase.
	_, err = handler.RecordDeath(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: gameID, RoleId: "imp", PhaseId: &night.Id, Cause: clockkeeperv1.DeathCause_DEATH_CAUSE_DEMON,
	}))
	require.NoError(t, err)

	// Grimoire state keyed by the two seats + reminder attachments pointing at them.
	_, err = handler.UpdateGrimoireState(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.UpdateGrimoireStateRequest{
		GameId: gameID,
		Positions: map[string]*clockkeeperv1.Position{
			"imp":      {X: 10, Y: 10},
			"poisoner": {X: 20, Y: 20},
			"chef":     {X: 30, Y: 30},
		},
		PlayerNames: map[string]string{"imp": "Demon", "poisoner": "Minion", "chef": "Carol"},
		GameNotes:   map[string]string{"imp": "gn-imp", "poisoner": "gn-poi"},
		RoundNotes:  map[string]string{"1:imp": "rn-imp", "1:poisoner": "rn-poi"},
		ReminderAttachments: map[string]string{
			"reminder-monk-0":   "imp:0.5",      // value playerId imp -> poisoner
			"reminder-empath-0": "poisoner:0.3", // value playerId poisoner -> imp
			"reminder-chef-0":   "chef:0.1",     // unaffected
		},
	}))
	require.NoError(t, err)

	g, err := starPass(t, handler, ownerID, gameID, "poisoner")
	require.NoError(t, err)

	// selected_roles: imp<->poisoner swapped in place, order otherwise preserved.
	assert.Equal(t, []string{"poisoner", "imp", "drunk", "washerwoman", "chef", "empath", "monk"}, g.SelectedRoleIds)

	// The dead seat (old imp) now holds the minion's former role id.
	remapped := map[string]*clockkeeperv1.Death{}
	for _, p := range g.PlayState.Phases {
		if p.Id == night.Id {
			for _, d := range p.Deaths {
				remapped[d.RoleId] = d
			}
		}
	}
	require.Contains(t, remapped, "poisoner")
	assert.NotContains(t, remapped, "imp")
	assert.Equal(t, clockkeeperv1.DeathCause_DEATH_CAUSE_DEMON, remapped["poisoner"].Cause)

	// Positions / names / game notes / round notes re-keyed (imp<->poisoner).
	assert.Equal(t, float32(10), g.GrimoirePositions["poisoner"].X)
	assert.Equal(t, float32(20), g.GrimoirePositions["imp"].X)
	assert.Equal(t, float32(30), g.GrimoirePositions["chef"].X)
	assert.Equal(t, "Demon", g.GrimoirePlayerNames["poisoner"])
	assert.Equal(t, "Minion", g.GrimoirePlayerNames["imp"])
	assert.Equal(t, "Carol", g.GrimoirePlayerNames["chef"])
	assert.Equal(t, "gn-imp", g.GrimoireGameNotes["poisoner"])
	assert.Equal(t, "gn-poi", g.GrimoireGameNotes["imp"])
	assert.Equal(t, "rn-imp", g.GrimoireRoundNotes["1:poisoner"])
	assert.Equal(t, "rn-poi", g.GrimoireRoundNotes["1:imp"])

	// Reminder-attachment KEYS unchanged (they are reminder ids), VALUES remapped.
	assert.Equal(t, "poisoner:0.5", g.GrimoireReminderAttachments["reminder-monk-0"])
	assert.Equal(t, "imp:0.3", g.GrimoireReminderAttachments["reminder-empath-0"])
	assert.Equal(t, "chef:0.1", g.GrimoireReminderAttachments["reminder-chef-0"])

	// Alignments remapped on ALL phases.
	require.Len(t, g.PlayState.Phases, len(before.PlayState.Phases))
	for _, p := range g.PlayState.Phases {
		assert.Equal(t, "good", p.CharacterAlignments["imp"], "phase %d", p.Id)
		assert.Equal(t, "evil", p.CharacterAlignments["poisoner"], "phase %d", p.Id)
	}

	// Response equals a fresh GetGame.
	fresh := getGame(t, handler, ownerID, gameID)
	assert.True(t, proto.Equal(g, fresh), "star pass response should equal fresh GetGame")
}

// --- Both dead in the same phase (unique-index safety on a swap) ---

func TestStarPass_BothDeadSamePhase(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := testHandler(t)
	ownerID, gameID := createTestGame(t, handler)
	setRoles(t, handler, ownerID, gameID, happyPathRoles)

	_, err := handler.StartGame(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.StartGameRequest{GameId: gameID}))
	require.NoError(t, err)

	before := getGame(t, handler, ownerID, gameID)
	night := nightPhase(t, before)

	// Both imp and poisoner are dead in the same phase before the swap.
	_, err = handler.RecordDeath(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: gameID, RoleId: "imp", PhaseId: &night.Id, Cause: clockkeeperv1.DeathCause_DEATH_CAUSE_DEMON,
	}))
	require.NoError(t, err)
	_, err = handler.RecordDeath(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: gameID, RoleId: "poisoner", PhaseId: &night.Id, Cause: clockkeeperv1.DeathCause_DEATH_CAUSE_EXECUTION,
	}))
	require.NoError(t, err)

	g, err := starPass(t, handler, ownerID, gameID, "poisoner")
	require.NoError(t, err)

	byRole := map[string]*clockkeeperv1.Death{}
	for _, p := range g.PlayState.Phases {
		if p.Id == night.Id {
			for _, d := range p.Deaths {
				byRole[d.RoleId] = d
			}
		}
	}
	require.Len(t, byRole, 2)
	// Swap: imp's death -> poisoner (DEMON), poisoner's death -> imp (EXECUTION).
	assert.Equal(t, clockkeeperv1.DeathCause_DEATH_CAUSE_DEMON, byRole["poisoner"].Cause)
	assert.Equal(t, clockkeeperv1.DeathCause_DEATH_CAUSE_EXECUTION, byRole["imp"].Cause)
}

// --- Validation / errors ---

func TestStarPass_MinionNotInPlay(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := testHandler(t)
	ownerID, gameID := createTestGame(t, handler)
	setRoles(t, handler, ownerID, gameID, happyPathRoles)
	_, err := handler.StartGame(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.StartGameRequest{GameId: gameID}))
	require.NoError(t, err)

	// "baron" is a valid Minion but not in selected_roles.
	_, err = starPass(t, handler, ownerID, gameID, "baron")
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestStarPass_TownsfolkTarget(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := testHandler(t)
	ownerID, gameID := createTestGame(t, handler)
	setRoles(t, handler, ownerID, gameID, happyPathRoles)
	_, err := handler.StartGame(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.StartGameRequest{GameId: gameID}))
	require.NoError(t, err)

	// "washerwoman" is in play but a Townsfolk, not a Minion.
	_, err = starPass(t, handler, ownerID, gameID, "washerwoman")
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestStarPass_SetupStateRejected(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := testHandler(t)
	ownerID, gameID := createTestGame(t, handler)
	setRoles(t, handler, ownerID, gameID, happyPathRoles)
	// Not started -> still in setup.

	_, err := starPass(t, handler, ownerID, gameID, "poisoner")
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestStarPass_BlocksOtherUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := testHandler(t)
	ownerID, gameID := createTestGame(t, handler)
	setRoles(t, handler, ownerID, gameID, happyPathRoles)
	_, err := handler.StartGame(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.StartGameRequest{GameId: gameID}))
	require.NoError(t, err)

	attacker, err := handler.db.User.Create().Save(authedCtx(ownerID))
	require.NoError(t, err)

	_, err = starPass(t, handler, attacker.ID, gameID, "poisoner")
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
