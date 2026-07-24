package web

import (
	"testing"

	"connectrpc.com/connect"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateDemonBluffs_PreservesPlayState is the regression test for the
// Save-then-serialize bug: on an in-progress game, UpdateDemonBluffs used to
// serialize the entity returned by Update().Save(), which has no eager-loaded
// phases, so PlayState came back nil and the in-progress view unmounted.
func TestUpdateDemonBluffs_PreservesPlayState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	handler := testHandler(t)
	ownerID, gameID := createTestGame(t, handler)
	setRoles(t, handler, ownerID, gameID, happyPathRoles)

	// Start the game so phases exist.
	_, err := handler.StartGame(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.StartGameRequest{GameId: gameID}))
	require.NoError(t, err)

	bluffs := []string{"undertaker", "ravenkeeper", "virgin"}
	resp, err := handler.UpdateDemonBluffs(authedCtx(ownerID), connect.NewRequest(&clockkeeperv1.UpdateDemonBluffsRequest{
		GameId:   gameID,
		BluffIds: bluffs,
	}))
	require.NoError(t, err)

	g := resp.Msg.Game
	// The response must carry play state with phases (was nil before the fix).
	require.NotNil(t, g.PlayState, "PlayState must survive the update")
	require.NotEmpty(t, g.PlayState.Phases, "PlayState must include phases")

	// Bluffs were applied.
	assert.Equal(t, bluffs, g.SelectedBluffIds)

	// Response matches a fresh GetGame.
	fresh := getGame(t, handler, ownerID, gameID)
	assert.Equal(t, fresh.SelectedBluffIds, g.SelectedBluffIds)
	require.NotNil(t, fresh.PlayState)
	assert.Len(t, g.PlayState.Phases, len(fresh.PlayState.Phases))
}

// Ownership (other user -> CodeNotFound) and the in-progress/completed state
// gates are already covered by TestUpdateDemonBluffs_BlocksOtherUser /
// _AllowedInProgress / _BlockedWhenCompleted in api_lifecycle_test.go.
