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

// --- ApplySetupModifiers tests ---

func TestApplySetupModifiers_Baron(t *testing.T) {
	base := Distribution{Townsfolk: 5, Outsiders: 1, Minions: 1, Demons: 1}
	chars := []*Character{
		{ID: "baron", Team: TeamMinion, Setup: true},
	}

	result := ApplySetupModifiers(base, chars)
	assert.Equal(t, 3, result.Distribution.Townsfolk) // -2
	assert.Equal(t, 3, result.Distribution.Outsiders) // +2
	assert.Equal(t, 1, result.Distribution.Minions)
	assert.Equal(t, 1, result.Distribution.Demons)
	assert.Empty(t, result.ManualModifiers)
}

func TestApplySetupModifiers_Godfather(t *testing.T) {
	base := Distribution{Townsfolk: 5, Outsiders: 2, Minions: 1, Demons: 1}
	chars := []*Character{
		{ID: "godfather", Team: TeamMinion, Setup: true},
	}

	result := ApplySetupModifiers(base, chars)
	assert.Equal(t, 6, result.Distribution.Townsfolk) // +1
	assert.Equal(t, 1, result.Distribution.Outsiders) // -1
	assert.Empty(t, result.ManualModifiers)
}

func TestApplySetupModifiers_ClampOutsiders(t *testing.T) {
	// Godfather with 0 outsiders: outsiders stays 0, townsfolk unchanged.
	base := Distribution{Townsfolk: 5, Outsiders: 0, Minions: 1, Demons: 1}
	chars := []*Character{
		{ID: "godfather", Team: TeamMinion, Setup: true},
	}

	result := ApplySetupModifiers(base, chars)
	assert.Equal(t, 0, result.Distribution.Outsiders)
	assert.Equal(t, 5, result.Distribution.Townsfolk) // clamp prevents any change
}

func TestApplySetupModifiers_MinionDelta(t *testing.T) {
	// Lil' Monsta adds +1 minion, reduces townsfolk by 1.
	base := Distribution{Townsfolk: 7, Outsiders: 0, Minions: 2, Demons: 1}
	chars := []*Character{
		{ID: "lilmonsta", Team: TeamDemon, Setup: true},
	}

	result := ApplySetupModifiers(base, chars)
	assert.Equal(t, 6, result.Distribution.Townsfolk) // -1
	assert.Equal(t, 3, result.Distribution.Minions)   // +1
	assert.Equal(t, 0, result.Distribution.Outsiders)
	assert.Equal(t, 1, result.Distribution.Demons)
	assert.Empty(t, result.ManualModifiers)
}

func TestApplySetupModifiers_MinionDeltaClamp(t *testing.T) {
	// No real character has a negative MinionDelta, so temporarily register a
	// hypothetical one to exercise the clamping branch.
	setupModifiers["test_minion_reducer"] = SetupModifierDef{MinionDelta: -2}
	t.Cleanup(func() { delete(setupModifiers, "test_minion_reducer") })

	// Base has 1 minion; applying -2 would push it to -1 without clamping.
	base := Distribution{Townsfolk: 5, Outsiders: 0, Minions: 1, Demons: 1}
	chars := []*Character{
		{ID: "test_minion_reducer", Name: "Test Minion Reducer", Team: TeamMinion, Setup: true},
	}

	result := ApplySetupModifiers(base, chars)
	// Minions clamped from -1 to 0; the leftover -1 is absorbed by townsfolk.
	assert.Equal(t, 0, result.Distribution.Minions)
	assert.Equal(t, 6, result.Distribution.Townsfolk) // 5 - (-2) = 7, then clamp adds -1 → 6
	assert.Equal(t, 0, result.Distribution.Outsiders)
	assert.Equal(t, 1, result.Distribution.Demons)
}

