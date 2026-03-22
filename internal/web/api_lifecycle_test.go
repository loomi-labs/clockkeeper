package web

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startedGame creates a user, script, game with roles, and starts the game.
// Returns the owner username, the started game proto, and the handler.
func startedGame(t *testing.T, handler *ClockKeeperServiceHandler) (ownerUsername string, game *clockkeeperv1.Game) {
	t.Helper()
	ctx := context.Background()

	hash, err := HashPassword("pass")
	require.NoError(t, err)
	_, err = handler.db.User.Create().SetUsername("owner").SetPasswordHash(hash).Save(ctx)
	require.NoError(t, err)

	// Find a system script.
	scriptsResp, err := handler.ListScripts(authedCtx("owner"), connect.NewRequest(&clockkeeperv1.ListScriptsRequest{}))
	require.NoError(t, err)
	var scriptID int64
	for _, s := range scriptsResp.Msg.Scripts {
		if s.IsSystem {
			scriptID = s.Id
			break
		}
	}
	require.NotZero(t, scriptID)

	// Create a 5-player game.
	gameResp, err := handler.CreateGame(authedCtx("owner"), connect.NewRequest(&clockkeeperv1.CreateGameRequest{
		ScriptId:    scriptID,
		PlayerCount: 5,
	}))
	require.NoError(t, err)

	// Randomize roles so we have a valid set.
	_, err = handler.RandomizeRoles(authedCtx("owner"), connect.NewRequest(&clockkeeperv1.RandomizeRolesRequest{
		GameId: gameResp.Msg.Game.Id,
	}))
	require.NoError(t, err)

	// Start the game.
	startResp, err := handler.StartGame(authedCtx("owner"), connect.NewRequest(&clockkeeperv1.StartGameRequest{
		GameId: gameResp.Msg.Game.Id,
	}))
	require.NoError(t, err)

	return "owner", startResp.Msg.Game
}

// --- StartGame tests ---

func TestStartGame_Success(t *testing.T) {
	handler := testHandler(t)
	_, game := startedGame(t, handler)

	assert.Equal(t, clockkeeperv1.GameState_GAME_STATE_IN_PROGRESS, game.State)
	require.NotNil(t, game.PlayState)
	require.NotNil(t, game.PlayState.CurrentPhase)
	assert.Equal(t, int32(1), game.PlayState.CurrentRound)
	assert.Equal(t, clockkeeperv1.PhaseType_PHASE_TYPE_NIGHT, game.PlayState.CurrentPhase.Type)
	assert.True(t, game.PlayState.CurrentPhase.IsActive)
	assert.Len(t, game.PlayState.Phases, 1)
}

