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

	steps := GenerateSetupChecklist(chars, nil)
	require.Greater(t, len(steps), 0)

	stepIDs := make([]string, len(steps))
	for i, s := range steps {
		stepIDs[i] = s.ID
	}

	assert.Contains(t, stepIDs, "prepare_tokens")
	assert.Contains(t, stepIDs, "bag_tokens")
	assert.Contains(t, stepIDs, "distribute_tokens")
	assert.Contains(t, stepIDs, "collect_tokens")
	assert.Contains(t, stepIDs, "begin_night")
}

func TestGenerateSetupChecklist_WithReminders(t *testing.T) {
	chars := []*Character{
		{ID: "washerwoman", Name: "Washerwoman", Team: TeamTownsfolk, Reminders: []string{"Townsfolk", "Wrong"}},
		{ID: "imp", Name: "Imp", Team: TeamDemon, Reminders: []string{"Dead"}},
	}

	steps := GenerateSetupChecklist(chars, nil)

	stepIDs := make([]string, len(steps))
	for i, s := range steps {
		stepIDs[i] = s.ID
	}

	assert.Contains(t, stepIDs, "prepare_reminders")
}

func TestGenerateSetupChecklist_Empty(t *testing.T) {
	steps := GenerateSetupChecklist(nil, nil)
	require.Greater(t, len(steps), 0)

	stepIDs := make([]string, len(steps))
	for i, s := range steps {
		stepIDs[i] = s.ID
	}

	assert.Contains(t, stepIDs, "prepare_tokens")
	assert.Contains(t, stepIDs, "bag_tokens")
	assert.Contains(t, stepIDs, "distribute_tokens")
	assert.Contains(t, stepIDs, "collect_tokens")
	assert.Contains(t, stepIDs, "begin_night")
	assert.NotContains(t, stepIDs, "prepare_reminders")
}
