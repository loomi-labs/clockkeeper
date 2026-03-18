package botc

import (
	"fmt"
	"math/rand/v2"
)

// Distribution defines the team counts for a given player count.
type Distribution struct {
	Townsfolk int
	Outsiders int
	Minions   int
	Demons    int
}

// Total returns the sum of all roles.
func (d Distribution) Total() int {
	return d.Townsfolk + d.Outsiders + d.Minions + d.Demons
}

// baseDistributions maps player count (5–15) to the base role distribution.
var baseDistributions = map[int]Distribution{
	5:  {Townsfolk: 3, Outsiders: 0, Minions: 1, Demons: 1},
	6:  {Townsfolk: 3, Outsiders: 1, Minions: 1, Demons: 1},
	7:  {Townsfolk: 5, Outsiders: 0, Minions: 1, Demons: 1},
	8:  {Townsfolk: 5, Outsiders: 1, Minions: 1, Demons: 1},
	9:  {Townsfolk: 5, Outsiders: 2, Minions: 1, Demons: 1},
	10: {Townsfolk: 7, Outsiders: 0, Minions: 2, Demons: 1},
	11: {Townsfolk: 7, Outsiders: 1, Minions: 2, Demons: 1},
	12: {Townsfolk: 7, Outsiders: 2, Minions: 2, Demons: 1},
	13: {Townsfolk: 9, Outsiders: 0, Minions: 3, Demons: 1},
	14: {Townsfolk: 9, Outsiders: 1, Minions: 3, Demons: 1},
	15: {Townsfolk: 9, Outsiders: 2, Minions: 3, Demons: 1},
}

// DistributionForPlayerCount returns the base distribution for the given player count.
func DistributionForPlayerCount(n int) (Distribution, error) {
	d, ok := baseDistributions[n]
	if !ok {
		return Distribution{}, fmt.Errorf("unsupported player count: %d (must be 5–15)", n)
	}
	return d, nil
}

// setupModifiers maps character IDs to their outsider count modification.
// Positive values add outsiders (and remove townsfolk), negative values remove outsiders (and add townsfolk).
var setupModifiers = map[string]int{
	"baron":        +2,
	"godfather":    -1,
	"fanggu":       +1,
	"vigormortis":  -1,
	"xaan":         0, // Xaan's modifier depends on storyteller choice, handled manually
	"balloonist":   +1,
	"huntsman":     0, // Adds the Damsel, handled manually
	"villageidiot": 0, // Adds 0–2 Village Idiots, handled manually
	"hermit":       0, // Removes 0–1 outsider, handled manually
	"sentinel":     0, // ±1 outsider, handled manually
}

// SetupModifier describes a character's effect on role distribution.
type SetupModifier struct {
	CharacterID   string
	CharacterName string
	OutsiderDelta int
	Manual        bool // true if the modifier requires storyteller choice
	Description   string
}

// ApplySetupModifiers adjusts the distribution based on characters with setup=true.
// Returns the adjusted distribution and any modifiers that require manual handling.
func ApplySetupModifiers(base Distribution, characters []*Character) (Distribution, []SetupModifier) {
	d := base
	var manual []SetupModifier

	for _, c := range characters {
		if !c.Setup {
			continue
		}

		delta, known := setupModifiers[c.ID]
		if !known {
			manual = append(manual, SetupModifier{
				CharacterID:   c.ID,
				CharacterName: c.Name,
				Manual:        true,
				Description:   fmt.Sprintf("%s has a setup ability — adjust manually", c.Name),
			})
			continue
		}

		if delta == 0 && setupModifiers[c.ID] == 0 {
			// Known but requires manual handling.
			if c.ID == "xaan" || c.ID == "huntsman" || c.ID == "villageidiot" || c.ID == "hermit" || c.ID == "sentinel" {
				manual = append(manual, SetupModifier{
					CharacterID:   c.ID,
					CharacterName: c.Name,
					Manual:        true,
					Description:   fmt.Sprintf("%s: %s", c.Name, c.Ability),
				})
			}
			continue
		}

		d.Outsiders += delta
		d.Townsfolk -= delta

		// Clamp to valid ranges.
		if d.Outsiders < 0 {
			d.Townsfolk += d.Outsiders // add back the negative
			d.Outsiders = 0
		}
	}

	return d, manual
}

// pickRandom shuffles pool in place and returns the first n characters.
func pickRandom(pool []*Character, n int) ([]*Character, error) {
	if len(pool) < n {
		return nil, fmt.Errorf("not enough characters: need %d, have %d", n, len(pool))
	}
	rand.Shuffle(len(pool), func(i, j int) {
		pool[i], pool[j] = pool[j], pool[i]
	})
	return pool[:n], nil
}

// automaticDelta returns the automatic setup modifier delta for a character.
// Returns 0 if the character has no automatic modifier.
func automaticDelta(c *Character) int {
	if !c.Setup {
		return 0
	}
	delta, known := setupModifiers[c.ID]
	if !known || delta == 0 {
		return 0
	}
	return delta
}