func TestStartGame_FailsNoRoles(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()

	hash, err := HashPassword("pass")
	require.NoError(t, err)
	_, err = handler.db.User.Create().SetUsername("testuser").SetPasswordHash(hash).Save(ctx)
	require.NoError(t, err)

	// Find a system script.
	scriptsResp, err := handler.ListScripts(authedCtx("testuser"), connect.NewRequest(&clockkeeperv1.ListScriptsRequest{}))
	require.NoError(t, err)
	var scriptID int64
	for _, s := range scriptsResp.Msg.Scripts {
		if s.IsSystem {
			scriptID = s.Id
			break
		}
	}
	require.NotZero(t, scriptID)

	// Create a game but don't assign roles.
	gameResp, err := handler.CreateGame(authedCtx("testuser"), connect.NewRequest(&clockkeeperv1.CreateGameRequest{
		ScriptId:    scriptID,
		PlayerCount: 5,
	}))
	require.NoError(t, err)

	// Try to start without roles.
	_, err = handler.StartGame(authedCtx("testuser"), connect.NewRequest(&clockkeeperv1.StartGameRequest{
		GameId: gameResp.Msg.Game.Id,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestStartGame_FailsNotSetup(t *testing.T) {
	handler := testHandler(t)
	ownerName, game := startedGame(t, handler)

	// Game is already in_progress, starting again should fail.
	_, err := handler.StartGame(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.StartGameRequest{
		GameId: game.Id,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestStartGame_BlocksOtherUser(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()

	hash, err := HashPassword("pass")
	require.NoError(t, err)
	_, err = handler.db.User.Create().SetUsername("userA").SetPasswordHash(hash).Save(ctx)
	require.NoError(t, err)
	_, err = handler.db.User.Create().SetUsername("userB").SetPasswordHash(hash).Save(ctx)
	require.NoError(t, err)

	// User A creates and sets up a game.
	scriptsResp, err := handler.ListScripts(authedCtx("userA"), connect.NewRequest(&clockkeeperv1.ListScriptsRequest{}))
	require.NoError(t, err)
	var scriptID int64
	for _, s := range scriptsResp.Msg.Scripts {
		if s.IsSystem {
			scriptID = s.Id
			break
		}
	}
	require.NotZero(t, scriptID)

	gameResp, err := handler.CreateGame(authedCtx("userA"), connect.NewRequest(&clockkeeperv1.CreateGameRequest{
		ScriptId:    scriptID,
		PlayerCount: 5,
	}))
	require.NoError(t, err)

	_, err = handler.RandomizeRoles(authedCtx("userA"), connect.NewRequest(&clockkeeperv1.RandomizeRolesRequest{
		GameId: gameResp.Msg.Game.Id,
	}))
	require.NoError(t, err)

	// User B tries to start it.
	_, err = handler.StartGame(authedCtx("userB"), connect.NewRequest(&clockkeeperv1.StartGameRequest{
		GameId: gameResp.Msg.Game.Id,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// --- AdvancePhase tests ---

func TestAdvancePhase_NightToDay(t *testing.T) {
	handler := testHandler(t)
	ownerName, game := startedGame(t, handler)

	// Game starts at Night 1 — advance to Day 1.
	resp, err := handler.AdvancePhase(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.AdvancePhaseRequest{
		GameId: game.Id,
	}))
	require.NoError(t, err)

	g := resp.Msg.Game
	require.NotNil(t, g.PlayState)
	require.NotNil(t, g.PlayState.CurrentPhase)
	assert.Equal(t, clockkeeperv1.PhaseType_PHASE_TYPE_DAY, g.PlayState.CurrentPhase.Type)
	assert.Equal(t, int32(1), g.PlayState.CurrentRound, "round should stay the same when going night->day")
	assert.True(t, g.PlayState.CurrentPhase.IsActive)
	assert.Len(t, g.PlayState.Phases, 2)
}

func TestAdvancePhase_DayToNight(t *testing.T) {
	handler := testHandler(t)
	ownerName, game := startedGame(t, handler)

	// Advance Night 1 -> Day 1.
	_, err := handler.AdvancePhase(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.AdvancePhaseRequest{
		GameId: game.Id,
	}))
	require.NoError(t, err)

	// Advance Day 1 -> Night 2.
	resp, err := handler.AdvancePhase(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.AdvancePhaseRequest{
		GameId: game.Id,
	}))
	require.NoError(t, err)

	g := resp.Msg.Game
	require.NotNil(t, g.PlayState)
	require.NotNil(t, g.PlayState.CurrentPhase)
	assert.Equal(t, clockkeeperv1.PhaseType_PHASE_TYPE_NIGHT, g.PlayState.CurrentPhase.Type)
	assert.Equal(t, int32(2), g.PlayState.CurrentRound, "round should increment when going day->night")
	assert.True(t, g.PlayState.CurrentPhase.IsActive)
	assert.Len(t, g.PlayState.Phases, 3)
}

func TestAdvancePhase_FailsNotInProgress(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()

	hash, err := HashPassword("pass")
	require.NoError(t, err)
	_, err = handler.db.User.Create().SetUsername("testuser").SetPasswordHash(hash).Save(ctx)
	require.NoError(t, err)

	// Create a game in setup state (no roles, not started).
	scriptsResp, err := handler.ListScripts(authedCtx("testuser"), connect.NewRequest(&clockkeeperv1.ListScriptsRequest{}))
	require.NoError(t, err)
	var scriptID int64
	for _, s := range scriptsResp.Msg.Scripts {
		if s.IsSystem {
			scriptID = s.Id
			break
		}
	}
	require.NotZero(t, scriptID)

	gameResp, err := handler.CreateGame(authedCtx("testuser"), connect.NewRequest(&clockkeeperv1.CreateGameRequest{
		ScriptId:    scriptID,
		PlayerCount: 5,
	}))
	require.NoError(t, err)

	_, err = handler.AdvancePhase(authedCtx("testuser"), connect.NewRequest(&clockkeeperv1.AdvancePhaseRequest{
		GameId: gameResp.Msg.Game.Id,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestAdvancePhase_BlocksOtherUser(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()
	_, game := startedGame(t, handler)

	hash, err := HashPassword("pass")
	require.NoError(t, err)
	_, err = handler.db.User.Create().SetUsername("attacker").SetPasswordHash(hash).Save(ctx)
	require.NoError(t, err)

	_, err = handler.AdvancePhase(authedCtx("attacker"), connect.NewRequest(&clockkeeperv1.AdvancePhaseRequest{
		GameId: game.Id,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// --- EndGame tests ---

func TestEndGame_Success(t *testing.T) {
	handler := testHandler(t)
	ownerName, game := startedGame(t, handler)

	resp, err := handler.EndGame(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.EndGameRequest{
		GameId: game.Id,
	}))
	require.NoError(t, err)

	g := resp.Msg.Game
	assert.Equal(t, clockkeeperv1.GameState_GAME_STATE_COMPLETED, g.State)
	// All phases should be deactivated.
	require.NotNil(t, g.PlayState)
	for _, p := range g.PlayState.Phases {
		assert.False(t, p.IsActive, "all phases should be deactivated after ending game")
	}
	assert.Nil(t, g.PlayState.CurrentPhase)
}

func TestEndGame_FailsNotInProgress(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()

	hash, err := HashPassword("pass")
	require.NoError(t, err)
	_, err = handler.db.User.Create().SetUsername("testuser").SetPasswordHash(hash).Save(ctx)
	require.NoError(t, err)

	scriptsResp, err := handler.ListScripts(authedCtx("testuser"), connect.NewRequest(&clockkeeperv1.ListScriptsRequest{}))
	require.NoError(t, err)
	var scriptID int64
	for _, s := range scriptsResp.Msg.Scripts {
		if s.IsSystem {
			scriptID = s.Id
			break
		}
	}
	require.NotZero(t, scriptID)

	gameResp, err := handler.CreateGame(authedCtx("testuser"), connect.NewRequest(&clockkeeperv1.CreateGameRequest{
		ScriptId:    scriptID,
		PlayerCount: 5,
	}))
	require.NoError(t, err)

	// Game is in setup state, ending should fail.
	_, err = handler.EndGame(authedCtx("testuser"), connect.NewRequest(&clockkeeperv1.EndGameRequest{
		GameId: gameResp.Msg.Game.Id,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestEndGame_BlocksOtherUser(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()
	_, game := startedGame(t, handler)

	hash, err := HashPassword("pass")
	require.NoError(t, err)
	_, err = handler.db.User.Create().SetUsername("attacker").SetPasswordHash(hash).Save(ctx)
	require.NoError(t, err)

	_, err = handler.EndGame(authedCtx("attacker"), connect.NewRequest(&clockkeeperv1.EndGameRequest{
		GameId: game.Id,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// --- RecordDeath tests ---

func TestRecordDeath_Success(t *testing.T) {
	handler := testHandler(t)
	ownerName, game := startedGame(t, handler)

	// Use the first selected role for the death.
	require.NotEmpty(t, game.SelectedRoleIds)
	roleID := game.SelectedRoleIds[0]

	resp, err := handler.RecordDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: game.Id,
		RoleId: roleID,
	}))
	require.NoError(t, err)

	g := resp.Msg.Game
	require.NotNil(t, g.PlayState)
	require.Len(t, g.PlayState.AllDeaths, 1)
	assert.Equal(t, roleID, g.PlayState.AllDeaths[0].RoleId)
	assert.True(t, g.PlayState.AllDeaths[0].GhostVote, "ghost vote should default to true")
}

func TestRecordDeath_FailsUnknownRole(t *testing.T) {
	handler := testHandler(t)
	ownerName, game := startedGame(t, handler)

	_, err := handler.RecordDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: game.Id,
		RoleId: "nonexistent_character_xyz",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestRecordDeath_FailsRoleNotInGame(t *testing.T) {
	handler := testHandler(t)
	ownerName, game := startedGame(t, handler)

	// "grandmother" is a valid character but unlikely to be in a 5-player randomized game.
	// We need to find a character that exists in the registry but is not in the game.
	// Use a role from a different edition/team that won't be randomly selected.
	// Let's just pick a role NOT in the selected roles.
	inGame := make(map[string]bool)
	for _, id := range game.SelectedRoleIds {
		inGame[id] = true
	}

	// Find a character not in the game by listing all characters.
	charsResp, err := handler.ListCharacters(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.ListCharactersRequest{}))
	require.NoError(t, err)

	var notInGameRoleID string
	for _, c := range charsResp.Msg.Characters {
		if !inGame[c.Id] && c.Team != clockkeeperv1.Team_TEAM_TRAVELLER {
			notInGameRoleID = c.Id
			break
		}
	}
	require.NotEmpty(t, notInGameRoleID, "expected to find a character not in the game")

	_, err = handler.RecordDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: game.Id,
		RoleId: notInGameRoleID,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestRecordDeath_IdempotentSamePhase(t *testing.T) {
	handler := testHandler(t)
	ownerName, game := startedGame(t, handler)

	require.NotEmpty(t, game.SelectedRoleIds)
	roleID := game.SelectedRoleIds[0]

	// Record first death.
	_, err := handler.RecordDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: game.Id,
		RoleId: roleID,
	}))
	require.NoError(t, err)

	// Recording same role in same phase should succeed silently (idempotent).
	resp, err := handler.RecordDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: game.Id,
		RoleId: roleID,
	}))
	require.NoError(t, err)
	assert.Len(t, resp.Msg.Game.PlayState.AllDeaths, 1, "should still have exactly 1 death")
}

func TestRecordDeath_BlocksOtherUser(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()
	_, game := startedGame(t, handler)

	hash, err := HashPassword("pass")
	require.NoError(t, err)
	_, err = handler.db.User.Create().SetUsername("attacker").SetPasswordHash(hash).Save(ctx)
	require.NoError(t, err)

	require.NotEmpty(t, game.SelectedRoleIds)
	_, err = handler.RecordDeath(authedCtx("attacker"), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: game.Id,
		RoleId: game.SelectedRoleIds[0],
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// --- RemoveDeath tests ---

func TestRemoveDeath_Success(t *testing.T) {
	handler := testHandler(t)
	ownerName, game := startedGame(t, handler)

	require.NotEmpty(t, game.SelectedRoleIds)
	roleID := game.SelectedRoleIds[0]

	// Record a death.
	deathResp, err := handler.RecordDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: game.Id,
		RoleId: roleID,
	}))
	require.NoError(t, err)
	require.Len(t, deathResp.Msg.Game.PlayState.AllDeaths, 1)
	deathID := deathResp.Msg.Game.PlayState.AllDeaths[0].Id

	// Remove the death.
	removeResp, err := handler.RemoveDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RemoveDeathRequest{
		GameId:  game.Id,
		DeathId: deathID,
	}))
	require.NoError(t, err)
	assert.Empty(t, removeResp.Msg.Game.PlayState.AllDeaths)
}

func TestRemoveDeath_FailsWrongGame(t *testing.T) {
	handler := testHandler(t)

	// Create the first game with the startedGame helper (user "owner").
	ownerName, game1 := startedGame(t, handler)

	require.NotEmpty(t, game1.SelectedRoleIds)

	// Record a death in game 1.
	deathResp, err := handler.RecordDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: game1.Id,
		RoleId: game1.SelectedRoleIds[0],
	}))
	require.NoError(t, err)
	require.Len(t, deathResp.Msg.Game.PlayState.AllDeaths, 1)
	deathID := deathResp.Msg.Game.PlayState.AllDeaths[0].Id

	// Create a second game for the same owner.
	scriptsResp, err := handler.ListScripts(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.ListScriptsRequest{}))
	require.NoError(t, err)
	var scriptID int64
	for _, s := range scriptsResp.Msg.Scripts {
		if s.IsSystem {
			scriptID = s.Id
			break
		}
	}
	require.NotZero(t, scriptID)

	game2Resp, err := handler.CreateGame(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.CreateGameRequest{
		ScriptId:    scriptID,
		PlayerCount: 5,
	}))
	require.NoError(t, err)

	_, err = handler.RandomizeRoles(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RandomizeRolesRequest{
		GameId: game2Resp.Msg.Game.Id,
	}))
	require.NoError(t, err)

	startResp, err := handler.StartGame(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.StartGameRequest{
		GameId: game2Resp.Msg.Game.Id,
	}))
	require.NoError(t, err)
	game2 := startResp.Msg.Game

	// Try to remove death from game1 using game2's ID.
	_, err = handler.RemoveDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RemoveDeathRequest{
		GameId:  game2.Id,
		DeathId: deathID,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// --- UseGhostVote tests ---

func TestUseGhostVote_Success(t *testing.T) {
	handler := testHandler(t)
	ownerName, game := startedGame(t, handler)

	require.NotEmpty(t, game.SelectedRoleIds)
	roleID := game.SelectedRoleIds[0]

	// Record a death (ghost_vote defaults to true).
	deathResp, err := handler.RecordDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: game.Id,
		RoleId: roleID,
	}))
	require.NoError(t, err)
	require.Len(t, deathResp.Msg.Game.PlayState.AllDeaths, 1)
	d := deathResp.Msg.Game.PlayState.AllDeaths[0]
	assert.True(t, d.GhostVote)

	// Use the ghost vote.
	resp, err := handler.UseGhostVote(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.UseGhostVoteRequest{
		GameId:  game.Id,
		DeathId: d.Id,
	}))
	require.NoError(t, err)

	// Find the death in the response and verify ghost_vote is now false.
	require.Len(t, resp.Msg.Game.PlayState.AllDeaths, 1)
	assert.False(t, resp.Msg.Game.PlayState.AllDeaths[0].GhostVote)
}

func TestUseGhostVote_FailsAlreadyUsed(t *testing.T) {
	handler := testHandler(t)
	ownerName, game := startedGame(t, handler)

	require.NotEmpty(t, game.SelectedRoleIds)
	roleID := game.SelectedRoleIds[0]

	// Record a death.
	deathResp, err := handler.RecordDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: game.Id,
		RoleId: roleID,
	}))
	require.NoError(t, err)
	deathID := deathResp.Msg.Game.PlayState.AllDeaths[0].Id

	// Use the ghost vote.
	_, err = handler.UseGhostVote(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.UseGhostVoteRequest{
		GameId:  game.Id,
		DeathId: deathID,
	}))
	require.NoError(t, err)

	// Try to use it again.
	_, err = handler.UseGhostVote(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.UseGhostVoteRequest{
		GameId:  game.Id,
		DeathId: deathID,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

// --- Per-phase death propagation tests ---

func findPhase(g *clockkeeperv1.Game, phaseType clockkeeperv1.PhaseType, round int32) *clockkeeperv1.Phase {
	for _, p := range g.PlayState.Phases {
		if p.Type == phaseType && p.RoundNumber == round {
			return p
		}
	}
	return nil
}

func phaseHasDeathForRole(p *clockkeeperv1.Phase, roleID string) bool {
	for _, d := range p.Deaths {
		if d.RoleId == roleID {
			return true
		}
	}
	return false
}

func TestRecordDeath_Propagate(t *testing.T) {
	handler := testHandler(t)
	ownerName, game := startedGame(t, handler)

	require.NotEmpty(t, game.SelectedRoleIds)
	roleID := game.SelectedRoleIds[0]

	// Advance to Day 1 first so there are 2 phases.
	advResp, err := handler.AdvancePhase(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.AdvancePhaseRequest{
		GameId: game.Id,
	}))
	require.NoError(t, err)
	game = advResp.Msg.Game

	n1 := findPhase(game, clockkeeperv1.PhaseType_PHASE_TYPE_NIGHT, 1)
	require.NotNil(t, n1)

	// Record death on Night 1 with propagation.
	resp, err := handler.RecordDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId:    game.Id,
		RoleId:    roleID,
		PhaseId:   &n1.Id,
		Propagate: true,
	}))
	require.NoError(t, err)

	g := resp.Msg.Game
	n1After := findPhase(g, clockkeeperv1.PhaseType_PHASE_TYPE_NIGHT, 1)
	require.NotNil(t, n1After, "expected phase type=NIGHT round=1")
	d1After := findPhase(g, clockkeeperv1.PhaseType_PHASE_TYPE_DAY, 1)
	require.NotNil(t, d1After, "expected phase type=DAY round=1")

	assert.True(t, phaseHasDeathForRole(n1After, roleID), "should be dead in Night 1")
	assert.True(t, phaseHasDeathForRole(d1After, roleID), "should be dead in Day 1 (propagated)")
}

