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

func undoStarPass(t *testing.T, h *ClockKeeperServiceHandler, ownerID int, gameID int64, roleID string) (*clockkeeperv1.Game, error) {
	t.Helper()
	resp, err := h.UndoStarPass(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.UndoStarPassRequest{
		GameId: gameID,
		RoleId: roleID,
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

// deathsByRole returns the deaths of the given phase keyed by role id.
func deathsByRole(g *clockkeeperv1.Game, phaseID int64) map[string]*clockkeeperv1.Death {
	out := map[string]*clockkeeperv1.Death{}
	for _, p := range g.PlayState.Phases {
		if p.Id == phaseID {
			for _, d := range p.Deaths {
				out[d.RoleId] = d
			}
		}
	}
	return out
}

// --- Happy path ---

func TestStarPass_RecordsPromotion(t *testing.T) {
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

	// Grimoire state keyed by real seats — must be untouched by the promotion.
	_, err = handler.UpdateGrimoireState(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.UpdateGrimoireStateRequest{
		GameId: gameID,
		Positions: map[string]*clockkeeperv1.Position{
			"imp":      {X: 10, Y: 10},
			"poisoner": {X: 20, Y: 20},
		},
		PlayerNames: map[string]string{"imp": "Demon", "poisoner": "Minion"},
	}))
	require.NoError(t, err)

	// The Imp kills himself: record a death for "imp" on the night phase.
	_, err = handler.RecordDeath(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: gameID, RoleId: "imp", PhaseId: &night.Id, Cause: clockkeeperv1.DeathCause_DEATH_CAUSE_DEMON,
	}))
	require.NoError(t, err)

	g, err := starPass(t, handler, ownerID, gameID, "poisoner")
	require.NoError(t, err)

	// A single promotion overlay entry: poisoner ACTS AS imp.
	require.Len(t, g.RolePromotions, 1)
	assert.Equal(t, "poisoner", g.RolePromotions[0].RoleId)
	assert.Equal(t, "imp", g.RolePromotions[0].ActsAsRoleId)

	// selected_roles unchanged (seats keep their real role ids).
	assert.Equal(t, happyPathRoles, g.SelectedRoleIds)

	// Death rows untouched: the imp's self-kill still sits on role "imp".
	deaths := deathsByRole(g, night.Id)
	require.Contains(t, deaths, "imp")
	assert.NotContains(t, deaths, "poisoner")
	assert.Equal(t, clockkeeperv1.DeathCause_DEATH_CAUSE_DEMON, deaths["imp"].Cause)

	// Grimoire maps untouched.
	assert.Equal(t, float32(10), g.GrimoirePositions["imp"].X)
	assert.Equal(t, float32(20), g.GrimoirePositions["poisoner"].X)
	assert.Equal(t, "Demon", g.GrimoirePlayerNames["imp"])
	assert.Equal(t, "Minion", g.GrimoirePlayerNames["poisoner"])

	// Response equals a fresh GetGame.
	fresh := getGame(t, handler, ownerID, gameID)
	assert.True(t, proto.Equal(g, fresh), "star pass response should equal fresh GetGame")
}

// --- Dead promotion target is rejected (only a living Minion can be promoted) ---

func TestStarPass_DeadMinionRejected(t *testing.T) {
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

	// The imp self-kills AND the poisoner is already dead in the active phase —
	// a dead Minion must not be promotable, even though the client filters it.
	_, err = handler.RecordDeath(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: gameID, RoleId: "imp", PhaseId: &night.Id, Cause: clockkeeperv1.DeathCause_DEATH_CAUSE_DEMON,
	}))
	require.NoError(t, err)
	_, err = handler.RecordDeath(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: gameID, RoleId: "poisoner", PhaseId: &night.Id, Cause: clockkeeperv1.DeathCause_DEATH_CAUSE_EXECUTION,
	}))
	require.NoError(t, err)

	_, err = starPass(t, handler, ownerID, gameID, "poisoner")
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))

	// Nothing recorded.
	after := getGame(t, handler, ownerID, gameID)
	assert.Empty(t, after.RolePromotions)
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

func TestStarPass_NoDemonInPlay(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := testHandler(t)
	ownerID, gameID := createTestGame(t, handler)
	// A Minion is in play (poisoner) but NO demon.
	setRoles(t, handler, ownerID, gameID, []string{"poisoner", "washerwoman", "chef", "empath", "monk", "fortuneteller", "librarian"})
	_, err := handler.StartGame(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.StartGameRequest{GameId: gameID}))
	require.NoError(t, err)

	_, err = starPass(t, handler, ownerID, gameID, "poisoner")
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestStarPass_DuplicateRejected(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := testHandler(t)
	ownerID, gameID := createTestGame(t, handler)
	setRoles(t, handler, ownerID, gameID, happyPathRoles)
	_, err := handler.StartGame(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.StartGameRequest{GameId: gameID}))
	require.NoError(t, err)

	g, err := starPass(t, handler, ownerID, gameID, "poisoner")
	require.NoError(t, err)
	require.Len(t, g.RolePromotions, 1)

	// Promoting the same seat again is rejected.
	_, err = starPass(t, handler, ownerID, gameID, "poisoner")
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
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

