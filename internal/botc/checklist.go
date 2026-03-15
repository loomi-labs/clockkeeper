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
func GenerateSetupChecklist(chars []*Character, registry *Registry) []SetupStep {
	var steps []SetupStep

	// 1. Character tokens to prepare.
	var tokenNames []string
	for _, c := range chars {
		tokenNames = append(tokenNames, c.Name)
	}
	steps = append(steps, SetupStep{
		ID:             "prepare_tokens",
		Title:          "Prepare character tokens",
		Description:    fmt.Sprintf("Get out these character tokens: %s", strings.Join(tokenNames, ", ")),
		RequiresAction: true,
	})

	// 2. Setup modifications.
	if dist, err := DistributionForPlayerCount(len(chars)); err == nil {
		_, manual := ApplySetupModifiers(dist, chars)
		for _, m := range manual {
			steps = append(steps, SetupStep{
				ID:             fmt.Sprintf("setup_mod_%s", m.CharacterID),
				Title:          fmt.Sprintf("Setup: %s", m.CharacterName),
				Description:    m.Description,
				RequiresAction: true,
			})
		}
	}

	// 3. Reminder tokens.
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

	// 4. Jinxes.
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

	// 5. Bag tokens.
	steps = append(steps, SetupStep{
		ID:             "bag_tokens",
		Title:          "Put tokens in the bag",
		Description:    "Place all character tokens into the bag for distribution.",
		RequiresAction: true,
	})

	// 6. Distribute.
	steps = append(steps, SetupStep{
		ID:             "distribute_tokens",
		Title:          "Distribute tokens to players",
		Description:    "Pass the bag around. Each player draws a token and looks at it secretly.",
		RequiresAction: true,
	})

	// 7. Collect tokens back (optional).
	steps = append(steps, SetupStep{
		ID:             "collect_tokens",
		Title:          "Collect tokens back",
		Description:    "Collect all character tokens back from players.",
		RequiresAction: true,
	})

	// 8. Begin first night.
	steps = append(steps, SetupStep{
		ID:             "begin_night",
		Title:          "Begin first night",
		Description:    "Ask all players to close their eyes. The first night begins.",
		RequiresAction: true,
	})

	return steps
}
