package botc

import (
	"errors"
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

// SetupModifierDef defines a character's setup modification rules.
type SetupModifierDef struct {
	OutsiderDelta int    // Change to outsider count (townsfolk adjusts inversely)
	MinionDelta   int    // Change to minion count (townsfolk adjusts inversely)
	Companion     string // Required companion character ID (e.g., "king" for Choirboy)
	BagTeam       Team   // If set, pick an extra character of this team for the bag (Drunk → TeamTownsfolk)
	Manual        bool   // Always requires storyteller intervention
}

// setupModifiers maps character IDs to their setup modification rules.
var setupModifiers = map[string]SetupModifierDef{
	// Automatic outsider delta.
	"baron":       {OutsiderDelta: +2},
	"godfather":   {OutsiderDelta: -1},
	"fanggu":      {OutsiderDelta: +1},
	"vigormortis": {OutsiderDelta: -1},
	"balloonist":  {OutsiderDelta: +1},

	// Automatic minion delta.
	"lilmonsta":     {MinionDelta: +1},
	"lordoftyphon":  {MinionDelta: +1},

	// Bag substitution: pick an extra character of this team for the physical bag.
	"drunk": {BagTeam: TeamTownsfolk},

	// Companion: auto-add a required character.
	"choirboy": {Companion: "king"},
	"huntsman": {Companion: "damsel"},

	// Manual: requires storyteller decision.
	"marionette":   {Manual: true},
	"summoner":     {Manual: true},
	"bountyhunter": {Manual: true},
	"atheist":      {Manual: true},
	"legion":       {Manual: true},
	"kazali":       {Manual: true},
	"xaan":         {Manual: true},
	"villageidiot": {Manual: true},
	"hermit":       {Manual: true},
	"sentinel":     {Manual: true},
	"deusexfiasco": {Manual: true},
	"pope":         {Manual: true},
	"tor":          {Manual: true},
}

// BagTeamForCharacter returns the bag substitution team for a character, if any.
func BagTeamForCharacter(characterID string) Team {
	if def, ok := setupModifiers[characterID]; ok {
		return def.BagTeam
	}
	return ""
}

// SetupModifier describes a character's effect on role distribution (returned to callers).
type SetupModifier struct {
	CharacterID   string
	CharacterName string
	OutsiderDelta int
	Manual        bool // true if the modifier requires storyteller choice
	Description   string
}

// BagSubstitution represents an extra token needed in the physical bag.
type BagSubstitution struct {
	CausedByID   string // Character that causes this (e.g., "drunk")
	CausedByName string
	Team         Team   // Team of the extra token (e.g., TeamTownsfolk)
	CharacterID  string // Specific character picked (filled by RandomizeRoles)
	CharacterName string
}

// SetupResult holds the full result of applying setup modifiers.
type SetupResult struct {
	Distribution     Distribution
	ManualModifiers  []SetupModifier
	BagSubstitutions []BagSubstitution
}

// RandomizeResult holds the result of role randomization.
type RandomizeResult struct {
	SelectedIDs      []string
	BagSubstitutions []BagSubstitution
	ManualModifiers  []SetupModifier
}

// ApplySetupModifiers adjusts the distribution based on characters with setup=true.
// Returns the adjusted distribution, manual modifiers, and bag substitution needs.
func ApplySetupModifiers(base Distribution, characters []*Character) SetupResult {
	d := base
	result := SetupResult{Distribution: d}

	for _, c := range characters {
		if !c.Setup {
			continue
		}

		def, known := setupModifiers[c.ID]
		if !known {
			result.ManualModifiers = append(result.ManualModifiers, SetupModifier{
				CharacterID:   c.ID,
				CharacterName: c.Name,
				Manual:        true,
				Description:   fmt.Sprintf("%s has a setup ability — adjust manually", c.Name),
			})
			continue
		}

		if def.Manual {
			result.ManualModifiers = append(result.ManualModifiers, SetupModifier{
				CharacterID:   c.ID,
				CharacterName: c.Name,
				Manual:        true,
				Description:   fmt.Sprintf("%s: %s", c.Name, c.Ability),
			})
			continue
		}

		// Apply automatic outsider delta.
		if def.OutsiderDelta != 0 {
			d.Outsiders += def.OutsiderDelta
			d.Townsfolk -= def.OutsiderDelta
		}

		// Apply automatic minion delta.
		if def.MinionDelta != 0 {
			d.Minions += def.MinionDelta
			d.Townsfolk -= def.MinionDelta
		}

		// Track bag substitution needs.
		if def.BagTeam != "" {
			result.BagSubstitutions = append(result.BagSubstitutions, BagSubstitution{
				CausedByID:   c.ID,
				CausedByName: c.Name,
				Team:         def.BagTeam,
			})
		}

		// Companion requirements are handled by RandomizeRoles, not here.
	}

	// Clamp to valid ranges.
	if d.Outsiders < 0 {
		d.Townsfolk += d.Outsiders
		d.Outsiders = 0
	}
	if d.Minions < 0 {
		d.Townsfolk += d.Minions
		d.Minions = 0
	}

	result.Distribution = d
	return result
}

// lockConflictError marks a randomization failure caused by locked roles that do
// not fit the distribution. RandomizeRoles retries these (a different evil pick
// may free up the needed slots) before giving up.
type lockConflictError struct{ msg string }

func (e *lockConflictError) Error() string { return e.msg }

func lockConflict(format string, args ...any) error {
	return &lockConflictError{msg: fmt.Sprintf(format, args...)}
}

func isLockConflict(err error) bool {
	var target *lockConflictError
	return errors.As(err, &target)
}

// pickRandom shuffles pool in place and returns the first n characters.
func pickRandom(pool []*Character, n int) ([]*Character, error) {
	return pickRandomLocked(pool, nil, n, "")
}

// pickRandomLocked returns n characters from pool, guaranteeing that every locked
// character in the pool is part of the result.
//
// The pool is reordered in place: locked characters move to the front (they become
// the first picks), the remainder is shuffled. That keeps the caller's convention
// of "picks are pool[:n], leftovers are pool[n:]" intact, and leftovers never
// contain a locked character.
//
// With an empty locked set this is a plain shuffle-and-take-n.
func pickRandomLocked(pool []*Character, locked map[string]bool, n int, team string) ([]*Character, error) {
	if len(pool) < n {
		return nil, fmt.Errorf("not enough characters: need %d, have %d", n, len(pool))
	}

	lockedCount := 0
	if len(locked) > 0 {
		for i := range pool {
			if locked[pool[i].ID] {
				pool[lockedCount], pool[i] = pool[i], pool[lockedCount]
				lockedCount++
			}
		}
		if lockedCount > n {
			return nil, lockConflict("%d locked %s role(s) exceed the %d available %s slot(s)", lockedCount, team, n, team)
		}
	}

	rest := pool[lockedCount:]
	rand.Shuffle(len(rest), func(i, j int) {
		rest[i], rest[j] = rest[j], rest[i]
	})
	return pool[:n], nil
}

// wrapPickErr turns a pool-exhaustion error into a team-specific message while
// leaving lock conflicts (which carry their own message) untouched.
func wrapPickErr(team string, err error) error {
	if isLockConflict(err) {
		return err
	}
	return fmt.Errorf("not enough %s characters: %w", team, err)
}

// automaticOutsiderDelta returns the automatic outsider delta for a character.
// Returns 0 if the character has no automatic outsider modifier.
func automaticOutsiderDelta(c *Character) int {
	if !c.Setup {
		return 0
	}
	def, known := setupModifiers[c.ID]
	if !known || def.Manual {
		return 0
	}
	return def.OutsiderDelta
}

// RandomizeRoles selects random characters from the given character pool to fill
// the distribution for the given player count.
//
// lockedRoleIDs are characters the caller wants to keep (the setup UI locks seats
// by role id so the player name attached to that role survives a re-randomize).
// Locked characters are pre-placed into their team's picks and only the remaining
// slots are filled randomly. Locked ids must be part of the character pool;
// travellers are ignored here (they are not part of the distribution).
//
// Uses a two-round selection:
//   - Round 1a: pick demons + minions (base counts), apply their setup modifiers
//   - Pick extra minions if needed (e.g., Lil' Monsta)
//   - Round 1b: pick outsiders + townsfolk (adjusted counts), check for new modifiers
//   - Re-adjust and swap if Round 1b introduced new setup modifiers
//   - Handle companions (Choirboy→King, Huntsman→Damsel)
//   - Pick bag substitutions (Drunk→random townsfolk)
func RandomizeRoles(characters []*Character, playerCount int, lockedRoleIDs []string) (*RandomizeResult, error) {
	base, err := DistributionForPlayerCount(playerCount)
	if err != nil {
		return nil, err
	}

	charByID := make(map[string]*Character, len(characters))
	for _, c := range characters {
		charByID[c.ID] = c
	}

	locked := make(map[string]bool, len(lockedRoleIDs))
	lockedByTeam := make(map[Team]int, 4)
	for _, id := range lockedRoleIDs {
		c, onScript := charByID[id]
		if !onScript {
			return nil, fmt.Errorf("locked role not on script: %s", id)
		}
		if locked[id] {
			continue
		}
		locked[id] = true
		lockedByTeam[c.Team]++
	}

	// The demon count never changes, so an overflow here can never be resolved.
	if lockedByTeam[TeamDemon] > base.Demons {
		return nil, lockConflict("%d locked demon role(s) exceed the %d demon slot(s) for %d players",
			lockedByTeam[TeamDemon], base.Demons, playerCount)
	}

	// Which slots exist depends on the (random) evil pick — e.g. a locked outsider
	// only fits at 7 players if the picked minion is a Baron. Retry a few times
	// before reporting the conflict.
	attempts := 1
	if len(locked) > 0 {
		attempts = 100
	}
	var lastErr error
	for i := 0; i < attempts; i++ {
		result, err := randomizeRoles(characters, charByID, playerCount, base, locked, lockedByTeam)
		if err == nil {
			return result, nil
		}
		if !isLockConflict(err) {
			return nil, err
		}
		lastErr = err
	}
	return nil, lastErr
}

// randomizeRoles is one randomization attempt. See RandomizeRoles.
func randomizeRoles(
	characters []*Character,
	charByID map[string]*Character,
	playerCount int,
	base Distribution,
	locked map[string]bool,
	lockedByTeam map[Team]int,
) (*RandomizeResult, error) {
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

	// Round 1a: pick demons and minions at base counts (locked evil characters are
	// part of this pick so their setup modifiers shape the distribution).
	demons, err := pickRandomLocked(byTeam[TeamDemon], locked, base.Demons, "demon")
	if err != nil {
		return nil, wrapPickErr("demon", err)
	}
	minionCount := max(base.Minions, lockedByTeam[TeamMinion])
	minions, err := pickRandomLocked(byTeam[TeamMinion], locked, minionCount, "minion")
	if err != nil {
		return nil, wrapPickErr("minion", err)
	}

	// Adjust distribution based on selected evil characters.
	evil := make([]*Character, 0, len(demons)+len(minions))
	evil = append(evil, demons...)
	evil = append(evil, minions...)
	setupResult := ApplySetupModifiers(base, evil)
	dist := setupResult.Distribution

	// More locked minions than the game has minion slots (even after modifiers).
	if dist.Minions < len(minions) {
		return nil, lockConflict("%d locked minion role(s) exceed the %d minion slot(s) for %d players",
			lockedByTeam[TeamMinion], dist.Minions, playerCount)
	}

	// Pick extra minions if needed (e.g., Lil' Monsta adds +1 minion).
	if dist.Minions > len(minions) {
		extraNeeded := dist.Minions - len(minions)
		remainingMinions := byTeam[TeamMinion][len(minions):]
		extra, err := pickRandom(remainingMinions, extraNeeded)
		if err != nil {
			// Not enough minions in pool — clamp and compensate with townsfolk.
			extra = remainingMinions
			dist.Minions = len(minions) + len(extra)
			dist.Townsfolk = playerCount - dist.Outsiders - dist.Minions - dist.Demons
		}
		minions = append(minions, extra...)
	}

	// Clamp outsiders to available pool size.
	outsiderPool := byTeam[TeamOutsider]
	if dist.Outsiders > len(outsiderPool) {
		diff := dist.Outsiders - len(outsiderPool)
		dist.Outsiders = len(outsiderPool)
		dist.Townsfolk += diff
	}

	// Round 1b: pick outsiders and townsfolk with adjusted counts. Locked good
	// characters must fit the adjusted counts (e.g. a Baron trades two townsfolk
	// slots for outsider slots — the trade has to come out of unlocked slots).
	outsiders, err := pickRandomLocked(outsiderPool, locked, dist.Outsiders, "outsider")
	if err != nil {
		return nil, wrapPickErr("outsider", err)
	}
	townsfolkPool := byTeam[TeamTownsfolk]
	townsfolk, err := pickRandomLocked(townsfolkPool, locked, dist.Townsfolk, "townsfolk")
	if err != nil {
		return nil, wrapPickErr("townsfolk", err)
	}

	// Re-adjust: check if any selected outsiders/townsfolk have automatic outsider modifiers.
	goodDelta := 0
	for _, c := range outsiders {
		goodDelta += automaticOutsiderDelta(c)
	}
	for _, c := range townsfolk {
		goodDelta += automaticOutsiderDelta(c)
	}

	if goodDelta != 0 {
		if goodDelta > 0 {
			// Swap non-setup townsfolk for outsiders from remaining pool.
			remainingOutsiders := outsiderPool[dist.Outsiders:]
			for goodDelta > 0 && len(remainingOutsiders) > 0 {
				swapped := false
				for i := len(townsfolk) - 1; i >= 0; i-- {
					if automaticOutsiderDelta(townsfolk[i]) == 0 && !locked[townsfolk[i].ID] {
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
				swapped := false
				for i := len(outsiders) - 1; i >= 0; i-- {
					if automaticOutsiderDelta(outsiders[i]) == 0 && !locked[outsiders[i].ID] {
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

	// Build selected set for lookups.
	allSelected := make([]*Character, 0, len(demons)+len(minions)+len(outsiders)+len(townsfolk))
	allSelected = append(allSelected, demons...)
	allSelected = append(allSelected, minions...)
	allSelected = append(allSelected, outsiders...)
	allSelected = append(allSelected, townsfolk...)

	selectedSet := make(map[string]bool, len(allSelected))
	for _, c := range allSelected {
		selectedSet[c.ID] = true
	}

	// Handle companions: auto-add required characters (e.g., Choirboy→King).
	for _, c := range allSelected {
		if !c.Setup {
			continue
		}
		def, known := setupModifiers[c.ID]
		if !known || def.Companion == "" {
			continue
		}
		if selectedSet[def.Companion] {
			continue // Companion already selected.
		}
		companion, onScript := charByID[def.Companion]
		if !onScript {
			setupResult.ManualModifiers = append(setupResult.ManualModifiers, SetupModifier{
				CharacterID:   c.ID,
				CharacterName: c.Name,
				Manual:        true,
				Description:   fmt.Sprintf("%s requires %s, but %s is not on this script", c.Name, def.Companion, def.Companion),
			})
			continue
		}
		// Determine which team slice to replace from based on companion's team.
		var targetSlice *[]*Character
		switch companion.Team {
		case TeamTownsfolk:
			targetSlice = &townsfolk
		case TeamOutsider:
			targetSlice = &outsiders
		}
		if targetSlice == nil {
			// Companion is minion/demon — unlikely, fall through to manual.
			setupResult.ManualModifiers = append(setupResult.ManualModifiers, SetupModifier{
				CharacterID:   c.ID,
				CharacterName: c.Name,
				Manual:        true,
				Description:   fmt.Sprintf("%s requires %s — add manually (no %s slot available to swap)", c.Name, def.Companion, companion.Team),
			})
			continue
		}
		// Replace a non-setup, unlocked character from the matching team with the companion.
		replaced := false
		for i := len(*targetSlice) - 1; i >= 0; i-- {
			if automaticOutsiderDelta((*targetSlice)[i]) == 0 && !hasCompanion((*targetSlice)[i]) && !locked[(*targetSlice)[i].ID] {
				selectedSet[(*targetSlice)[i].ID] = false
				(*targetSlice)[i] = companion
				selectedSet[companion.ID] = true
				replaced = true
				break
			}
		}
		if !replaced {
			setupResult.ManualModifiers = append(setupResult.ManualModifiers, SetupModifier{
				CharacterID:   c.ID,
				CharacterName: c.Name,
				Manual:        true,
				Description:   fmt.Sprintf("%s requires %s — add manually (no %s slot available to swap)", c.Name, def.Companion, companion.Team),
			})
		}
	}

	// Handle bag substitutions: scan all selected characters for bag sub needs,
	// then pick specific characters for the physical bag.
	var bagSubs []BagSubstitution
	for _, c := range allSelected {
		if !c.Setup {
			continue
		}
		def, known := setupModifiers[c.ID]
		if !known || def.BagTeam == "" {
			continue
		}
		sub := BagSubstitution{
			CausedByID:   c.ID,
			CausedByName: c.Name,
			Team:         def.BagTeam,
		}
		if def.BagTeam == TeamTownsfolk {
			// Pick a townsfolk NOT in the selected roles.
			for _, tc := range characters {
				if tc.Team == TeamTownsfolk && !selectedSet[tc.ID] {
					sub.CharacterID = tc.ID
					sub.CharacterName = tc.Name
					selectedSet[tc.ID] = true // Mark as used so we don't pick it again.
					break
				}
			}
		}
		bagSubs = append(bagSubs, sub)
	}

	// Collect all selected IDs.
	var selected []string
	for _, groups := range [][]*Character{demons, minions, outsiders, townsfolk} {
		for _, c := range groups {
			selected = append(selected, c.ID)
		}
	}

	return &RandomizeResult{
		SelectedIDs:      selected,
		BagSubstitutions: bagSubs,
		ManualModifiers:  setupResult.ManualModifiers,
	}, nil
}

// hasCompanion returns true if the character is a required companion for another character.
func hasCompanion(c *Character) bool {
	for _, def := range setupModifiers {
		if def.Companion == c.ID {
			return true
		}
	}
	return false
}

// BagSubstitutionsForRoles returns empty bag substitutions for each selected
// character that requires one (e.g., the Drunk needs a townsfolk token).
// Existing bag subs are preserved if their caused_by_id is still selected.
func BagSubstitutionsForRoles(selectedIDs []string, characters []*Character, existing []BagSubstitution) []BagSubstitution {
	selectedSet := make(map[string]bool, len(selectedIDs))
	for _, id := range selectedIDs {
		selectedSet[id] = true
	}

	// Index existing bag subs by caused_by_id.
	existingByID := make(map[string]BagSubstitution, len(existing))
	for _, bs := range existing {
		existingByID[bs.CausedByID] = bs
	}

	charByID := make(map[string]*Character, len(characters))
	for _, c := range characters {
		charByID[c.ID] = c
	}

	var result []BagSubstitution
	for _, id := range selectedIDs {
		c, ok := charByID[id]
		if !ok || !c.Setup {
			continue
		}
		def, known := setupModifiers[c.ID]
		if !known || def.BagTeam == "" {
			continue
		}
		// Preserve existing bag sub if it exists, otherwise create empty.
		if bs, exists := existingByID[c.ID]; exists {
			result = append(result, bs)
		} else {
			result = append(result, BagSubstitution{
				CausedByID:   c.ID,
				CausedByName: c.Name,
				Team:         def.BagTeam,
			})
		}
	}
	return result
}

// SelectDemonBluffs picks random not-in-play good characters for demon bluffs.
func SelectDemonBluffs(scriptChars []*Character, selectedRoleIds []string, count int) []string {
	selected := make(map[string]bool, len(selectedRoleIds))
	for _, id := range selectedRoleIds {
		selected[id] = true
	}

	var candidates []*Character
	for _, c := range scriptChars {
		if selected[c.ID] {
			continue
		}
		if c.Team == TeamTownsfolk || c.Team == TeamOutsider {
			candidates = append(candidates, c)
		}
	}

	rand.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	n := count
	if n > len(candidates) {
		n = len(candidates)
	}

	bluffs := make([]string, n)
	for i := 0; i < n; i++ {
		bluffs[i] = candidates[i].ID
	}
	return bluffs
}

// ValidateDistribution checks whether a set of characters matches the expected
// distribution for a player count (accounting for setup modifiers).
func ValidateDistribution(characters []*Character, playerCount int) error {
	base, err := DistributionForPlayerCount(playerCount)
	if err != nil {
		return err
	}

	expected := ApplySetupModifiers(base, characters).Distribution

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