// RandomizeRoles selects random characters from the given character pool to fill
// the distribution for the given player count. Returns the selected character IDs.
//
// Uses a two-round selection:
//   - Round 1a: pick demons + minions (fixed counts), apply their setup modifiers
//   - Round 1b: pick outsiders + townsfolk (adjusted counts), check for new modifiers
//   - Re-adjust and swap if Round 1b introduced new setup modifiers
func RandomizeRoles(characters []*Character, playerCount int) ([]string, error) {
	base, err := DistributionForPlayerCount(playerCount)
	if err != nil {
		return nil, err
	}

	// Group available characters by team.
	byTeam := map[Team][]*Character{
		TeamTownsfolk: {},
		TeamOutsider:  {},
		TeamMinion:    {},
		TeamDemon:     {},
	}
	for _, c := range characters {
		if _, ok := byTeam[c.Team]; ok {
			byTeam[c.Team] = append(byTeam[c.Team], c)
		}
	}

	// Round 1a: pick demons and minions (counts are fixed).
	demons, err := pickRandom(byTeam[TeamDemon], base.Demons)
	if err != nil {
		return nil, fmt.Errorf("not enough demon characters: %w", err)
	}
	minions, err := pickRandom(byTeam[TeamMinion], base.Minions)
	if err != nil {
		return nil, fmt.Errorf("not enough minion characters: %w", err)
	}

	// Adjust distribution based on selected demons + minions.
	evil := append(demons, minions...)
	dist, _ := ApplySetupModifiers(base, evil)

	// Clamp outsiders to available pool size.
	outsiderPool := byTeam[TeamOutsider]
	if dist.Outsiders > len(outsiderPool) {
		diff := dist.Outsiders - len(outsiderPool)
		dist.Outsiders = len(outsiderPool)
		dist.Townsfolk += diff
	}

	// Round 1b: pick outsiders and townsfolk with adjusted counts.
	outsiders, err := pickRandom(outsiderPool, dist.Outsiders)
	if err != nil {
		return nil, fmt.Errorf("not enough outsider characters: %w", err)
	}
	townsfolkPool := byTeam[TeamTownsfolk]
	townsfolk, err := pickRandom(townsfolkPool, dist.Townsfolk)
	if err != nil {
		return nil, fmt.Errorf("not enough townsfolk characters: %w", err)
	}

	// Re-adjust: check if any selected outsiders/townsfolk have automatic modifiers.
	goodDelta := 0
	for _, c := range outsiders {
		goodDelta += automaticDelta(c)
	}
	for _, c := range townsfolk {
		goodDelta += automaticDelta(c)
	}

	if goodDelta != 0 {
		// Need more outsiders (positive delta) or fewer (negative delta).
		if goodDelta > 0 {
			// Swap non-setup townsfolk for outsiders from remaining pool.
			remainingOutsiders := outsiderPool[dist.Outsiders:]
			for goodDelta > 0 && len(remainingOutsiders) > 0 {
				// Find a non-setup townsfolk to remove.
				swapped := false
				for i := len(townsfolk) - 1; i >= 0; i-- {
					if automaticDelta(townsfolk[i]) == 0 {
						townsfolk = append(townsfolk[:i], townsfolk[i+1:]...)
						outsiders = append(outsiders, remainingOutsiders[0])
						remainingOutsiders = remainingOutsiders[1:]
						swapped = true
						break
					}
				}
				if !swapped {
					break
				}
				goodDelta--
			}
		} else {
			// Swap outsiders for townsfolk from remaining pool.
			remainingTownsfolk := townsfolkPool[dist.Townsfolk:]
			for goodDelta < 0 && len(remainingTownsfolk) > 0 && len(outsiders) > 0 {
				// Find a non-setup outsider to remove.
				swapped := false
				for i := len(outsiders) - 1; i >= 0; i-- {
					if automaticDelta(outsiders[i]) == 0 {
						outsiders = append(outsiders[:i], outsiders[i+1:]...)
						townsfolk = append(townsfolk, remainingTownsfolk[0])
						remainingTownsfolk = remainingTownsfolk[1:]
						swapped = true
						break
					}
				}
				if !swapped {
					break
				}
				goodDelta++
			}
		}
	}

	// Collect all selected IDs.
	var selected []string
	for _, groups := range [][]*Character{demons, minions, outsiders, townsfolk} {
		for _, c := range groups {
			selected = append(selected, c.ID)
		}
	}

	return selected, nil
}

// ValidateDistribution checks whether a set of characters matches the expected
// distribution for a player count (accounting for setup modifiers).
func ValidateDistribution(characters []*Character, playerCount int) error {
	base, err := DistributionForPlayerCount(playerCount)
	if err != nil {
		return err
	}

	expected, _ := ApplySetupModifiers(base, characters)

	var actual Distribution
	for _, c := range characters {
		switch c.Team {
		case TeamTownsfolk:
			actual.Townsfolk++
		case TeamOutsider:
			actual.Outsiders++
		case TeamMinion:
			actual.Minions++
		case TeamDemon:
			actual.Demons++
		}
	}

	if actual.Total() != playerCount {
		return fmt.Errorf("total roles (%d) does not match player count (%d)", actual.Total(), playerCount)
	}
	if actual.Demons != expected.Demons {
		return fmt.Errorf("expected %d demon(s), got %d", expected.Demons, actual.Demons)
	}
	if actual.Minions != expected.Minions {
		return fmt.Errorf("expected %d minion(s), got %d", expected.Minions, actual.Minions)
	}

	return nil
}
