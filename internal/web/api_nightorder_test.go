package web

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/loomi-labs/clockkeeper/ent/game"
	clockkeeperv1 "github.com/loomi-labs/clockkeeper/gen/clockkeeper/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func nightPositions(t *testing.T, chars []*clockkeeperv1.Character) map[string]*clockkeeperv1.Character {
	t.Helper()
	byID := make(map[string]*clockkeeperv1.Character, len(chars))
	for _, c := range chars {
		byID[c.Id] = c
	}
	return byID
}

// A Trouble Brewing script must serve the printed night sheet's wake order:
// Spy right after Poisoner on the first night (not at the end), Undertaker
// between Ravenkeeper and Empath on other nights.
func TestGetScript_EditionScriptUsesPrintedNightOrder(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	handler := testHandler(t)
	ctx := context.Background()

	u, err := handler.db.User.Create().Save(ctx)
	require.NoError(t, err)

	created, err := handler.CreateScriptFromEdition(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.CreateScriptFromEditionRequest{
		EditionId: "tb",
	}))
	require.NoError(t, err)

	resp, err := handler.GetScript(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.GetScriptRequest{
		Id: created.Msg.Script.Id,
	}))
	require.NoError(t, err)

	chars := nightPositions(t, resp.Msg.Script.Characters)

	// First night: Poisoner < Spy < Washerwoman.
	assert.Less(t, chars["poisoner"].FirstNight, chars["spy"].FirstNight)
	assert.Less(t, chars["spy"].FirstNight, chars["washerwoman"].FirstNight)
	// Other nights: Monk < Spy < Scarlet Woman, Ravenkeeper < Undertaker < Empath.
	assert.Less(t, chars["monk"].OtherNight, chars["spy"].OtherNight)
	assert.Less(t, chars["spy"].OtherNight, chars["scarletwoman"].OtherNight)
	assert.Less(t, chars["ravenkeeper"].OtherNight, chars["undertaker"].OtherNight)
	assert.Less(t, chars["undertaker"].OtherNight, chars["empath"].OtherNight)
}

// Custom scripts keep the global night order (the one the official script
// tool prints): Spy wakes at the end of the night.
func TestGetScript_CustomScriptKeepsGlobalNightOrder(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	handler := testHandler(t)
	ctx := context.Background()

	u, err := handler.db.User.Create().Save(ctx)
	require.NoError(t, err)

	created, err := handler.CreateScript(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.CreateScriptRequest{
		Name:         "Custom",
		CharacterIds: []string{"poisoner", "spy", "washerwoman", "butler"},
	}))
	require.NoError(t, err)

	resp, err := handler.GetScript(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.GetScriptRequest{
		Id: created.Msg.Script.Id,
	}))
	require.NoError(t, err)

	chars := nightPositions(t, resp.Msg.Script.Characters)
	assert.Less(t, chars["poisoner"].FirstNight, chars["washerwoman"].FirstNight)
	assert.Less(t, chars["butler"].FirstNight, chars["spy"].FirstNight, "custom scripts wake the Spy last")
}

// Games on an edition script serve the printed order for their selected
// characters (exercises the script edge eager-loading in getOwnedGame).
func TestGetGame_EditionGameUsesPrintedNightOrder(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	handler := testHandler(t)
	ctx := context.Background()

	u, err := handler.db.User.Create().Save(ctx)
	require.NoError(t, err)

	created, err := handler.CreateScriptFromEdition(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.CreateScriptFromEditionRequest{
		EditionId: "tb",
	}))
	require.NoError(t, err)

	g, err := handler.db.Game.Create().
		SetName("TB Game").
		SetUserID(u.ID).
		SetScriptID(int(created.Msg.Script.Id)).
		SetPlayerCount(7).
		SetSelectedRoles([]string{"poisoner", "spy", "washerwoman", "imp"}).
		SetSelectedTravellers([]string{}).
		SetExtraCharacters([]string{}).
		SetState(game.StateSetup).
		Save(ctx)
	require.NoError(t, err)

	resp, err := handler.GetGame(authedCtx(u.ID), connect.NewRequest(&clockkeeperv1.GetGameRequest{
		Id: int64(g.ID),
	}))
	require.NoError(t, err)

	chars := nightPositions(t, resp.Msg.Game.SelectedCharacters)
	assert.Less(t, chars["poisoner"].FirstNight, chars["spy"].FirstNight)
	assert.Less(t, chars["spy"].FirstNight, chars["washerwoman"].FirstNight)
}