// --- UndoStarPass ---

func TestUndoStarPass_RemovesPromotion(t *testing.T) {
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

	_, err = handler.RecordDeath(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: gameID, RoleId: "imp", PhaseId: &night.Id, Cause: clockkeeperv1.DeathCause_DEATH_CAUSE_DEMON,
	}))
	require.NoError(t, err)

	g, err := starPass(t, handler, ownerID, gameID, "poisoner")
	require.NoError(t, err)
	require.Len(t, g.RolePromotions, 1)

	g, err = undoStarPass(t, handler, ownerID, gameID, "poisoner")
	require.NoError(t, err)
	assert.Empty(t, g.RolePromotions)

	// Deaths untouched by the undo.
	deaths := deathsByRole(g, night.Id)
	require.Contains(t, deaths, "imp")
	assert.Equal(t, clockkeeperv1.DeathCause_DEATH_CAUSE_DEMON, deaths["imp"].Cause)

	// Response equals a fresh GetGame.
	fresh := getGame(t, handler, ownerID, gameID)
	assert.True(t, proto.Equal(g, fresh), "undo response should equal fresh GetGame")
}

func TestUndoStarPass_NotPromoted(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := testHandler(t)
	ownerID, gameID := createTestGame(t, handler)
	setRoles(t, handler, ownerID, gameID, happyPathRoles)
	_, err := handler.StartGame(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.StartGameRequest{GameId: gameID}))
	require.NoError(t, err)

	// Nothing was promoted.
	_, err = undoStarPass(t, handler, ownerID, gameID, "poisoner")
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestUndoStarPass_BlocksOtherUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := testHandler(t)
	ownerID, gameID := createTestGame(t, handler)
	setRoles(t, handler, ownerID, gameID, happyPathRoles)
	_, err := handler.StartGame(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.StartGameRequest{GameId: gameID}))
	require.NoError(t, err)

	_, err = starPass(t, handler, ownerID, gameID, "poisoner")
	require.NoError(t, err)

	attacker, err := handler.db.User.Create().Save(authedCtx(ownerID))
	require.NoError(t, err)

	_, err = undoStarPass(t, handler, attacker.ID, gameID, "poisoner")
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// --- Chained promotions (Imp -> Poisoner -> Scarlet Woman) ---

func TestStarPass_ChainedPromotions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := testHandler(t)
	ownerID, gameID := createTestGame(t, handler)
	// Two minions (poisoner + scarletwoman) so the chain can pass the demon along.
	roles := []string{"imp", "poisoner", "scarletwoman", "washerwoman", "chef", "empath", "monk"}
	setRoles(t, handler, ownerID, gameID, roles)
	_, err := handler.StartGame(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.StartGameRequest{GameId: gameID}))
	require.NoError(t, err)

	before := getGame(t, handler, ownerID, gameID)
	night := nightPhase(t, before)

	// Imp self-kills; poisoner is promoted to act as imp.
	_, err = handler.RecordDeath(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: gameID, RoleId: "imp", PhaseId: &night.Id, Cause: clockkeeperv1.DeathCause_DEATH_CAUSE_DEMON,
	}))
	require.NoError(t, err)

	g, err := starPass(t, handler, ownerID, gameID, "poisoner")
	require.NoError(t, err)
	require.Len(t, g.RolePromotions, 1)
	assert.Equal(t, "poisoner", g.RolePromotions[0].RoleId)
	assert.Equal(t, "imp", g.RolePromotions[0].ActsAsRoleId)

	// Poisoner (acting as imp) dies; scarletwoman is promoted next.
	_, err = handler.RecordDeath(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: gameID, RoleId: "poisoner", PhaseId: &night.Id, Cause: clockkeeperv1.DeathCause_DEATH_CAUSE_EXECUTION,
	}))
	require.NoError(t, err)

	g, err = starPass(t, handler, ownerID, gameID, "scarletwoman")
	require.NoError(t, err)

	// Two ordered entries.
	require.Len(t, g.RolePromotions, 2)
	assert.Equal(t, "poisoner", g.RolePromotions[0].RoleId)
	assert.Equal(t, "imp", g.RolePromotions[0].ActsAsRoleId)
	assert.Equal(t, "scarletwoman", g.RolePromotions[1].RoleId)
	assert.Equal(t, "imp", g.RolePromotions[1].ActsAsRoleId)

	// Undo the first promotion only — the scarletwoman entry survives.
	g, err = undoStarPass(t, handler, ownerID, gameID, "poisoner")
	require.NoError(t, err)
	require.Len(t, g.RolePromotions, 1)
	assert.Equal(t, "scarletwoman", g.RolePromotions[0].RoleId)
	assert.Equal(t, "imp", g.RolePromotions[0].ActsAsRoleId)
}
