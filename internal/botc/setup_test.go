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

func TestRandomizeRoles_BaronSelected(t *testing.T) {
	// Baron is the only minion, so it must be selected.
	// For 7 players: base is {5, 0, 1, 1}. Baron adds +2 outsiders → {3, 2, 1, 1}.
	chars := []*Character{
		{ID: "washerwoman", Team: TeamTownsfolk},
		{ID: "librarian", Team: TeamTownsfolk},
		{ID: "investigator", Team: TeamTownsfolk},
		{ID: "chef", Team: TeamTownsfolk},
		{ID: "empath", Team: TeamTownsfolk},
		{ID: "fortuneteller", Team: TeamTownsfolk},
		{ID: "butler", Team: TeamOutsider},
		{ID: "drunk", Team: TeamOutsider, Setup: true},
		{ID: "recluse", Team: TeamOutsider},
		{ID: "saint", Team: TeamOutsider},
		{ID: "baron", Team: TeamMinion, Setup: true},
		{ID: "imp", Team: TeamDemon},
	}

	selected, err := RandomizeRoles(chars, 7)
	require.NoError(t, err)
	assert.Len(t, selected, 7)

	// Baron must be selected (only minion).
	assert.Contains(t, selected, "baron")

	// Count teams — expect 2 outsiders due to Baron.
	counts := countTeams(t, selected, chars)
	assert.Equal(t, 3, counts[TeamTownsfolk])
	assert.Equal(t, 2, counts[TeamOutsider])
	assert.Equal(t, 1, counts[TeamMinion])
	assert.Equal(t, 1, counts[TeamDemon])
}

func TestRandomizeRoles_BaronNotInScript(t *testing.T) {
	// No Baron in the script. For 7 players: base is {5, 0, 1, 1}, should stay unchanged.
	chars := []*Character{
		{ID: "washerwoman", Team: TeamTownsfolk},
		{ID: "librarian", Team: TeamTownsfolk},
		{ID: "investigator", Team: TeamTownsfolk},
		{ID: "chef", Team: TeamTownsfolk},
		{ID: "empath", Team: TeamTownsfolk},
		{ID: "fortuneteller", Team: TeamTownsfolk},
		{ID: "butler", Team: TeamOutsider},
		{ID: "recluse", Team: TeamOutsider},
		{ID: "poisoner", Team: TeamMinion},
		{ID: "imp", Team: TeamDemon},
	}

	selected, err := RandomizeRoles(chars, 7)
	require.NoError(t, err)
	assert.Len(t, selected, 7)

	counts := countTeams(t, selected, chars)
	assert.Equal(t, 5, counts[TeamTownsfolk])
	assert.Equal(t, 0, counts[TeamOutsider])
	assert.Equal(t, 1, counts[TeamMinion])
	assert.Equal(t, 1, counts[TeamDemon])
}

func TestRandomizeRoles_BaronClampedToPool(t *testing.T) {
	// Baron selected but only 1 outsider available.
	// For 7 players: base {5, 0, 1, 1}, Baron wants +2 outsiders but only 1 exists → {4, 1, 1, 1}.
	chars := []*Character{
		{ID: "washerwoman", Team: TeamTownsfolk},
		{ID: "librarian", Team: TeamTownsfolk},
		{ID: "investigator", Team: TeamTownsfolk},
		{ID: "chef", Team: TeamTownsfolk},
		{ID: "empath", Team: TeamTownsfolk},
		{ID: "fortuneteller", Team: TeamTownsfolk},
		{ID: "butler", Team: TeamOutsider}, // only 1 outsider
		{ID: "baron", Team: TeamMinion, Setup: true},
		{ID: "imp", Team: TeamDemon},
	}

	selected, err := RandomizeRoles(chars, 7)
	require.NoError(t, err)
	assert.Len(t, selected, 7)

	counts := countTeams(t, selected, chars)
	assert.Equal(t, 4, counts[TeamTownsfolk]) // 5 - 2 + 1 (clamped)
	assert.Equal(t, 1, counts[TeamOutsider])  // clamped to pool size
	assert.Equal(t, 1, counts[TeamMinion])
	assert.Equal(t, 1, counts[TeamDemon])
}

