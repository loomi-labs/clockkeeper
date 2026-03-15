package botc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDistributionForPlayerCount(t *testing.T) {
	tests := []struct {
		players  int
		expected Distribution
	}{
		{5, Distribution{3, 0, 1, 1}},
		{6, Distribution{3, 1, 1, 1}},
		{7, Distribution{5, 0, 1, 1}},
		{8, Distribution{5, 1, 1, 1}},
		{9, Distribution{5, 2, 1, 1}},
		{10, Distribution{7, 0, 2, 1}},
		{11, Distribution{7, 1, 2, 1}},
		{12, Distribution{7, 2, 2, 1}},
		{13, Distribution{9, 0, 3, 1}},
		{14, Distribution{9, 1, 3, 1}},
		{15, Distribution{9, 2, 3, 1}},
	}

	for _, tt := range tests {
		d, err := DistributionForPlayerCount(tt.players)
		require.NoError(t, err)
		assert.Equal(t, tt.expected, d, "player count %d", tt.players)
		assert.Equal(t, tt.players, d.Total(), "total should equal player count")
	}
}

func TestDistributionForPlayerCount_Invalid(t *testing.T) {
	for _, n := range []int{0, 4, 16, -1} {
		_, err := DistributionForPlayerCount(n)
		assert.Error(t, err, "player count %d should be invalid", n)
	}
}

func TestApplySetupModifiers_Baron(t *testing.T) {
	base := Distribution{Townsfolk: 5, Outsiders: 1, Minions: 1, Demons: 1}
	chars := []*Character{
		{ID: "baron", Team: TeamMinion, Setup: true},
	}

	d, manual := ApplySetupModifiers(base, chars)
	assert.Equal(t, 3, d.Townsfolk) // -2
	assert.Equal(t, 3, d.Outsiders) // +2
	assert.Equal(t, 1, d.Minions)
	assert.Equal(t, 1, d.Demons)
	assert.Empty(t, manual)
}

func TestApplySetupModifiers_Godfather(t *testing.T) {
	base := Distribution{Townsfolk: 5, Outsiders: 2, Minions: 1, Demons: 1}
	chars := []*Character{
		{ID: "godfather", Team: TeamMinion, Setup: true},
	}

	d, manual := ApplySetupModifiers(base, chars)
	assert.Equal(t, 6, d.Townsfolk) // +1
	assert.Equal(t, 1, d.Outsiders) // -1
	assert.Empty(t, manual)
}

func TestApplySetupModifiers_ClampOutsiders(t *testing.T) {
	// Godfather with 0 outsiders: outsiders stays 0, townsfolk unchanged.
	base := Distribution{Townsfolk: 5, Outsiders: 0, Minions: 1, Demons: 1}
	chars := []*Character{
		{ID: "godfather", Team: TeamMinion, Setup: true},
	}

	d, _ := ApplySetupModifiers(base, chars)
	assert.Equal(t, 0, d.Outsiders)
	assert.Equal(t, 5, d.Townsfolk) // clamp prevents any change
}

func TestApplySetupModifiers_ManualModifier(t *testing.T) {
	base := Distribution{Townsfolk: 5, Outsiders: 1, Minions: 1, Demons: 1}
	chars := []*Character{
		{ID: "huntsman", Team: TeamTownsfolk, Setup: true},
	}

	d, manual := ApplySetupModifiers(base, chars)
	// Huntsman doesn't change distribution automatically.
	assert.Equal(t, base, d)
	require.Len(t, manual, 1)
	assert.Equal(t, "huntsman", manual[0].CharacterID)
	assert.True(t, manual[0].Manual)
}

func TestApplySetupModifiers_UnknownSetupChar(t *testing.T) {
	base := Distribution{Townsfolk: 5, Outsiders: 1, Minions: 1, Demons: 1}
	chars := []*Character{
		{ID: "some_new_char", Team: TeamTownsfolk, Setup: true},
	}

	d, manual := ApplySetupModifiers(base, chars)
	assert.Equal(t, base, d)
	require.Len(t, manual, 1)
	assert.True(t, manual[0].Manual)
}

