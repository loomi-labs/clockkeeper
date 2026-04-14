package botc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSetupChecklist_BasicSteps(t *testing.T) {
	chars := []*Character{
		{ID: "washerwoman", Name: "Washerwoman", Team: TeamTownsfolk},
		{ID: "imp", Name: "Imp", Team: TeamDemon},
	}

	steps := GenerateSetupChecklist(chars, nil, nil, nil)
	require.Greater(t, len(steps), 0)

	stepIDs := make([]string, len(steps))
	for i, s := range steps {
		stepIDs[i] = s.ID
	}

	assert.Contains(t, stepIDs, "prepare_tokens")
	assert.Contains(t, stepIDs, "bag_tokens")
	assert.Contains(t, stepIDs, "distribute_tokens")
	assert.Contains(t, stepIDs, "collect_tokens")
	assert.NotContains(t, stepIDs, "begin_night")
}

func TestGenerateSetupChecklist_WithReminders(t *testing.T) {
	chars := []*Character{
		{ID: "washerwoman", Name: "Washerwoman", Team: TeamTownsfolk, Reminders: []string{"Townsfolk", "Wrong"}},
		{ID: "imp", Name: "Imp", Team: TeamDemon, Reminders: []string{"Dead"}},
	}

	steps := GenerateSetupChecklist(chars, nil, nil, nil)

	stepIDs := make([]string, len(steps))
	for i, s := range steps {
		stepIDs[i] = s.ID
	}

	assert.Contains(t, stepIDs, "prepare_reminders")
}

func TestGenerateSetupChecklist_SetupCharacterSteps(t *testing.T) {
	chars := []*Character{
		{ID: "washerwoman", Name: "Washerwoman", Team: TeamTownsfolk},
		{ID: "baron", Name: "Baron", Team: TeamMinion, Setup: true, Ability: "There are extra Outsiders in play. [+2 Outsiders]"},
		{ID: "imp", Name: "Imp", Team: TeamDemon},
	}

	steps := GenerateSetupChecklist(chars, nil, nil, nil)

	stepIDs := make([]string, len(steps))
	for i, s := range steps {
		stepIDs[i] = s.ID
	}

	// Baron should have a setup step with its ability text.
	assert.Contains(t, stepIDs, "setup_mod_baron")
	for _, s := range steps {
		if s.ID == "setup_mod_baron" {
			assert.Equal(t, "Setup: Baron", s.Title)
			assert.Contains(t, s.Description, "+2 Outsiders")
			break
		}
	}
}

func TestGenerateSetupChecklist_WithBagSubstitutions(t *testing.T) {
	chars := []*Character{
		{ID: "washerwoman", Name: "Washerwoman", Team: TeamTownsfolk},
		{ID: "drunk", Name: "Drunk", Team: TeamOutsider, Setup: true},
		{ID: "imp", Name: "Imp", Team: TeamDemon},
	}
	bagSubs := []BagSubstitution{
		{CausedByID: "drunk", CausedByName: "Drunk", Team: TeamTownsfolk, CharacterID: "chef", CharacterName: "Chef"},
	}

	steps := GenerateSetupChecklist(chars, nil, bagSubs, nil)

	stepIDs := make([]string, len(steps))
	for i, s := range steps {
		stepIDs[i] = s.ID
	}

	// Setup and bag steps should be merged into one step for Drunk.
	assert.Contains(t, stepIDs, "setup_mod_drunk")
	assert.NotContains(t, stepIDs, "bag_sub_drunk") // merged into setup_mod_drunk

	// The setup step should contain the bag substitution info.
	for _, s := range steps {
		if s.ID == "setup_mod_drunk" {
			assert.Contains(t, s.Description, "Chef")
			assert.Contains(t, s.Description, "Drunk")
			break
		}
	}

	// Token list should show "Chef (for Drunk)" instead of just "Drunk".
	for _, s := range steps {
		if s.ID == "prepare_tokens" {
			assert.Contains(t, s.Description, "Chef (for Drunk)")
			assert.NotContains(t, s.Description, "Drunk,") // Should not list Drunk alone
			break
		}
	}
}

func TestGenerateSetupChecklist_Empty(t *testing.T) {
	steps := GenerateSetupChecklist(nil, nil, nil, nil)
	require.Greater(t, len(steps), 0)

	stepIDs := make([]string, len(steps))
	for i, s := range steps {
		stepIDs[i] = s.ID
	}

	assert.Contains(t, stepIDs, "prepare_tokens")
	assert.Contains(t, stepIDs, "bag_tokens")
	assert.Contains(t, stepIDs, "distribute_tokens")
	assert.Contains(t, stepIDs, "collect_tokens")
	assert.NotContains(t, stepIDs, "begin_night")
	assert.NotContains(t, stepIDs, "prepare_reminders")
}