func TestRecordDeath_NoPropagateOnlyTargetPhase(t *testing.T) {
	handler := testHandler(t)
	ownerName, game := startedGame(t, handler)

	require.NotEmpty(t, game.SelectedRoleIds)
	roleID := game.SelectedRoleIds[0]

	// Advance to Day 1.
	advResp, err := handler.AdvancePhase(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.AdvancePhaseRequest{
		GameId: game.Id,
	}))
	require.NoError(t, err)
	game = advResp.Msg.Game

	n1 := findPhase(game, clockkeeperv1.PhaseType_PHASE_TYPE_NIGHT, 1)
	require.NotNil(t, n1)

	// Record death on Night 1 WITHOUT propagation.
	resp, err := handler.RecordDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId:    game.Id,
		RoleId:    roleID,
		PhaseId:   &n1.Id,
		Propagate: false,
	}))
	require.NoError(t, err)

	g := resp.Msg.Game
	n1After := findPhase(g, clockkeeperv1.PhaseType_PHASE_TYPE_NIGHT, 1)
	require.NotNil(t, n1After, "expected phase type=NIGHT round=1")
	d1After := findPhase(g, clockkeeperv1.PhaseType_PHASE_TYPE_DAY, 1)
	require.NotNil(t, d1After, "expected phase type=DAY round=1")

	assert.True(t, phaseHasDeathForRole(n1After, roleID), "should be dead in Night 1")
	assert.False(t, phaseHasDeathForRole(d1After, roleID), "should NOT be dead in Day 1")
}

