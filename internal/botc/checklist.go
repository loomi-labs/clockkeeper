package botc

import (
	"fmt"
	"strings"
)

// SetupStep represents a step in the game setup checklist.
type SetupStep struct {
	ID             string
	Title          string
	Description    string
	RequiresAction bool
}

// GenerateSetupChecklist creates a dynamic setup checklist based on the selected characters.
// bagSubs contains bag substitutions from randomization (e.g., Drunk → townsfolk token).
func GenerateSetupChecklist(chars []*Character, registry *Registry, bagSubs []BagSubstitution) []SetupStep {
	var steps []SetupStep

	// Build a set of bag substitution character IDs for token list adjustment.
	bagSubCausedBy := make(map[string]BagSubstitution, len(bagSubs))
	for _, bs := range bagSubs {
		bagSubCausedBy[bs.CausedByID] = bs
	}

	// 1. Character tokens to prepare.
	// For bag substitutions: list the substitute token instead of the original.
	var tokenNames []string
	for _, c := range chars {
		if bs, ok := bagSubCausedBy[c.ID]; ok && bs.CharacterName != "" {
			tokenNames = append(tokenNames, fmt.Sprintf("%s (for %s)", bs.CharacterName, c.Name))
		} else {
			tokenNames = append(tokenNames, c.Name)
		}
	}
	desc := "No character tokens to prepare."
	if len(tokenNames) > 0 {
		desc = fmt.Sprintf("Get out these character tokens: %s", strings.Join(tokenNames, ", "))
	}
	steps = append(steps, SetupStep{
		ID:             "prepare_tokens",
		Title:          "Prepare character tokens",
		Description:    desc,
		RequiresAction: true,
	})

	// 2. Setup modifications — one step per setup character showing ability text.
	for _, c := range chars {
		if !c.Setup {
			continue
		}
		steps = append(steps, SetupStep{
			ID:             fmt.Sprintf("setup_mod_%s", c.ID),
			Title:          fmt.Sprintf("Setup: %s", c.Name),
			Description:    c.Ability,
			RequiresAction: true,
		})
	}

	// 3. Bag substitution steps.
	for _, bs := range bagSubs {
		if bs.CharacterName != "" {
			steps = append(steps, SetupStep{
				ID:             fmt.Sprintf("bag_sub_%s", bs.CausedByID),
				Title:          fmt.Sprintf("Bag: %s", bs.CausedByName),
				Description:    fmt.Sprintf("Put the %s token in the bag instead of the %s token.", bs.CharacterName, bs.CausedByName),
				RequiresAction: false,
			})
		}
	}

	// 4. Reminder tokens.
	var reminders []string
	for _, c := range chars {
		for _, r := range c.Reminders {
			reminders = append(reminders, fmt.Sprintf("%s (%s)", r, c.Name))
		}
		for _, r := range c.RemindersGlobal {
			reminders = append(reminders, fmt.Sprintf("%s (%s)", r, c.Name))
		}
	}
	if len(reminders) > 0 {
		steps = append(steps, SetupStep{
			ID:             "prepare_reminders",
			Title:          "Prepare reminder tokens",
			Description:    fmt.Sprintf("Get out these reminder tokens: %s", strings.Join(reminders, ", ")),
			RequiresAction: true,
		})
	}

	// 5. Jinxes.
	if registry != nil {
		charIDs := make([]string, len(chars))
		for i, c := range chars {
			charIDs[i] = c.ID
		}
		jinxes := registry.JinxesBetween(charIDs)
		if len(jinxes) > 0 {
			var jinxDescs []string
			for _, j := range jinxes {
				jinxDescs = append(jinxDescs, fmt.Sprintf("• %s: %s", j.ID, j.Reason))
			}
			steps = append(steps, SetupStep{
				ID:             "check_jinxes",
				Title:          "Review jinxes",
				Description:    strings.Join(jinxDescs, "\n"),
				RequiresAction: false,
			})
		}
	}

	// 6. Bag tokens.
	steps = append(steps, SetupStep{
		ID:             "bag_tokens",
		Title:          "Put tokens in the bag",
		Description:    "Place all character tokens into the bag for distribution.",
		RequiresAction: true,
	})

	// 7. Distribute.
	steps = append(steps, SetupStep{
		ID:             "distribute_tokens",
		Title:          "Distribute tokens to players",
		Description:    "Pass the bag around. Each player draws a token and looks at it secretly.",
		RequiresAction: true,
	})

	// 8. Collect tokens back (optional).
	steps = append(steps, SetupStep{
		ID:             "collect_tokens",
		Title:          "Collect tokens back",
		Description:    "Collect all character tokens back from players.",
		RequiresAction: true,
	})

	// 9. Begin first night.
	steps = append(steps, SetupStep{
		ID:             "begin_night",
		Title:          "Begin first night",
		Description:    "Ask all players to close their eyes. The first night begins.",
		RequiresAction: true,
	})

	return steps
}
