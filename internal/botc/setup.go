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

// RandomizeRoles selects random characters from the given character pool to fill
// the distribution for the given player count. Returns the selected character IDs.
func RandomizeRoles(characters []*Character, playerCount int) ([]string, error) {
	base, err := DistributionForPlayerCount(playerCount)
	if err != nil {
		return nil, err
	}

	dist, _ := ApplySetupModifiers(base, characters)

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

	// Pick random characters for each team slot.
	var selected []string
	picks := []struct {
		team  Team
		count int
	}{
		{TeamDemon, dist.Demons},
		{TeamMinion, dist.Minions},
		{TeamOutsider, dist.Outsiders},
		{TeamTownsfolk, dist.Townsfolk},
	}

	for _, p := range picks {
		pool := byTeam[p.team]
		if len(pool) < p.count {
			return nil, fmt.Errorf("not enough %s characters: need %d, have %d", p.team, p.count, len(pool))
		}
		rand.Shuffle(len(pool), func(i, j int) {
			pool[i], pool[j] = pool[j], pool[i]
		})
		for i := range p.count {
			selected = append(selected, pool[i].ID)
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