func TestRecordDeath_PropagateToAllLaterPhases(t *testing.T) {
	handler := testHandler(t)
	ownerName, game := startedGame(t, handler)

	require.NotEmpty(t, game.SelectedRoleIds)
	roleID := game.SelectedRoleIds[0]

	// Advance to Day 1, then Night 2 (3 phases total).
	_, err := handler.AdvancePhase(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.AdvancePhaseRequest{GameId: game.Id}))
	require.NoError(t, err)
	advResp, err := handler.AdvancePhase(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.AdvancePhaseRequest{GameId: game.Id}))
	require.NoError(t, err)
	game = advResp.Msg.Game
	require.Len(t, game.PlayState.Phases, 3)

	n1 := findPhase(game, clockkeeperv1.PhaseType_PHASE_TYPE_NIGHT, 1)
	require.NotNil(t, n1)

	// Record death on Night 1 with propagation — should hit all 3 phases.
	resp, err := handler.RecordDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId:    game.Id,
		RoleId:    roleID,
		PhaseId:   &n1.Id,
		Propagate: true,
	}))
	require.NoError(t, err)

	g := resp.Msg.Game
	pN1 := findPhase(g, clockkeeperv1.PhaseType_PHASE_TYPE_NIGHT, 1)
	require.NotNil(t, pN1, "expected phase type=NIGHT round=1")
	pD1 := findPhase(g, clockkeeperv1.PhaseType_PHASE_TYPE_DAY, 1)
	require.NotNil(t, pD1, "expected phase type=DAY round=1")
	pN2 := findPhase(g, clockkeeperv1.PhaseType_PHASE_TYPE_NIGHT, 2)
	require.NotNil(t, pN2, "expected phase type=NIGHT round=2")
	assert.True(t, phaseHasDeathForRole(pN1, roleID))
	assert.True(t, phaseHasDeathForRole(pD1, roleID))
	assert.True(t, phaseHasDeathForRole(pN2, roleID))
}

func TestRemoveDeath_Propagate(t *testing.T) {
	handler := testHandler(t)
	ownerName, game := startedGame(t, handler)

	require.NotEmpty(t, game.SelectedRoleIds)
	roleID := game.SelectedRoleIds[0]

	// Advance to Day 1, then Night 2.
	_, err := handler.AdvancePhase(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.AdvancePhaseRequest{GameId: game.Id}))
	require.NoError(t, err)
	advResp, err := handler.AdvancePhase(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.AdvancePhaseRequest{GameId: game.Id}))
	require.NoError(t, err)
	game = advResp.Msg.Game

	n1 := findPhase(game, clockkeeperv1.PhaseType_PHASE_TYPE_NIGHT, 1)
	require.NotNil(t, n1, "expected phase type=NIGHT round=1")

	// Record death across all phases.
	deathResp, err := handler.RecordDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: game.Id, RoleId: roleID, PhaseId: &n1.Id, Propagate: true,
	}))
	require.NoError(t, err)

	// Find the death in Day 1 and remove with propagation.
	g := deathResp.Msg.Game
	d1 := findPhase(g, clockkeeperv1.PhaseType_PHASE_TYPE_DAY, 1)
	require.NotNil(t, d1, "expected phase type=DAY round=1")
	var d1DeathID int64
	for _, d := range d1.Deaths {
		if d.RoleId == roleID {
			d1DeathID = d.Id
			break
		}
	}
	require.NotZero(t, d1DeathID)

	removeResp, err := handler.RemoveDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RemoveDeathRequest{
		GameId: game.Id, DeathId: d1DeathID, Propagate: true,
	}))
	require.NoError(t, err)

	g = removeResp.Msg.Game
	rN1 := findPhase(g, clockkeeperv1.PhaseType_PHASE_TYPE_NIGHT, 1)
	require.NotNil(t, rN1, "expected phase type=NIGHT round=1")
	rD1 := findPhase(g, clockkeeperv1.PhaseType_PHASE_TYPE_DAY, 1)
	require.NotNil(t, rD1, "expected phase type=DAY round=1")
	rN2 := findPhase(g, clockkeeperv1.PhaseType_PHASE_TYPE_NIGHT, 2)
	require.NotNil(t, rN2, "expected phase type=NIGHT round=2")
	assert.True(t, phaseHasDeathForRole(rN1, roleID), "N1 death should remain")
	assert.False(t, phaseHasDeathForRole(rD1, roleID), "D1 death should be removed")
	assert.False(t, phaseHasDeathForRole(rN2, roleID), "N2 death should be removed")
}

func TestRemoveDeath_NoPropagateOnlyOne(t *testing.T) {
	handler := testHandler(t)
	ownerName, game := startedGame(t, handler)

	require.NotEmpty(t, game.SelectedRoleIds)
	roleID := game.SelectedRoleIds[0]

	// Advance to Day 1.
	advResp, err := handler.AdvancePhase(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.AdvancePhaseRequest{GameId: game.Id}))
	require.NoError(t, err)
	game = advResp.Msg.Game

	n1 := findPhase(game, clockkeeperv1.PhaseType_PHASE_TYPE_NIGHT, 1)
	require.NotNil(t, n1, "expected phase type=NIGHT round=1")

	// Record death in both phases.
	deathResp, err := handler.RecordDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: game.Id, RoleId: roleID, PhaseId: &n1.Id, Propagate: true,
	}))
	require.NoError(t, err)

	// Remove only the Night 1 death (no propagation).
	g := deathResp.Msg.Game
	n1After := findPhase(g, clockkeeperv1.PhaseType_PHASE_TYPE_NIGHT, 1)
	require.NotNil(t, n1After, "expected phase type=NIGHT round=1")
	var n1DeathID int64
	for _, d := range n1After.Deaths {
		if d.RoleId == roleID {
			n1DeathID = d.Id
			break
		}
	}
	require.NotZero(t, n1DeathID)

	removeResp, err := handler.RemoveDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RemoveDeathRequest{
		GameId: game.Id, DeathId: n1DeathID, Propagate: false,
	}))
	require.NoError(t, err)

	g = removeResp.Msg.Game
	finalN1 := findPhase(g, clockkeeperv1.PhaseType_PHASE_TYPE_NIGHT, 1)
	require.NotNil(t, finalN1, "expected phase type=NIGHT round=1")
	finalD1 := findPhase(g, clockkeeperv1.PhaseType_PHASE_TYPE_DAY, 1)
	require.NotNil(t, finalD1, "expected phase type=DAY round=1")
	assert.False(t, phaseHasDeathForRole(finalN1, roleID), "N1 should be removed")
	assert.True(t, phaseHasDeathForRole(finalD1, roleID), "D1 should remain")
}

