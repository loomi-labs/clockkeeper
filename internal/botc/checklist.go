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
	CharacterIDs   []string
	Editions       []string
}

// GenerateSetupChecklist creates a dynamic setup checklist based on the selected characters.
// bagSubs contains bag substitutions from randomization (e.g., Drunk → townsfolk token).
func GenerateSetupChecklist(chars []*Character, registry *Registry, bagSubs []BagSubstitution, bluffIDs []string) []SetupStep {
	var steps []SetupStep

	// Build a set of bag substitution character IDs for token list adjustment.
	bagSubCausedBy := make(map[string]BagSubstitution, len(bagSubs))
	for _, bs := range bagSubs {
		bagSubCausedBy[bs.CausedByID] = bs
	}

	// 1. Character tokens to prepare.
	// For bag substitutions: list the substitute token instead of the original.
	var tokenNames []string
	var allIDs, allEditions []string
	for _, c := range chars {
		if bs, ok := bagSubCausedBy[c.ID]; ok && bs.CharacterName != "" {
			tokenNames = append(tokenNames, fmt.Sprintf("%s (for %s)", bs.CharacterName, c.Name))
		} else {
			tokenNames = append(tokenNames, c.Name)
		}
		allIDs = append(allIDs, c.ID)
		allEditions = append(allEditions, c.Edition)
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
		CharacterIDs:   allIDs,
		Editions:       allEditions,
	})

	// Track which bag subs were merged into setup steps.
	mergedBagSubs := make(map[string]bool)

	// 2. Setup modifications — one step per setup character showing ability text.
	// If the character also has a bag substitution, merge the bag step into this one.
	for _, c := range chars {
		if !c.Setup {
			continue
		}
		stepDesc := c.Ability
		charIDs := []string{c.ID}
		editions := []string{c.Edition}

		if bs, ok := bagSubCausedBy[c.ID]; ok && bs.CharacterName != "" {
			stepDesc += fmt.Sprintf("\n\nPut the %s token in the bag instead of the %s token.", bs.CharacterName, bs.CausedByName)
			charIDs = append(charIDs, bs.CharacterID)
			editions = append(editions, "") // substitute may not have a known edition
			mergedBagSubs[bs.CausedByID] = true
		}

		steps = append(steps, SetupStep{
			ID:             fmt.Sprintf("setup_mod_%s", c.ID),
			Title:          fmt.Sprintf("Setup: %s", c.Name),
			Description:    stepDesc,
			RequiresAction: true,
			CharacterIDs:   charIDs,
			Editions:       editions,
		})
	}

	// 3. Bag substitution steps (only those not already merged into setup steps).
	for _, bs := range bagSubs {
		if bs.CharacterName != "" && !mergedBagSubs[bs.CausedByID] {
			var charIDs, editions []string
			if bs.CharacterID != "" {
				charIDs = []string{bs.CharacterID}
				editions = []string{""}
			}
			steps = append(steps, SetupStep{
				ID:             fmt.Sprintf("bag_sub_%s", bs.CausedByID),
				Title:          fmt.Sprintf("Bag: %s", bs.CausedByName),
				Description:    fmt.Sprintf("Put the %s token in the bag instead of the %s token.", bs.CharacterName, bs.CausedByName),
				RequiresAction: false,
				CharacterIDs:   charIDs,
				Editions:       editions,
			})
		}
	}

	// 4. Reminder tokens.
	var reminders []string
	var reminderIDs, reminderEditions []string
	for _, c := range chars {
		hasReminders := len(c.Reminders) > 0 || len(c.RemindersGlobal) > 0
		for _, r := range c.Reminders {
			reminders = append(reminders, fmt.Sprintf("%s (%s)", r, c.Name))
		}
		for _, r := range c.RemindersGlobal {
			reminders = append(reminders, fmt.Sprintf("%s (%s)", r, c.Name))
		}
		if hasReminders {
			reminderIDs = append(reminderIDs, c.ID)
			reminderEditions = append(reminderEditions, c.Edition)
		}
	}
	if len(reminders) > 0 {
		steps = append(steps, SetupStep{
			ID:             "prepare_reminders",
			Title:          "Prepare reminder tokens",
			Description:    fmt.Sprintf("Get out these reminder tokens: %s", strings.Join(reminders, ", ")),
			RequiresAction: true,
			CharacterIDs:   reminderIDs,
			Editions:       reminderEditions,
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

	// 7b. Demon bluffs — prepare the 3 not-in-play good tokens to show the Demon.
	if len(bluffIDs) > 0 && registry != nil {
		bluffChars := registry.Characters(bluffIDs)
		// Only add bluff step if all IDs resolved — skip if stale data would
		// show an incomplete/misleading bluff list to the storyteller.
		if len(bluffChars) == len(bluffIDs) {
			var bluffNames []string
			var bluffCharIDs, bluffEditions []string
			for _, c := range bluffChars {
				bluffNames = append(bluffNames, c.Name)
				bluffCharIDs = append(bluffCharIDs, c.ID)
				bluffEditions = append(bluffEditions, c.Edition)
			}
			steps = append(steps, SetupStep{
				ID:             "prepare_bluffs",
				Title:          "Prepare demon bluff tokens",
				Description:    fmt.Sprintf("Get out these tokens to show the Demon as not-in-play characters: %s", strings.Join(bluffNames, ", ")),
				RequiresAction: true,
				CharacterIDs:   bluffCharIDs,
				Editions:       bluffEditions,
			})
		}
	}

	// 8. Collect tokens back (optional).
	steps = append(steps, SetupStep{
		ID:             "collect_tokens",
		Title:          "Collect tokens back",
		Description:    "Collect all character tokens back from players.",
		RequiresAction: true,
	})

	return steps
}
