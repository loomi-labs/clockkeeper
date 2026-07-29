package botc

import (
	"fmt"
	"slices"
)

// printedNightOrders holds the wake order printed on the physical night sheets
// of the base editions. The global nightsheet.json order is TPI's custom-script
// order and deliberately differs (e.g. in Trouble Brewing the printed sheet
// wakes the Spy right after the Poisoner, while the global order wakes the Spy
// at the end of the night). Base-edition games follow the printed sheet;
// custom scripts keep the global order.
//
// Characters only — the dusk/dawn/minion-info/demon-info steps are rendered by
// the frontend at fixed positions. Each list must contain exactly the edition's
// characters that act that night; NewRegistry fails loudly otherwise, which
// guards against upstream roles/nightsheet data drifting.
var printedNightOrders = map[string]NightSheet{
	"tb": {
		FirstNight: []string{
			"poisoner", "spy", "washerwoman", "librarian", "investigator",
			"chef", "empath", "fortuneteller", "butler",
		},
		OtherNight: []string{
			"poisoner", "monk", "spy", "scarletwoman", "imp",
			"ravenkeeper", "undertaker", "empath", "fortuneteller", "butler",
		},
	},
}

// nightPos is a character's override night positions for one edition.
// A zero value means the character does not act that night.
type nightPos struct {
	first int
	other int
}

// applyPrintedOrders computes per-edition override positions from
// printedNightOrders. Must run after global positions have been assigned.
//
// The overrides are a slot permutation: the edition's characters keep the same
// SET of global positions, reassigned in printed-sheet sequence. Staying inside
// the edition's global slot envelope preserves interleaving with travellers,
// fabled, and the frontend's fixed special-step positions, and survives
// upstream reindexing of nightsheet.json.
func (r *Registry) applyPrintedOrders() error {
	r.editionNightPos = make(map[string]map[string]*nightPos, len(printedNightOrders))
	for edition, printed := range printedNightOrders {
		overrides := make(map[string]*nightPos)
		posFor := func(id string) *nightPos {
			if p, ok := overrides[id]; ok {
				return p
			}
			p := &nightPos{}
			overrides[id] = p
			return p
		}

		firstSlots, err := r.permuteSlots(edition, printed.FirstNight, func(c *Character) int { return c.FirstNight })
		if err != nil {
			return fmt.Errorf("edition %s first night: %w", edition, err)
		}
		for id, slot := range firstSlots {
			posFor(id).first = slot
		}

		otherSlots, err := r.permuteSlots(edition, printed.OtherNight, func(c *Character) int { return c.OtherNight })
		if err != nil {
			return fmt.Errorf("edition %s other night: %w", edition, err)
		}
		for id, slot := range otherSlots {
			posFor(id).other = slot
		}

		r.editionNightPos[edition] = overrides
	}
	return nil
}

// permuteSlots assigns the sorted global slots of the edition's acting
// characters to those characters in printed order. It errors when the printed
// list doesn't exactly match the set of edition characters acting that night.
func (r *Registry) permuteSlots(edition string, printed []string, globalPos func(*Character) int) (map[string]int, error) {
	acting := make(map[string]int) // id -> global slot
	for _, c := range r.byEdition[edition] {
		// Travellers and fabled are not on the printed sheet — they keep
		// their global positions.
		switch c.Team {
		case TeamTownsfolk, TeamOutsider, TeamMinion, TeamDemon:
		default:
			continue
		}
		if p := globalPos(c); p > 0 {
			acting[c.ID] = p
		}
	}

	slots := make([]int, 0, len(printed))
	for _, id := range printed {
		p, ok := acting[id]
		if !ok {
			return nil, fmt.Errorf("printed order lists %q, which is not an acting %s character", id, edition)
		}
		delete(acting, id)
		slots = append(slots, p)
	}
	if len(acting) > 0 {
		for id := range acting {
			return nil, fmt.Errorf("acting character %q is missing from the printed order", id)
		}
	}
	slices.Sort(slots)

	result := make(map[string]int, len(printed))
	for i, id := range printed {
		result[id] = slots[i]
	}
	return result, nil
}

// EditionNightPos returns the printed-sheet night positions for a character
// when the game's script is the given base edition. ok is false when the
// edition has no printed-order override or the character is not part of it —
// callers then keep the character's global positions.
func (r *Registry) EditionNightPos(edition, charID string) (first, other int, ok bool) {
	p, ok := r.editionNightPos[edition][charID]
	if !ok {
		return 0, 0, false
	}
	return p.first, p.other, true
}