func TestAdvancePhase_CopiesDeaths(t *testing.T) {
	handler := testHandler(t)
	ownerName, game := startedGame(t, handler)

	require.NotEmpty(t, game.SelectedRoleIds)
	roleID := game.SelectedRoleIds[0]

	// Record death in Night 1.
	_, err := handler.RecordDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: game.Id, RoleId: roleID,
	}))
	require.NoError(t, err)

	// Advance to Day 1 — should auto-copy the death.
	advResp, err := handler.AdvancePhase(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.AdvancePhaseRequest{GameId: game.Id}))
	require.NoError(t, err)

	g := advResp.Msg.Game
	d1 := findPhase(g, clockkeeperv1.PhaseType_PHASE_TYPE_DAY, 1)
	require.NotNil(t, d1)
	assert.True(t, phaseHasDeathForRole(d1, roleID), "death should be copied to Day 1")

	// Verify ghost_vote is preserved.
	for _, d := range d1.Deaths {
		if d.RoleId == roleID {
			assert.True(t, d.GhostVote, "ghost vote should be preserved when copying")
		}
	}
}

func TestAdvancePhase_CopiesMultipleDeaths(t *testing.T) {
	handler := testHandler(t)
	ownerName, game := startedGame(t, handler)

	require.GreaterOrEqual(t, len(game.SelectedRoleIds), 2)
	role1 := game.SelectedRoleIds[0]
	role2 := game.SelectedRoleIds[1]

	// Record two deaths in Night 1.
	_, err := handler.RecordDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{GameId: game.Id, RoleId: role1}))
	require.NoError(t, err)
	_, err = handler.RecordDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{GameId: game.Id, RoleId: role2}))
	require.NoError(t, err)

	// Advance.
	advResp, err := handler.AdvancePhase(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.AdvancePhaseRequest{GameId: game.Id}))
	require.NoError(t, err)

	d1 := findPhase(advResp.Msg.Game, clockkeeperv1.PhaseType_PHASE_TYPE_DAY, 1)
	require.NotNil(t, d1)
	assert.True(t, phaseHasDeathForRole(d1, role1), "role1 death should be copied")
	assert.True(t, phaseHasDeathForRole(d1, role2), "role2 death should be copied")
}

func TestUseGhostVote_SyncsAcrossPhases(t *testing.T) {
	handler := testHandler(t)
	ownerName, game := startedGame(t, handler)

	require.NotEmpty(t, game.SelectedRoleIds)
	roleID := game.SelectedRoleIds[0]

	// Record death in Night 1.
	deathResp, err := handler.RecordDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: game.Id, RoleId: roleID,
	}))
	require.NoError(t, err)

	// Advance to Day 1 (death copied).
	_, err = handler.AdvancePhase(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.AdvancePhaseRequest{GameId: game.Id}))
	require.NoError(t, err)

	// Use ghost vote on the Night 1 death record.
	n1DeathID := deathResp.Msg.Game.PlayState.AllDeaths[0].Id
	voteResp, err := handler.UseGhostVote(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.UseGhostVoteRequest{
		GameId: game.Id, DeathId: n1DeathID,
	}))
	require.NoError(t, err)

	// Both phase death records should have ghost_vote=false.
	g := voteResp.Msg.Game
	for _, d := range g.PlayState.AllDeaths {
		if d.RoleId == roleID {
			assert.False(t, d.GhostVote, "ghost vote should be false in phase %d", d.PhaseId)
		}
	}
}

func TestRecordDeath_ResurrectionFlow(t *testing.T) {
	handler := testHandler(t)
	ownerName, game := startedGame(t, handler)

	require.NotEmpty(t, game.SelectedRoleIds)
	roleID := game.SelectedRoleIds[0]

	// Record death in Night 1 (auto-propagates to current phase only since it's the only one).
	_, err := handler.RecordDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: game.Id, RoleId: roleID,
	}))
	require.NoError(t, err)

	// Advance to Day 1 (death copied), then Night 2 (death copied again).
	_, err = handler.AdvancePhase(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.AdvancePhaseRequest{GameId: game.Id}))
	require.NoError(t, err)
	advResp, err := handler.AdvancePhase(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.AdvancePhaseRequest{GameId: game.Id}))
	require.NoError(t, err)
	game = advResp.Msg.Game

	// Dead in all 3 phases.
	require.Len(t, game.PlayState.Phases, 3)
	for _, p := range game.PlayState.Phases {
		assert.True(t, phaseHasDeathForRole(p, roleID), "should be dead in %v %d", p.Type, p.RoundNumber)
	}

	// Resurrect in Night 2 only (no propagation) — simulate "this phase only" undo.
	n2 := findPhase(game, clockkeeperv1.PhaseType_PHASE_TYPE_NIGHT, 2)
	require.NotNil(t, n2, "expected phase type=NIGHT round=2")
	var n2DeathID int64
	for _, d := range n2.Deaths {
		if d.RoleId == roleID {
			n2DeathID = d.Id
			break
		}
	}
	require.NotZero(t, n2DeathID)

	removeResp, err := handler.RemoveDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RemoveDeathRequest{
		GameId: game.Id, DeathId: n2DeathID, Propagate: false,
	}))
	require.NoError(t, err)
	game = removeResp.Msg.Game

	// Dead in N1 and D1, alive in N2.
	resN1 := findPhase(game, clockkeeperv1.PhaseType_PHASE_TYPE_NIGHT, 1)
	require.NotNil(t, resN1, "expected phase type=NIGHT round=1")
	resD1 := findPhase(game, clockkeeperv1.PhaseType_PHASE_TYPE_DAY, 1)
	require.NotNil(t, resD1, "expected phase type=DAY round=1")
	resN2 := findPhase(game, clockkeeperv1.PhaseType_PHASE_TYPE_NIGHT, 2)
	require.NotNil(t, resN2, "expected phase type=NIGHT round=2")
	assert.True(t, phaseHasDeathForRole(resN1, roleID), "dead in N1")
	assert.True(t, phaseHasDeathForRole(resD1, roleID), "dead in D1")
	assert.False(t, phaseHasDeathForRole(resN2, roleID), "alive in N2 (resurrected)")

	// Can die again in Night 2.
	resp, err := handler.RecordDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: game.Id, RoleId: roleID,
	}))
	require.NoError(t, err)
	redeathN2 := findPhase(resp.Msg.Game, clockkeeperv1.PhaseType_PHASE_TYPE_NIGHT, 2)
	require.NotNil(t, redeathN2, "expected phase type=NIGHT round=2")
	assert.True(t, phaseHasDeathForRole(redeathN2, roleID), "dead again in N2")
}

// --- GetGame play state test ---

