package botc

import (
	"encoding/json"
	"fmt"
	"slices"
)

// Registry holds all Blood on the Clocktower game data in memory.
type Registry struct {
	characters map[string]*Character
	byEdition  map[string][]*Character
	byTeam     map[Team][]*Character
	jinxes     map[string][]Jinx
	nightSheet *NightSheet
}

// NewRegistry creates a registry from the raw JSON data files.
func NewRegistry(rolesJSON, jinxesJSON, nightSheetJSON []byte) (*Registry, error) {
	var characters []*Character
	if err := json.Unmarshal(rolesJSON, &characters); err != nil {
		return nil, fmt.Errorf("parsing roles: %w", err)
	}

	var charJinxes []CharacterJinxes
	if err := json.Unmarshal(jinxesJSON, &charJinxes); err != nil {
		return nil, fmt.Errorf("parsing jinxes: %w", err)
	}

	var nightSheet NightSheet
	if err := json.Unmarshal(nightSheetJSON, &nightSheet); err != nil {
		return nil, fmt.Errorf("parsing night sheet: %w", err)
	}

	r := &Registry{
		characters: make(map[string]*Character, len(characters)),
		byEdition:  make(map[string][]*Character),
		byTeam:     make(map[Team][]*Character),
		jinxes:     make(map[string][]Jinx),
		nightSheet: &nightSheet,
	}

	for _, c := range characters {
		r.characters[c.ID] = c
		r.byEdition[c.Edition] = append(r.byEdition[c.Edition], c)
		r.byTeam[c.Team] = append(r.byTeam[c.Team], c)
	}

	for _, cj := range charJinxes {
		r.jinxes[cj.ID] = cj.Jinx
	}

	return r, nil
}

// Character returns a character by ID.
func (r *Registry) Character(id string) (*Character, bool) {
	c, ok := r.characters[id]
	return c, ok
}

// Characters returns all characters matching the given IDs.
// Unknown IDs are silently skipped.
func (r *Registry) Characters(ids []string) []*Character {
	result := make([]*Character, 0, len(ids))
	for _, id := range ids {
		if c, ok := r.characters[id]; ok {
			result = append(result, c)
		}
	}
	return result
}

// AllCharacters returns all characters sorted by name.
func (r *Registry) AllCharacters() []*Character {
	result := make([]*Character, 0, len(r.characters))
	for _, c := range r.characters {
		result = append(result, c)
	}
	slices.SortFunc(result, func(a, b *Character) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})
	return result
}

// CharactersByEdition returns characters belonging to a specific edition.
func (r *Registry) CharactersByEdition(edition string) []*Character {
	return r.byEdition[edition]
}

// CharactersByTeam returns characters belonging to a specific team.
func (r *Registry) CharactersByTeam(team Team) []*Character {
	return r.byTeam[team]
}

// Editions returns the available edition templates with their characters.
// Only includes the three official editions (TB, BMR, SNV).
func (r *Registry) Editions() []Edition {
	editions := make([]Edition, 0, len(KnownEditions))
	for _, e := range KnownEditions {
		chars := r.byEdition[e.ID]
		if len(chars) == 0 {
			continue
		}
		editions = append(editions, Edition{
			ID:         e.ID,
			Name:       e.Name,
			Characters: chars,
		})
	}
	return editions
}

// Jinxes returns all jinxes for a character.
func (r *Registry) Jinxes(id string) []Jinx {
	return r.jinxes[id]
}

// JinxesBetween returns only jinxes that are relevant between the given character IDs.
func (r *Registry) JinxesBetween(ids []string) []Jinx {
	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}

	var result []Jinx
	for _, id := range ids {
		for _, j := range r.jinxes[id] {
			if idSet[j.ID] {
				result = append(result, Jinx{
					ID:     id + "/" + j.ID,
					Reason: j.Reason,
				})
			}
		}
	}
	return result
}

// NightOrder returns the night sheet.
func (r *Registry) NightOrder() *NightSheet {
	return r.nightSheet
}