func TestApplySetupModifiers_BagSubstitution(t *testing.T) {
	base := Distribution{Townsfolk: 5, Outsiders: 1, Minions: 1, Demons: 1}
	chars := []*Character{
		{ID: "drunk", Name: "Drunk", Team: TeamOutsider, Setup: true},
	}

	result := ApplySetupModifiers(base, chars)
	// Drunk doesn't change distribution.
	assert.Equal(t, base, result.Distribution)
	assert.Empty(t, result.ManualModifiers)
	// But it produces a bag substitution.
	require.Len(t, result.BagSubstitutions, 1)
	assert.Equal(t, "drunk", result.BagSubstitutions[0].CausedByID)
	assert.Equal(t, TeamTownsfolk, result.BagSubstitutions[0].Team)
}

func TestApplySetupModifiers_Companion(t *testing.T) {
	// Choirboy has a companion (King). ApplySetupModifiers doesn't handle companions
	// directly — it's done in RandomizeRoles. Verify it doesn't affect distribution.
	base := Distribution{Townsfolk: 5, Outsiders: 1, Minions: 1, Demons: 1}
	chars := []*Character{
		{ID: "choirboy", Name: "Choirboy", Team: TeamTownsfolk, Setup: true},
	}

	result := ApplySetupModifiers(base, chars)
	assert.Equal(t, base, result.Distribution)
	assert.Empty(t, result.ManualModifiers)
}

func TestApplySetupModifiers_ManualModifier(t *testing.T) {
	base := Distribution{Townsfolk: 5, Outsiders: 1, Minions: 1, Demons: 1}
	chars := []*Character{
		{ID: "marionette", Name: "Marionette", Team: TeamMinion, Setup: true, Ability: "You think you are a good character"},
	}

	result := ApplySetupModifiers(base, chars)
	assert.Equal(t, base, result.Distribution)
	require.Len(t, result.ManualModifiers, 1)
	assert.Equal(t, "marionette", result.ManualModifiers[0].CharacterID)
	assert.True(t, result.ManualModifiers[0].Manual)
}

func TestApplySetupModifiers_UnknownSetupChar(t *testing.T) {
	base := Distribution{Townsfolk: 5, Outsiders: 1, Minions: 1, Demons: 1}
	chars := []*Character{
		{ID: "some_new_char", Team: TeamTownsfolk, Setup: true},
	}

	result := ApplySetupModifiers(base, chars)
	assert.Equal(t, base, result.Distribution)
	require.Len(t, result.ManualModifiers, 1)
	assert.True(t, result.ManualModifiers[0].Manual)
}

func TestApplySetupModifiers_NonSetupCharsIgnored(t *testing.T) {
	base := Distribution{Townsfolk: 5, Outsiders: 1, Minions: 1, Demons: 1}
	chars := []*Character{
		{ID: "washerwoman", Team: TeamTownsfolk, Setup: false},
		{ID: "imp", Team: TeamDemon, Setup: false},
	}

	result := ApplySetupModifiers(base, chars)
	assert.Equal(t, base, result.Distribution)
	assert.Empty(t, result.ManualModifiers)
}

func TestApplySetupModifiers_CombinedModifiers(t *testing.T) {
	// Baron (+2 outsiders) + Lil' Monsta (+1 minion).
	base := Distribution{Townsfolk: 7, Outsiders: 0, Minions: 2, Demons: 1}
	chars := []*Character{
		{ID: "baron", Team: TeamMinion, Setup: true},
		{ID: "lilmonsta", Team: TeamDemon, Setup: true},
	}

	result := ApplySetupModifiers(base, chars)
	assert.Equal(t, 4, result.Distribution.Townsfolk) // 7 - 2 (baron) - 1 (lilmonsta)
	assert.Equal(t, 2, result.Distribution.Outsiders) // 0 + 2 (baron)
	assert.Equal(t, 3, result.Distribution.Minions)   // 2 + 1 (lilmonsta)
	assert.Equal(t, 1, result.Distribution.Demons)
}

// --- RandomizeRoles tests ---