func TestGetGame_IncludesPlayState(t *testing.T) {
	handler := testHandler(t)
	ownerName, game := startedGame(t, handler)

	// Record a death so there's play state data.
	require.NotEmpty(t, game.SelectedRoleIds)
	roleID := game.SelectedRoleIds[0]
	_, err := handler.RecordDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: game.Id,
		RoleId: roleID,
	}))
	require.NoError(t, err)

	// Advance to day.
	_, err = handler.AdvancePhase(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.AdvancePhaseRequest{
		GameId: game.Id,
	}))
	require.NoError(t, err)

	// Fetch the game via GetGame.
	resp, err := handler.GetGame(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.GetGameRequest{
		Id: game.Id,
	}))
	require.NoError(t, err)

	g := resp.Msg.Game
	assert.Equal(t, clockkeeperv1.GameState_GAME_STATE_IN_PROGRESS, g.State)
	require.NotNil(t, g.PlayState)
	assert.Equal(t, int32(1), g.PlayState.CurrentRound)
	assert.Equal(t, clockkeeperv1.PhaseType_PHASE_TYPE_DAY, g.PlayState.CurrentPhase.Type)
	assert.Len(t, g.PlayState.Phases, 2, "should have night 1 and day 1")
	assert.Len(t, g.PlayState.AllDeaths, 2, "death should be in both phases (propagated via AdvancePhase)")
	assert.Equal(t, roleID, g.PlayState.AllDeaths[0].RoleId)
}

// --- ListGames tests ---

func TestListGames_ReturnsOwnedGames(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()

	hash, err := HashPassword("pass")
	require.NoError(t, err)

	// Create two users.
	_, err = handler.db.User.Create().SetUsername("user1").SetPasswordHash(hash).Save(ctx)
	require.NoError(t, err)
	_, err = handler.db.User.Create().SetUsername("user2").SetPasswordHash(hash).Save(ctx)
	require.NoError(t, err)

	// Find a system script.
	scriptsResp, err := handler.ListScripts(authedCtx("user1"), connect.NewRequest(&clockkeeperv1.ListScriptsRequest{}))
	require.NoError(t, err)
	var scriptID int64
	for _, s := range scriptsResp.Msg.Scripts {
		if s.IsSystem {
			scriptID = s.Id
			break
		}
	}
	require.NotZero(t, scriptID)

	// User1 creates 2 games.
	_, err = handler.CreateGame(authedCtx("user1"), connect.NewRequest(&clockkeeperv1.CreateGameRequest{
		ScriptId: scriptID, PlayerCount: 5,
	}))
	require.NoError(t, err)
	_, err = handler.CreateGame(authedCtx("user1"), connect.NewRequest(&clockkeeperv1.CreateGameRequest{
		ScriptId: scriptID, PlayerCount: 7,
	}))
	require.NoError(t, err)

	// User2 creates 1 game.
	_, err = handler.CreateGame(authedCtx("user2"), connect.NewRequest(&clockkeeperv1.CreateGameRequest{
		ScriptId: scriptID, PlayerCount: 6,
	}))
	require.NoError(t, err)

	// User1 should see 2 games.
	resp, err := handler.ListGames(authedCtx("user1"), connect.NewRequest(&clockkeeperv1.ListGamesRequest{}))
	require.NoError(t, err)
	assert.Len(t, resp.Msg.Games, 2)

	// User2 should see 1 game.
	resp, err = handler.ListGames(authedCtx("user2"), connect.NewRequest(&clockkeeperv1.ListGamesRequest{}))
	require.NoError(t, err)
	assert.Len(t, resp.Msg.Games, 1)
	assert.Equal(t, int32(6), resp.Msg.Games[0].PlayerCount)
}

// --- UpdateTravellerAlignment tests ---

func TestUpdateTravellerAlignment_Success(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()

	hash, err := HashPassword("pass")
	require.NoError(t, err)
	_, err = handler.db.User.Create().SetUsername("owner").SetPasswordHash(hash).Save(ctx)
	require.NoError(t, err)

	// Find a system script.
	scriptsResp, err := handler.ListScripts(authedCtx("owner"), connect.NewRequest(&clockkeeperv1.ListScriptsRequest{}))
	require.NoError(t, err)
	var scriptID int64
	for _, s := range scriptsResp.Msg.Scripts {
		if s.IsSystem {
			scriptID = s.Id
			break
		}
	}
	require.NotZero(t, scriptID)

	// Create a game.
	gameResp, err := handler.CreateGame(authedCtx("owner"), connect.NewRequest(&clockkeeperv1.CreateGameRequest{
		ScriptId:    scriptID,
		PlayerCount: 7,
	}))
	require.NoError(t, err)
	gameID := gameResp.Msg.Game.Id

	// Find a valid traveller via ListCharacters.
	charsResp, err := handler.ListCharacters(authedCtx("owner"), connect.NewRequest(&clockkeeperv1.ListCharactersRequest{
		Team: clockkeeperv1.Team_TEAM_TRAVELLER,
	}))
	require.NoError(t, err)
	require.NotEmpty(t, charsResp.Msg.Characters, "expected at least one traveller character")
	travellerID := charsResp.Msg.Characters[0].Id

	// Add traveller to the game.
	_, err = handler.UpdateGameTravellers(authedCtx("owner"), connect.NewRequest(&clockkeeperv1.UpdateGameTravellersRequest{
		GameId:               gameID,
		SelectedTravellerIds: []string{travellerID},
	}))
	require.NoError(t, err)

	// Set alignment to GOOD.
	resp, err := handler.UpdateTravellerAlignment(authedCtx("owner"), connect.NewRequest(&clockkeeperv1.UpdateTravellerAlignmentRequest{
		GameId:    gameID,
		RoleId:    travellerID,
		Alignment: clockkeeperv1.TravellerAlignment_TRAVELLER_ALIGNMENT_GOOD,
	}))
	require.NoError(t, err)
	assert.Equal(t, clockkeeperv1.TravellerAlignment_TRAVELLER_ALIGNMENT_GOOD, resp.Msg.Game.TravellerAlignments[travellerID])

	// Set alignment to EVIL.
	resp, err = handler.UpdateTravellerAlignment(authedCtx("owner"), connect.NewRequest(&clockkeeperv1.UpdateTravellerAlignmentRequest{
		GameId:    gameID,
		RoleId:    travellerID,
		Alignment: clockkeeperv1.TravellerAlignment_TRAVELLER_ALIGNMENT_EVIL,
	}))
	require.NoError(t, err)
	assert.Equal(t, clockkeeperv1.TravellerAlignment_TRAVELLER_ALIGNMENT_EVIL, resp.Msg.Game.TravellerAlignments[travellerID])

	// Set alignment to UNSPECIFIED — should clear it.
	resp, err = handler.UpdateTravellerAlignment(authedCtx("owner"), connect.NewRequest(&clockkeeperv1.UpdateTravellerAlignmentRequest{
		GameId:    gameID,
		RoleId:    travellerID,
		Alignment: clockkeeperv1.TravellerAlignment_TRAVELLER_ALIGNMENT_UNSPECIFIED,
	}))
	require.NoError(t, err)
	_, exists := resp.Msg.Game.TravellerAlignments[travellerID]
	assert.False(t, exists, "UNSPECIFIED alignment should remove the entry")
}