func TestRandomizeRoles_MultipleModifiers(t *testing.T) {
	// Baron (+2) and Fanggu (+1) both selected.
	// For 10 players: base {7, 0, 2, 1}, adjusted → {4, 3, 2, 1}.
	chars := []*Character{
		{ID: "washerwoman", Team: TeamTownsfolk},
		{ID: "librarian", Team: TeamTownsfolk},
		{ID: "investigator", Team: TeamTownsfolk},
		{ID: "chef", Team: TeamTownsfolk},
		{ID: "empath", Team: TeamTownsfolk},
		{ID: "fortuneteller", Team: TeamTownsfolk},
		{ID: "undertaker", Team: TeamTownsfolk},
		{ID: "monk", Team: TeamTownsfolk},
		{ID: "butler", Team: TeamOutsider},
		{ID: "drunk", Team: TeamOutsider, Setup: true},
		{ID: "recluse", Team: TeamOutsider},
		{ID: "saint", Team: TeamOutsider},
		{ID: "baron", Team: TeamMinion, Setup: true},
		{ID: "poisoner", Team: TeamMinion},
		{ID: "fanggu", Team: TeamDemon, Setup: true},
	}

	selected, err := RandomizeRoles(chars, 10)
	require.NoError(t, err)
	assert.Len(t, selected, 10)

	// Both Baron and Fanggu are the only options for their slots.
	assert.Contains(t, selected, "baron")
	assert.Contains(t, selected, "fanggu")

	counts := countTeams(t, selected, chars)
	assert.Equal(t, 4, counts[TeamTownsfolk]) // 7 - 3
	assert.Equal(t, 3, counts[TeamOutsider])  // 0 + 3
	assert.Equal(t, 2, counts[TeamMinion])
	assert.Equal(t, 1, counts[TeamDemon])
}

func TestRandomizeRoles_BalloonistFixup(t *testing.T) {
	// Balloonist is the only townsfolk with setup (+1 outsider).
	// For 8 players: base {5, 1, 1, 1}.
	// Balloonist is the only townsfolk option (forced selection), so +1 outsider → {4, 2, 1, 1}.
	chars := []*Character{
		{ID: "balloonist", Team: TeamTownsfolk, Setup: true},
		{ID: "chef", Team: TeamTownsfolk},
		{ID: "empath", Team: TeamTownsfolk},
		{ID: "fortuneteller", Team: TeamTownsfolk},
		{ID: "undertaker", Team: TeamTownsfolk},
		{ID: "monk", Team: TeamTownsfolk},
		{ID: "butler", Team: TeamOutsider},
		{ID: "drunk", Team: TeamOutsider, Setup: true},
		{ID: "recluse", Team: TeamOutsider},
		{ID: "saint", Team: TeamOutsider},
		{ID: "poisoner", Team: TeamMinion},
		{ID: "imp", Team: TeamDemon},
	}

	// Run multiple times to catch the case where Balloonist is selected.
	// With 5 slots from 6 townsfolk, Balloonist has 5/6 chance each run.
	foundBalloonist := false
	for range 50 {
		selected, err := RandomizeRoles(chars, 8)
		require.NoError(t, err)
		assert.Len(t, selected, 8)

		counts := countTeams(t, selected, chars)
		hasBalloonist := false
		for _, id := range selected {
			if id == "balloonist" {
				hasBalloonist = true
				break
			}
		}

		if hasBalloonist {
			foundBalloonist = true
			assert.Equal(t, 4, counts[TeamTownsfolk], "with Balloonist: should have 4 townsfolk")
			assert.Equal(t, 2, counts[TeamOutsider], "with Balloonist: should have 2 outsiders")
		} else {
			assert.Equal(t, 5, counts[TeamTownsfolk], "without Balloonist: should have 5 townsfolk")
			assert.Equal(t, 1, counts[TeamOutsider], "without Balloonist: should have 1 outsider")
		}
		assert.Equal(t, 1, counts[TeamMinion])
		assert.Equal(t, 1, counts[TeamDemon])
	}
	assert.True(t, foundBalloonist, "Balloonist should have been selected at least once in 50 runs")
}

// countTeams counts the team distribution of selected character IDs.
func countTeams(t *testing.T, selectedIDs []string, allChars []*Character) map[Team]int {
	t.Helper()
	lookup := make(map[string]*Character, len(allChars))
	for _, c := range allChars {
		lookup[c.ID] = c
	}
	counts := make(map[Team]int)
	for _, id := range selectedIDs {
		c, ok := lookup[id]
		require.True(t, ok, "selected ID %q not found in character pool", id)
		counts[c.Team]++
	}
	return counts
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
