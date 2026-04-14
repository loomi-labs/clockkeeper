package botc

import (
	"testing"

	clockkeeper "github.com/loomi-labs/clockkeeper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRegistry(t *testing.T) *Registry {
	t.Helper()
	r, err := NewRegistry(clockkeeper.RolesJSON, clockkeeper.JinxesJSON, clockkeeper.NightSheetJSON)
	require.NoError(t, err)
	return r
}

func TestNewRegistry(t *testing.T) {
	r, err := NewRegistry(clockkeeper.RolesJSON, clockkeeper.JinxesJSON, clockkeeper.NightSheetJSON)
	require.NoError(t, err)
	assert.Greater(t, len(r.characters), 0)
}

func TestRegistry_Character(t *testing.T) {
	r := newTestRegistry(t)

	c, ok := r.Character("washerwoman")
	require.True(t, ok)
	assert.Equal(t, "Washerwoman", c.Name)
	assert.Equal(t, TeamTownsfolk, c.Team)
	assert.Equal(t, "tb", c.Edition)
}

func TestRegistry_Character_NotFound(t *testing.T) {
	r := newTestRegistry(t)

	_, ok := r.Character("nonexistent_character_xyz")
	assert.False(t, ok)
}

func TestRegistry_Characters(t *testing.T) {
	r := newTestRegistry(t)

	ids := []string{"washerwoman", "imp", "nonexistent_character_xyz"}
	chars := r.Characters(ids)

	assert.Len(t, chars, 2)

	foundIDs := make(map[string]bool)
	for _, c := range chars {
		foundIDs[c.ID] = true
	}
	assert.True(t, foundIDs["washerwoman"])
	assert.True(t, foundIDs["imp"])
}

func TestRegistry_Editions(t *testing.T) {
	r := newTestRegistry(t)

	editions := r.Editions()
	require.Len(t, editions, 3)

	editionIDs := make(map[string]bool)
	for _, e := range editions {
		editionIDs[e.ID] = true
		assert.Greater(t, len(e.Characters), 0, "edition %s should have characters", e.ID)
	}

	assert.True(t, editionIDs["tb"])
	assert.True(t, editionIDs["bmr"])
	assert.True(t, editionIDs["snv"])
}

func TestRegistry_CharactersByTeam(t *testing.T) {
	r := newTestRegistry(t)

	townsfolk := r.CharactersByTeam(TeamTownsfolk)
	assert.Greater(t, len(townsfolk), 0)

	for _, c := range townsfolk {
		assert.Equal(t, TeamTownsfolk, c.Team, "character %s should be townsfolk", c.ID)
	}
}

func TestRegistry_JinxesBetween(t *testing.T) {
	r := newTestRegistry(t)

	// magician has a jinx entry with spy
	jinxes := r.JinxesBetween([]string{"magician", "spy"})
	require.Greater(t, len(jinxes), 0)

	found := false
	for _, j := range jinxes {
		if j.ID == "magician/spy" {
			found = true
			assert.NotEmpty(t, j.Reason)
		}
	}
	assert.True(t, found, "expected jinx between magician and spy")
}
