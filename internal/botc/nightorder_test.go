package botc

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sortedByNight returns the given character IDs sorted by their effective
// night position for the edition (override when present, global otherwise).
func sortedByNight(t *testing.T, r *Registry, edition string, ids []string, first bool) []string {
	t.Helper()
	pos := make(map[string]int, len(ids))
	for _, id := range ids {
		c, ok := r.Character(id)
		require.True(t, ok, "character %s not found", id)
		p := c.OtherNight
		if first {
			p = c.FirstNight
		}
		if f, o, ok := r.EditionNightPos(edition, id); ok {
			p = o
			if first {
				p = f
			}
		}
		require.Positive(t, p, "character %s has no position", id)
		pos[id] = p
	}
	sorted := slices.Clone(ids)
	slices.SortFunc(sorted, func(a, b string) int { return pos[a] - pos[b] })
	return sorted
}

// TestEditionNightPos_TroubleBrewing asserts the wake order of a pure Trouble
// Brewing game matches the printed night sheet from the physical box.
func TestEditionNightPos_TroubleBrewing(t *testing.T) {
	r := newTestRegistry(t)

	first := []string{
		"poisoner", "spy", "washerwoman", "librarian", "investigator",
		"chef", "empath", "fortuneteller", "butler",
	}
	got := sortedByNight(t, r, "tb", slices.Clone(first), true)
	assert.Equal(t, first, got, "first night order")

	other := []string{
		"poisoner", "monk", "spy", "scarletwoman", "imp",
		"ravenkeeper", "undertaker", "empath", "fortuneteller", "butler",
	}
	got = sortedByNight(t, r, "tb", slices.Clone(other), false)
	assert.Equal(t, other, got, "other nights order")
}

// TestEditionNightPos_SlotEnvelope asserts overrides reuse the edition's own
// global slots, so travellers/fabled and the frontend's fixed special-step
// positions keep interleaving exactly as with the global order.
func TestEditionNightPos_SlotEnvelope(t *testing.T) {
	r := newTestRegistry(t)

	for _, night := range []bool{true, false} {
		var globalSlots, overrideSlots []int
		for _, c := range r.CharactersByEdition("tb") {
			if c.Team == TeamTraveller || c.Team == TeamFabled {
				continue
			}
			global := c.OtherNight
			if night {
				global = c.FirstNight
			}
			if global == 0 {
				continue
			}
			globalSlots = append(globalSlots, global)
			f, o, ok := r.EditionNightPos("tb", c.ID)
			require.True(t, ok, "expected override for %s", c.ID)
			if night {
				overrideSlots = append(overrideSlots, f)
			} else {
				overrideSlots = append(overrideSlots, o)
			}
		}
		slices.Sort(globalSlots)
		slices.Sort(overrideSlots)
		assert.Equal(t, globalSlots, overrideSlots)

		// The frontend renders the Minion Info / Demon Info steps at fixed
		// positions 20 and 25 (NightOrder.svelte SPECIAL_ENTRIES). On the
		// first night every TB character must wake after them.
		if night {
			for _, slot := range overrideSlots {
				assert.Greater(t, slot, 25, "first-night slot collides with the fixed info-step positions")
			}
		}
	}
}

func TestEditionNightPos_NoOverride(t *testing.T) {
	r := newTestRegistry(t)

	// Custom scripts (empty edition) and characters outside the edition have
	// no override.
	_, _, ok := r.EditionNightPos("", "spy")
	assert.False(t, ok)
	_, _, ok = r.EditionNightPos("tb", "monk")
	assert.True(t, ok)
	_, _, ok = r.EditionNightPos("tb", "sailor") // BMR character
	assert.False(t, ok)
	// TB characters without night actions have no override entry.
	_, _, ok = r.EditionNightPos("tb", "virgin")
	assert.False(t, ok)
}