func TestApplySetupModifiers_NonSetupCharsIgnored(t *testing.T) {
	base := Distribution{Townsfolk: 5, Outsiders: 1, Minions: 1, Demons: 1}
	chars := []*Character{
		{ID: "washerwoman", Team: TeamTownsfolk, Setup: false},
		{ID: "imp", Team: TeamDemon, Setup: false},
	}

	d, manual := ApplySetupModifiers(base, chars)
	assert.Equal(t, base, d)
	assert.Empty(t, manual)
}

func TestRandomizeRoles(t *testing.T) {
	// Create a pool of characters for Trouble Brewing-like script.
	chars := []*Character{
		{ID: "washerwoman", Team: TeamTownsfolk},
		{ID: "librarian", Team: TeamTownsfolk},
		{ID: "investigator", Team: TeamTownsfolk},
		{ID: "chef", Team: TeamTownsfolk},
		{ID: "empath", Team: TeamTownsfolk},
		{ID: "fortuneteller", Team: TeamTownsfolk},
		{ID: "undertaker", Team: TeamTownsfolk},
		{ID: "monk", Team: TeamTownsfolk},
		{ID: "ravenkeeper", Team: TeamTownsfolk},
		{ID: "virgin", Team: TeamTownsfolk},
		{ID: "slayer", Team: TeamTownsfolk},
		{ID: "soldier", Team: TeamTownsfolk},
		{ID: "mayor", Team: TeamTownsfolk},
		{ID: "butler", Team: TeamOutsider},
		{ID: "drunk", Team: TeamOutsider, Setup: true},
		{ID: "recluse", Team: TeamOutsider},
		{ID: "saint", Team: TeamOutsider},
		{ID: "poisoner", Team: TeamMinion},
		{ID: "spy", Team: TeamMinion},
		{ID: "scarletwoman", Team: TeamMinion},
		{ID: "baron", Team: TeamMinion, Setup: true},
		{ID: "imp", Team: TeamDemon},
	}

	selected, err := RandomizeRoles(chars, 7)
	require.NoError(t, err)
	assert.Len(t, selected, 7)

	// Count teams.
	teamCounts := make(map[Team]int)
	idSet := make(map[string]bool)
	for _, id := range selected {
		idSet[id] = true
		for _, c := range chars {
			if c.ID == id {
				teamCounts[c.Team]++
				break
			}
		}
	}

	// No duplicates.
	assert.Len(t, idSet, 7)
	// Exactly 1 demon and 1 minion.
	assert.Equal(t, 1, teamCounts[TeamDemon])
	assert.Equal(t, 1, teamCounts[TeamMinion])
}

func TestRandomizeRoles_NotEnoughCharacters(t *testing.T) {
	chars := []*Character{
		{ID: "chef", Team: TeamTownsfolk},
		{ID: "imp", Team: TeamDemon},
	}

	_, err := RandomizeRoles(chars, 5)
	assert.Error(t, err)
}

func TestValidateDistribution(t *testing.T) {
	chars := []*Character{
		{ID: "washerwoman", Team: TeamTownsfolk},
		{ID: "librarian", Team: TeamTownsfolk},
		{ID: "investigator", Team: TeamTownsfolk},
		{ID: "chef", Team: TeamTownsfolk},
		{ID: "empath", Team: TeamTownsfolk},
		{ID: "recluse", Team: TeamOutsider},
		{ID: "poisoner", Team: TeamMinion},
		{ID: "imp", Team: TeamDemon},
	}

	err := ValidateDistribution(chars, 8)
	assert.NoError(t, err)
}

func TestValidateDistribution_WrongTotal(t *testing.T) {
	chars := []*Character{
		{ID: "chef", Team: TeamTownsfolk},
		{ID: "imp", Team: TeamDemon},
	}

	err := ValidateDistribution(chars, 5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "total roles")
}

func TestValidateDistribution_WrongDemonCount(t *testing.T) {
	chars := []*Character{
		{ID: "washerwoman", Team: TeamTownsfolk},
		{ID: "librarian", Team: TeamTownsfolk},
		{ID: "investigator", Team: TeamTownsfolk},
		{ID: "chef", Team: TeamTownsfolk},
		{ID: "empath", Team: TeamTownsfolk},
		{ID: "imp", Team: TeamDemon},
		{ID: "imp2", Team: TeamDemon},
	}

	err := ValidateDistribution(chars, 7)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "demon")
}