func TestRandomizeRoles(t *testing.T) {
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

	result, err := RandomizeRoles(chars, 7)
	require.NoError(t, err)
	assert.Len(t, result.SelectedIDs, 7)

	// Count teams.
	teamCounts := make(map[Team]int)
	idSet := make(map[string]bool)
	for _, id := range result.SelectedIDs {
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

	result, err := RandomizeRoles(chars, 7)
	require.NoError(t, err)
	assert.Len(t, result.SelectedIDs, 7)

	// Baron must be selected (only minion).
	assert.Contains(t, result.SelectedIDs, "baron")

	// Count teams — expect 2 outsiders due to Baron.
	counts := countTeams(t, result.SelectedIDs, chars)
	assert.Equal(t, 3, counts[TeamTownsfolk])
	assert.Equal(t, 2, counts[TeamOutsider])
	assert.Equal(t, 1, counts[TeamMinion])
	assert.Equal(t, 1, counts[TeamDemon])
}

func TestRandomizeRoles_BaronNotInScript(t *testing.T) {
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

	result, err := RandomizeRoles(chars, 7)
	require.NoError(t, err)
	assert.Len(t, result.SelectedIDs, 7)

	counts := countTeams(t, result.SelectedIDs, chars)
	assert.Equal(t, 5, counts[TeamTownsfolk])
	assert.Equal(t, 0, counts[TeamOutsider])
	assert.Equal(t, 1, counts[TeamMinion])
	assert.Equal(t, 1, counts[TeamDemon])
}

func TestRandomizeRoles_BaronClampedToPool(t *testing.T) {
	// Baron selected but only 1 outsider available.
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

	result, err := RandomizeRoles(chars, 7)
	require.NoError(t, err)
	assert.Len(t, result.SelectedIDs, 7)

	counts := countTeams(t, result.SelectedIDs, chars)
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

	result, err := RandomizeRoles(chars, 10)
	require.NoError(t, err)
	assert.Len(t, result.SelectedIDs, 10)

	assert.Contains(t, result.SelectedIDs, "baron")
	assert.Contains(t, result.SelectedIDs, "fanggu")

	counts := countTeams(t, result.SelectedIDs, chars)
	assert.Equal(t, 4, counts[TeamTownsfolk]) // 7 - 3
	assert.Equal(t, 3, counts[TeamOutsider])  // 0 + 3
	assert.Equal(t, 2, counts[TeamMinion])
	assert.Equal(t, 1, counts[TeamDemon])
}

func TestRandomizeRoles_BalloonistFixup(t *testing.T) {
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

	foundBalloonist := false
	for range 50 {
		result, err := RandomizeRoles(chars, 8)
		require.NoError(t, err)
		assert.Len(t, result.SelectedIDs, 8)

		counts := countTeams(t, result.SelectedIDs, chars)
		hasBalloonist := false
		for _, id := range result.SelectedIDs {
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

func TestRandomizeRoles_LilMonstaExtraMinion(t *testing.T) {
	// Lil' Monsta is the only demon, adds +1 minion.
	// For 10 players: base {7, 0, 2, 1}, Lil' Monsta → {6, 0, 3, 1}.
	chars := []*Character{
		{ID: "washerwoman", Team: TeamTownsfolk},
		{ID: "librarian", Team: TeamTownsfolk},
		{ID: "investigator", Team: TeamTownsfolk},
		{ID: "chef", Team: TeamTownsfolk},
		{ID: "empath", Team: TeamTownsfolk},
		{ID: "fortuneteller", Team: TeamTownsfolk},
		{ID: "undertaker", Team: TeamTownsfolk},
		{ID: "monk", Team: TeamTownsfolk},
		{ID: "poisoner", Team: TeamMinion},
		{ID: "spy", Team: TeamMinion},
		{ID: "scarletwoman", Team: TeamMinion},
		{ID: "lilmonsta", Team: TeamDemon, Setup: true},
	}

	result, err := RandomizeRoles(chars, 10)
	require.NoError(t, err)
	assert.Len(t, result.SelectedIDs, 10)

	assert.Contains(t, result.SelectedIDs, "lilmonsta")

	counts := countTeams(t, result.SelectedIDs, chars)
	assert.Equal(t, 6, counts[TeamTownsfolk]) // 7 - 1 (minion delta)
	assert.Equal(t, 0, counts[TeamOutsider])
	assert.Equal(t, 3, counts[TeamMinion]) // 2 + 1
	assert.Equal(t, 1, counts[TeamDemon])
}

func TestRandomizeRoles_DrunkBagSubstitution(t *testing.T) {
	// When the Drunk is selected, a bag substitution should be returned
	// with a specific townsfolk character for the physical bag.
	chars := []*Character{
		{ID: "washerwoman", Name: "Washerwoman", Team: TeamTownsfolk},
		{ID: "librarian", Name: "Librarian", Team: TeamTownsfolk},
		{ID: "investigator", Name: "Investigator", Team: TeamTownsfolk},
		{ID: "chef", Name: "Chef", Team: TeamTownsfolk},
		{ID: "empath", Name: "Empath", Team: TeamTownsfolk},
		{ID: "fortuneteller", Name: "Fortune Teller", Team: TeamTownsfolk},
		{ID: "undertaker", Name: "Undertaker", Team: TeamTownsfolk},
		{ID: "monk", Name: "Monk", Team: TeamTownsfolk},
		{ID: "butler", Name: "Butler", Team: TeamOutsider},
		{ID: "drunk", Name: "Drunk", Team: TeamOutsider, Setup: true},
		{ID: "poisoner", Team: TeamMinion},
		{ID: "imp", Team: TeamDemon},
	}

	// Run multiple times — Drunk may or may not be selected.
	foundDrunk := false
	for range 50 {
		result, err := RandomizeRoles(chars, 8)
		require.NoError(t, err)

		hasDrunk := false
		for _, id := range result.SelectedIDs {
			if id == "drunk" {
				hasDrunk = true
				break
			}
		}

		if hasDrunk {
			foundDrunk = true
			require.Len(t, result.BagSubstitutions, 1)
			sub := result.BagSubstitutions[0]
			assert.Equal(t, "drunk", sub.CausedByID)
			assert.Equal(t, TeamTownsfolk, sub.Team)
			assert.NotEmpty(t, sub.CharacterID, "should pick a specific townsfolk")
			// The bag sub character should NOT be in the selected roles.
			assert.NotContains(t, result.SelectedIDs, sub.CharacterID,
				"bag sub townsfolk %q should not be in selected roles", sub.CharacterID)
		} else {
			assert.Empty(t, result.BagSubstitutions)
		}
	}
	assert.True(t, foundDrunk, "Drunk should have been selected at least once in 50 runs")
}

func TestRandomizeRoles_ChoirboyAddsKing(t *testing.T) {
	// Choirboy requires King. King is on the script, so it should be auto-added.
	chars := []*Character{
		{ID: "washerwoman", Team: TeamTownsfolk},
		{ID: "librarian", Team: TeamTownsfolk},
		{ID: "investigator", Team: TeamTownsfolk},
		{ID: "chef", Team: TeamTownsfolk},
		{ID: "empath", Team: TeamTownsfolk},
		{ID: "choirboy", Name: "Choirboy", Team: TeamTownsfolk, Setup: true},
		{ID: "king", Name: "King", Team: TeamTownsfolk},
		{ID: "butler", Team: TeamOutsider},
		{ID: "poisoner", Team: TeamMinion},
		{ID: "imp", Team: TeamDemon},
	}

	// Run multiple times to hit the case where Choirboy is selected.
	foundChoirboy := false
	for range 50 {
		result, err := RandomizeRoles(chars, 8)
		require.NoError(t, err)
		assert.Len(t, result.SelectedIDs, 8)

		hasChoirboy := false
		for _, id := range result.SelectedIDs {
			if id == "choirboy" {
				hasChoirboy = true
				break
			}
		}

		if hasChoirboy {
			foundChoirboy = true
			assert.Contains(t, result.SelectedIDs, "king",
				"King should be auto-added when Choirboy is selected")
		}
	}
	assert.True(t, foundChoirboy, "Choirboy should have been selected at least once in 50 runs")
}

func TestRandomizeRoles_HuntsmanAddsDamsel(t *testing.T) {
	// Huntsman requires Damsel. Damsel is on the script as an outsider.
	chars := []*Character{
		{ID: "washerwoman", Team: TeamTownsfolk},
		{ID: "librarian", Team: TeamTownsfolk},
		{ID: "investigator", Team: TeamTownsfolk},
		{ID: "chef", Team: TeamTownsfolk},
		{ID: "empath", Team: TeamTownsfolk},
		{ID: "huntsman", Name: "Huntsman", Team: TeamTownsfolk, Setup: true},
		{ID: "fortuneteller", Team: TeamTownsfolk},
		{ID: "butler", Team: TeamOutsider},
		{ID: "damsel", Name: "Damsel", Team: TeamOutsider},
		{ID: "poisoner", Team: TeamMinion},
		{ID: "imp", Team: TeamDemon},
	}

	// Run multiple times.
	foundHuntsman := false
	for range 50 {
		result, err := RandomizeRoles(chars, 8)
		require.NoError(t, err)
		assert.Len(t, result.SelectedIDs, 8)

		hasHuntsman := false
		for _, id := range result.SelectedIDs {
			if id == "huntsman" {
				hasHuntsman = true
				break
			}
		}

		if hasHuntsman {
			foundHuntsman = true
			// Damsel should be in the selected roles.
			// It may already be there as an outsider, or it may need special handling.
			// The companion logic replaces a townsfolk with the companion — but Damsel
			// is an outsider, not a townsfolk. Let's just verify it's present.
			assert.Contains(t, result.SelectedIDs, "damsel",
				"Damsel should be present when Huntsman is selected")
		}
	}
	assert.True(t, foundHuntsman, "Huntsman should have been selected at least once in 50 runs")
}

func TestRandomizeRoles_CompanionNotOnScript(t *testing.T) {
	// Choirboy is on the script but King is NOT.
	chars := []*Character{
		{ID: "washerwoman", Team: TeamTownsfolk},
		{ID: "librarian", Team: TeamTownsfolk},
		{ID: "investigator", Team: TeamTownsfolk},
		{ID: "chef", Team: TeamTownsfolk},
		{ID: "empath", Team: TeamTownsfolk},
		{ID: "choirboy", Name: "Choirboy", Team: TeamTownsfolk, Setup: true},
		// No "king" in the pool!
		{ID: "butler", Team: TeamOutsider},
		{ID: "poisoner", Team: TeamMinion},
		{ID: "imp", Team: TeamDemon},
	}

	// Run multiple times.
	foundManual := false
	for range 50 {
		result, err := RandomizeRoles(chars, 8)
		require.NoError(t, err)

		hasChoirboy := false
		for _, id := range result.SelectedIDs {
			if id == "choirboy" {
				hasChoirboy = true
				break
			}
		}

		if hasChoirboy {
			// Should produce a manual modifier since King is not on the script.
			for _, m := range result.ManualModifiers {
				if m.CharacterID == "choirboy" {
					foundManual = true
					assert.True(t, m.Manual)
					assert.Contains(t, m.Description, "king")
				}
			}
		}
	}
	assert.True(t, foundManual, "should have found manual modifier for Choirboy without King at least once")
}

func TestRandomizeRoles_NotEnoughCharacters(t *testing.T) {
	chars := []*Character{
		{ID: "chef", Team: TeamTownsfolk},
		{ID: "imp", Team: TeamDemon},
	}

	_, err := RandomizeRoles(chars, 5)
	assert.Error(t, err)
}

// --- Helpers ---

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

// --- SelectDemonBluffs tests ---

func TestSelectDemonBluffs_SelectsFromNotInPlay(t *testing.T) {
	chars := []*Character{
		{ID: "washerwoman", Team: TeamTownsfolk},
		{ID: "librarian", Team: TeamTownsfolk},
		{ID: "investigator", Team: TeamTownsfolk},
		{ID: "chef", Team: TeamTownsfolk},
		{ID: "empath", Team: TeamTownsfolk},
		{ID: "butler", Team: TeamOutsider},
		{ID: "recluse", Team: TeamOutsider},
		{ID: "poisoner", Team: TeamMinion},
		{ID: "spy", Team: TeamMinion},
		{ID: "imp", Team: TeamDemon},
	}

	selectedIDs := []string{"washerwoman", "librarian", "investigator", "chef", "empath"}

	bluffs := SelectDemonBluffs(chars, selectedIDs, 3)
	assert.Len(t, bluffs, 2, "only 2 unselected good chars available, should return 2")

	selectedSet := make(map[string]bool)
	for _, id := range selectedIDs {
		selectedSet[id] = true
	}
	for _, id := range bluffs {
		assert.False(t, selectedSet[id], "bluff %q should not be in the selected set", id)
	}
}

func TestSelectDemonBluffs_OnlyGoodTeams(t *testing.T) {
	chars := []*Character{
		{ID: "washerwoman", Team: TeamTownsfolk},
		{ID: "librarian", Team: TeamTownsfolk},
		{ID: "chef", Team: TeamTownsfolk},
		{ID: "butler", Team: TeamOutsider},
		{ID: "recluse", Team: TeamOutsider},
		{ID: "poisoner", Team: TeamMinion},
		{ID: "spy", Team: TeamMinion},
		{ID: "imp", Team: TeamDemon},
	}

	bluffs := SelectDemonBluffs(chars, nil, 3)
	assert.Len(t, bluffs, 3)

	lookup := make(map[string]*Character, len(chars))
	for _, c := range chars {
		lookup[c.ID] = c
	}
	for _, id := range bluffs {
		c, ok := lookup[id]
		require.True(t, ok, "bluff ID %q not found in characters", id)
		assert.True(t, c.Team == TeamTownsfolk || c.Team == TeamOutsider,
			"bluff %q has team %s, expected Townsfolk or Outsider", id, c.Team)
	}
}

func TestSelectDemonBluffs_RespectsCount(t *testing.T) {
	chars := []*Character{
		{ID: "washerwoman", Team: TeamTownsfolk},
		{ID: "librarian", Team: TeamTownsfolk},
		{ID: "investigator", Team: TeamTownsfolk},
		{ID: "chef", Team: TeamTownsfolk},
		{ID: "empath", Team: TeamTownsfolk},
		{ID: "butler", Team: TeamOutsider},
		{ID: "poisoner", Team: TeamMinion},
		{ID: "imp", Team: TeamDemon},
	}

	bluffs := SelectDemonBluffs(chars, nil, 3)
	assert.Len(t, bluffs, 3)

	selectedIDs := []string{"washerwoman", "librarian", "investigator", "chef", "empath"}
	bluffs = SelectDemonBluffs(chars, selectedIDs, 3)
	assert.Len(t, bluffs, 1)
}

func TestSelectDemonBluffs_EmptyInput(t *testing.T) {
	bluffs := SelectDemonBluffs(nil, nil, 3)
	assert.Empty(t, bluffs)

	chars := []*Character{
		{ID: "washerwoman", Team: TeamTownsfolk},
		{ID: "butler", Team: TeamOutsider},
		{ID: "imp", Team: TeamDemon},
	}
	selectedIDs := []string{"washerwoman", "butler"}
	bluffs = SelectDemonBluffs(chars, selectedIDs, 3)
	assert.Empty(t, bluffs)
}

// --- ValidateDistribution tests ---

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

func TestValidateDistribution_WithMinionDelta(t *testing.T) {
	// Lil' Monsta adds +1 minion. For 10 players: base {7, 0, 2, 1}, adjusted → {6, 0, 3, 1}.
	chars := []*Character{
		{ID: "washerwoman", Team: TeamTownsfolk},
		{ID: "librarian", Team: TeamTownsfolk},
		{ID: "investigator", Team: TeamTownsfolk},
		{ID: "chef", Team: TeamTownsfolk},
		{ID: "empath", Team: TeamTownsfolk},
		{ID: "fortuneteller", Team: TeamTownsfolk},
		{ID: "poisoner", Team: TeamMinion},
		{ID: "spy", Team: TeamMinion},
		{ID: "scarletwoman", Team: TeamMinion},
		{ID: "lilmonsta", Team: TeamDemon, Setup: true},
	}

	err := ValidateDistribution(chars, 10)
	assert.NoError(t, err)
}