func TestUpdateTravellerAlignment_FailsNotTraveller(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()

	hash, err := HashPassword("pass")
	require.NoError(t, err)
	_, err = handler.db.User.Create().SetUsername("owner").SetPasswordHash(hash).Save(ctx)
	require.NoError(t, err)

	// Find a system script.
	scriptsResp, err := handler.ListScripts(authedCtx("owner"), connect.NewRequest(&clockkeeperv1.ListScriptsRequest{}))
	require.NoError(t, err)
	var scriptID int64
	for _, s := range scriptsResp.Msg.Scripts {
		if s.IsSystem {
			scriptID = s.Id
			break
		}
	}
	require.NotZero(t, scriptID)

	// Create a game.
	gameResp, err := handler.CreateGame(authedCtx("owner"), connect.NewRequest(&clockkeeperv1.CreateGameRequest{
		ScriptId:    scriptID,
		PlayerCount: 7,
	}))
	require.NoError(t, err)

	// Try to set alignment on a non-traveller role (not in selected_travellers).
	_, err = handler.UpdateTravellerAlignment(authedCtx("owner"), connect.NewRequest(&clockkeeperv1.UpdateTravellerAlignmentRequest{
		GameId:    gameResp.Msg.Game.Id,
		RoleId:    "washerwoman",
		Alignment: clockkeeperv1.TravellerAlignment_TRAVELLER_ALIGNMENT_GOOD,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestUpdateTravellerAlignment_BlocksOtherUser(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()

	hash, err := HashPassword("pass")
	require.NoError(t, err)
	_, err = handler.db.User.Create().SetUsername("userA").SetPasswordHash(hash).Save(ctx)
	require.NoError(t, err)
	_, err = handler.db.User.Create().SetUsername("userB").SetPasswordHash(hash).Save(ctx)
	require.NoError(t, err)

	// Find a system script.
	scriptsResp, err := handler.ListScripts(authedCtx("userA"), connect.NewRequest(&clockkeeperv1.ListScriptsRequest{}))
	require.NoError(t, err)
	var scriptID int64
	for _, s := range scriptsResp.Msg.Scripts {
		if s.IsSystem {
			scriptID = s.Id
			break
		}
	}
	require.NotZero(t, scriptID)

	// User A creates a game with a traveller.
	gameResp, err := handler.CreateGame(authedCtx("userA"), connect.NewRequest(&clockkeeperv1.CreateGameRequest{
		ScriptId:    scriptID,
		PlayerCount: 7,
	}))
	require.NoError(t, err)
	gameID := gameResp.Msg.Game.Id

	// Find a valid traveller.
	charsResp, err := handler.ListCharacters(authedCtx("userA"), connect.NewRequest(&clockkeeperv1.ListCharactersRequest{
		Team: clockkeeperv1.Team_TEAM_TRAVELLER,
	}))
	require.NoError(t, err)
	require.NotEmpty(t, charsResp.Msg.Characters)
	travellerID := charsResp.Msg.Characters[0].Id

	_, err = handler.UpdateGameTravellers(authedCtx("userA"), connect.NewRequest(&clockkeeperv1.UpdateGameTravellersRequest{
		GameId:               gameID,
		SelectedTravellerIds: []string{travellerID},
	}))
	require.NoError(t, err)

	// User B tries to update alignment on user A's game.
	_, err = handler.UpdateTravellerAlignment(authedCtx("userB"), connect.NewRequest(&clockkeeperv1.UpdateTravellerAlignmentRequest{
		GameId:    gameID,
		RoleId:    travellerID,
		Alignment: clockkeeperv1.TravellerAlignment_TRAVELLER_ALIGNMENT_GOOD,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// --- CreateGame default name test ---

func TestCreateGame_HasDefaultName(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()

	hash, err := HashPassword("pass")
	require.NoError(t, err)
	_, err = handler.db.User.Create().SetUsername("namer").SetPasswordHash(hash).Save(ctx)
	require.NoError(t, err)

	// Find a system script.
	scriptsResp, err := handler.ListScripts(authedCtx("namer"), connect.NewRequest(&clockkeeperv1.ListScriptsRequest{}))
	require.NoError(t, err)
	var scriptID int64
	var scriptName string
	for _, s := range scriptsResp.Msg.Scripts {
		if s.IsSystem {
			scriptID = s.Id
			scriptName = s.Name
			break
		}
	}
	require.NotZero(t, scriptID)

	// Create a game.
	gameResp, err := handler.CreateGame(authedCtx("namer"), connect.NewRequest(&clockkeeperv1.CreateGameRequest{
		ScriptId:    scriptID,
		PlayerCount: 5,
	}))
	require.NoError(t, err)

	assert.Contains(t, gameResp.Msg.Game.Name, scriptName, "default game name should contain the script name")
}

// --- UpdateGameName tests ---

func TestUpdateGameName_Success(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()

	hash, err := HashPassword("pass")
	require.NoError(t, err)
	_, err = handler.db.User.Create().SetUsername("namer").SetPasswordHash(hash).Save(ctx)
	require.NoError(t, err)

	// Find a system script.
	scriptsResp, err := handler.ListScripts(authedCtx("namer"), connect.NewRequest(&clockkeeperv1.ListScriptsRequest{}))
	require.NoError(t, err)
	var scriptID int64
	for _, s := range scriptsResp.Msg.Scripts {
		if s.IsSystem {
			scriptID = s.Id
			break
		}
	}
	require.NotZero(t, scriptID)

	// Create a game.
	gameResp, err := handler.CreateGame(authedCtx("namer"), connect.NewRequest(&clockkeeperv1.CreateGameRequest{
		ScriptId:    scriptID,
		PlayerCount: 5,
	}))
	require.NoError(t, err)
	gameID := gameResp.Msg.Game.Id

	// Update the name.
	newName := "Epic Session #42"
	_, err = handler.UpdateGameName(authedCtx("namer"), connect.NewRequest(&clockkeeperv1.UpdateGameNameRequest{
		GameId: gameID,
		Name:   newName,
	}))
	require.NoError(t, err)

	// Verify via GetGame.
	getResp, err := handler.GetGame(authedCtx("namer"), connect.NewRequest(&clockkeeperv1.GetGameRequest{
		Id: gameID,
	}))
	require.NoError(t, err)
	assert.Equal(t, newName, getResp.Msg.Game.Name)
}

func TestUpdateGameName_BlocksOtherUser(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()

	hash, err := HashPassword("pass")
	require.NoError(t, err)
	_, err = handler.db.User.Create().SetUsername("userA").SetPasswordHash(hash).Save(ctx)
	require.NoError(t, err)
	_, err = handler.db.User.Create().SetUsername("userB").SetPasswordHash(hash).Save(ctx)
	require.NoError(t, err)

	// Find a system script.
	scriptsResp, err := handler.ListScripts(authedCtx("userA"), connect.NewRequest(&clockkeeperv1.ListScriptsRequest{}))
	require.NoError(t, err)
	var scriptID int64
	for _, s := range scriptsResp.Msg.Scripts {
		if s.IsSystem {
			scriptID = s.Id
			break
		}
	}
	require.NotZero(t, scriptID)

	// User A creates a game.
	gameResp, err := handler.CreateGame(authedCtx("userA"), connect.NewRequest(&clockkeeperv1.CreateGameRequest{
		ScriptId:    scriptID,
		PlayerCount: 5,
	}))
	require.NoError(t, err)

	// User B tries to update the name.
	_, err = handler.UpdateGameName(authedCtx("userB"), connect.NewRequest(&clockkeeperv1.UpdateGameNameRequest{
		GameId: gameResp.Msg.Game.Id,
		Name:   "Hacked Name",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestListGames_IncludesGameSummary(t *testing.T) {
	handler := testHandler(t)
	ownerName, game := startedGame(t, handler)

	// Record a death.
	_, err := handler.RecordDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: game.Id,
		RoleId: game.SelectedRoleIds[0],
	}))
	require.NoError(t, err)

	resp, err := handler.ListGames(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.ListGamesRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Games, 1)

	summary := resp.Msg.Games[0]
	assert.Equal(t, game.Id, summary.Id)
	assert.NotEmpty(t, summary.ScriptName)
	assert.Equal(t, int32(5), summary.PlayerCount)
	assert.Equal(t, clockkeeperv1.GameState_GAME_STATE_IN_PROGRESS, summary.State)
	assert.Equal(t, int32(1), summary.CurrentRound)
	assert.Equal(t, clockkeeperv1.PhaseType_PHASE_TYPE_NIGHT, summary.CurrentPhaseType)
	assert.Equal(t, int32(1), summary.DeathCount)
}

// --- ToggleNightAction tests ---

func TestToggleNightAction_MarkDone(t *testing.T) {
	handler := testHandler(t)
	ownerName, game := startedGame(t, handler)

	// Game starts at Night 1 — mark "dusk" as done.
	phaseId := game.PlayState.CurrentPhase.Id
	resp, err := handler.ToggleNightAction(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.ToggleNightActionRequest{
		GameId:   game.Id,
		PhaseId:  phaseId,
		ActionId: "dusk",
		Done:     true,
	}))
	require.NoError(t, err)

	g := resp.Msg.Game
	require.NotNil(t, g.PlayState)
	require.NotNil(t, g.PlayState.CurrentPhase)
	assert.Contains(t, g.PlayState.CurrentPhase.CompletedActions, "dusk")
}

func TestToggleNightAction_Unmark(t *testing.T) {
	handler := testHandler(t)
	ownerName, game := startedGame(t, handler)
	phaseId := game.PlayState.CurrentPhase.Id

	// Mark "dusk" as done.
	_, err := handler.ToggleNightAction(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.ToggleNightActionRequest{
		GameId:   game.Id,
		PhaseId:  phaseId,
		ActionId: "dusk",
		Done:     true,
	}))
	require.NoError(t, err)

	// Unmark "dusk".
	resp, err := handler.ToggleNightAction(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.ToggleNightActionRequest{
		GameId:   game.Id,
		PhaseId:  phaseId,
		ActionId: "dusk",
		Done:     false,
	}))
	require.NoError(t, err)

	g := resp.Msg.Game
	require.NotNil(t, g.PlayState)
	require.NotNil(t, g.PlayState.CurrentPhase)
	assert.NotContains(t, g.PlayState.CurrentPhase.CompletedActions, "dusk")
}

func TestToggleNightAction_FailsDayPhase(t *testing.T) {
	handler := testHandler(t)
	ownerName, game := startedGame(t, handler)

	// Advance Night 1 -> Day 1.
	advResp, err := handler.AdvancePhase(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.AdvancePhaseRequest{
		GameId: game.Id,
	}))
	require.NoError(t, err)
	dayPhaseId := advResp.Msg.Game.PlayState.CurrentPhase.Id

	// Try to toggle a night action on the day phase.
	_, err = handler.ToggleNightAction(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.ToggleNightActionRequest{
		GameId:   game.Id,
		PhaseId:  dayPhaseId,
		ActionId: "dusk",
		Done:     true,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestToggleNightAction_BlocksOtherUser(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()
	_, game := startedGame(t, handler)

	hash, err := HashPassword("pass")
	require.NoError(t, err)
	_, err = handler.db.User.Create().SetUsername("attacker").SetPasswordHash(hash).Save(ctx)
	require.NoError(t, err)

	// Another user tries to toggle a night action on the owner's game.
	_, err = handler.ToggleNightAction(authedCtx("attacker"), connect.NewRequest(&clockkeeperv1.ToggleNightActionRequest{
		GameId:   game.Id,
		PhaseId:  game.PlayState.CurrentPhase.Id,
		ActionId: "dusk",
		Done:     true,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// --- RemoveDeath ownership tests ---

func TestRemoveDeath_BlocksOtherUser(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()
	ownerName, game := startedGame(t, handler)

	require.NotEmpty(t, game.SelectedRoleIds)
	roleID := game.SelectedRoleIds[0]

	// Record a death as owner.
	deathResp, err := handler.RecordDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: game.Id,
		RoleId: roleID,
	}))
	require.NoError(t, err)
	require.Len(t, deathResp.Msg.Game.PlayState.AllDeaths, 1)
	deathID := deathResp.Msg.Game.PlayState.AllDeaths[0].Id

	// Create an attacker user.
	hash, err := HashPassword("pass")
	require.NoError(t, err)
	_, err = handler.db.User.Create().SetUsername("attacker").SetPasswordHash(hash).Save(ctx)
	require.NoError(t, err)

	// Attacker tries to remove the death.
	_, err = handler.RemoveDeath(authedCtx("attacker"), connect.NewRequest(&clockkeeperv1.RemoveDeathRequest{
		GameId:  game.Id,
		DeathId: deathID,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// --- UseGhostVote ownership tests ---

func TestUseGhostVote_BlocksOtherUser(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()
	ownerName, game := startedGame(t, handler)

	require.NotEmpty(t, game.SelectedRoleIds)
	roleID := game.SelectedRoleIds[0]

	// Record a death as owner.
	deathResp, err := handler.RecordDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: game.Id,
		RoleId: roleID,
	}))
	require.NoError(t, err)
	require.Len(t, deathResp.Msg.Game.PlayState.AllDeaths, 1)
	deathID := deathResp.Msg.Game.PlayState.AllDeaths[0].Id

	// Create an attacker user.
	hash, err := HashPassword("pass")
	require.NoError(t, err)
	_, err = handler.db.User.Create().SetUsername("attacker").SetPasswordHash(hash).Save(ctx)
	require.NoError(t, err)

	// Attacker tries to use the ghost vote.
	_, err = handler.UseGhostVote(authedCtx("attacker"), connect.NewRequest(&clockkeeperv1.UseGhostVoteRequest{
		GameId:  game.Id,
		DeathId: deathID,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// --- DeleteGame tests ---

func TestDeleteGame_Success(t *testing.T) {
	handler := testHandler(t)
	ownerName, game := startedGame(t, handler)

	// Record a death so the game has associated data.
	require.NotEmpty(t, game.SelectedRoleIds)
	_, err := handler.RecordDeath(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.RecordDeathRequest{
		GameId: game.Id,
		RoleId: game.SelectedRoleIds[0],
	}))
	require.NoError(t, err)

	// Delete the game.
	_, err = handler.DeleteGame(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.DeleteGameRequest{
		Id: game.Id,
	}))
	require.NoError(t, err)

	// Verify the game is gone.
	_, err = handler.GetGame(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.GetGameRequest{
		Id: game.Id,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))

	// Also verify via ListGames.
	listResp, err := handler.ListGames(authedCtx(ownerName), connect.NewRequest(&clockkeeperv1.ListGamesRequest{}))
	require.NoError(t, err)
	assert.Empty(t, listResp.Msg.Games)
}

func TestDeleteGame_BlocksOtherUser(t *testing.T) {
	handler := testHandler(t)
	ctx := context.Background()
	_, game := startedGame(t, handler)

	// Create an attacker user.
	hash, err := HashPassword("pass")
	require.NoError(t, err)
	_, err = handler.db.User.Create().SetUsername("attacker").SetPasswordHash(hash).Save(ctx)
	require.NoError(t, err)

	// Attacker tries to delete the game.
	_, err = handler.DeleteGame(authedCtx("attacker"), connect.NewRequest(&clockkeeperv1.DeleteGameRequest{
		Id: game.Id,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
