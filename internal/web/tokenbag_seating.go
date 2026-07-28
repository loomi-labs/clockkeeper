package web

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// seatingClaim is one player's neighbor picks. A zero LeftID/RightID means the
// player has not picked that side yet.
type seatingClaim struct {
	ID      int
	LeftID  int
	RightID int
}

// seatingIDRef marks a registration id inside a conflict string so callers can
// swap ids for display names — computeSeating is pure and knows nothing about
// names. Every id emitted in a conflict string is written as "#<id>".
var seatingIDRef = regexp.MustCompile(`#(\d+)`)

// computeSeating derives the clockwise seating order from the players' neighbor
// picks.
//
// ORIENTATION INVARIANT — the returned order is the seating CLOCKWISE AS SEEN
// FROM ABOVE the table. Players sit in a circle facing inward, so a player's
// LEFT-hand neighbor is the next seat clockwise from above, and their RIGHT-hand
// neighbor is the previous one. (Stand in the circle facing the middle: your
// left hand points the way the clock hands sweep when the table is drawn from
// above.) Get this backwards and the arranged grimoire is a mirror image of the
// real table.
//
// Every pick therefore contributes one directed edge "successor = the player
// sitting clockwise": r.LeftID = a yields r->a, r.RightID = b yields b->r. Two
// players agreeing on the same edge contribute it once.
//
// order covers ALL claims whenever the picks hold no contradiction, and complete
// says how much it is worth:
//
//   - complete: the edges form a single Hamiltonian cycle, so the order IS the
//     table. It starts at the lowest registration id and follows successor
//     (clockwise) edges.
//   - not complete: the picks only describe disjoint chains of neighbors (plus
//     players who picked nobody), and the order is a BEST EFFORT — every chain is
//     a correct run of seats, but where one chain ends and the next begins is a
//     guess for the storyteller to fix by dragging. Deterministic, so an arranged
//     grimoire does not reshuffle itself between polls: chains ordered by their
//     smallest registration id, then the players with no picks by id. See
//     chainOrder.
//
// conflicts lists contradictions only — picks that cannot all be true at one
// table: two players clockwise (or counter-clockwise) of the same seat, a pick on
// someone unregistered, a player picking themselves, a circle that closes before
// covering everyone. Players who have not picked yet are not a conflict. When
// there is a contradiction NO order is returned at all: an order built from picks
// that disagree would place players wrongly without saying so. Registration ids
// appear as "#<id>" — see seatingIDRef.
func computeSeating(claims []seatingClaim) (order []int, complete bool, conflicts []string) {
	if len(claims) == 0 {
		return nil, false, nil
	}

	// Work on an id-sorted copy so both the edge set and the conflict list are
	// deterministic regardless of the caller's ordering.
	sorted := make([]seatingClaim, len(claims))
	copy(sorted, claims)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	ids := make([]int, 0, len(sorted))
	known := make(map[int]bool, len(sorted))
	for _, c := range sorted {
		ids = append(ids, c.ID)
		known[c.ID] = true
	}

	succ := make(map[int]int, len(sorted)) // player -> player sitting clockwise
	pred := make(map[int]int, len(sorted)) // player -> player sitting counter-clockwise
	reportedSucc := make(map[int]bool)
	reportedPred := make(map[int]bool)
	reportedUnknown := make(map[int]bool)
	reportedSelf := make(map[int]bool)

	// addEdge records "to sits clockwise of from", claimed by owner.
	addEdge := func(owner, from, to int) {
		if !known[from] || !known[to] {
			if !reportedUnknown[owner] {
				conflicts = append(conflicts, fmt.Sprintf("#%d picked someone who is not registered", owner))
				reportedUnknown[owner] = true
			}
			return
		}
		if from == to {
			if !reportedSelf[owner] {
				conflicts = append(conflicts, fmt.Sprintf("#%d picked themselves as a neighbor", owner))
				reportedSelf[owner] = true
			}
			return
		}
		if existing, ok := succ[from]; ok && existing != to {
			if !reportedSucc[from] {
				conflicts = append(conflicts, fmt.Sprintf("#%d has two different players sitting clockwise: #%d and #%d", from, existing, to))
				reportedSucc[from] = true
			}
			return
		}
		if existing, ok := pred[to]; ok && existing != from {
			if !reportedPred[to] {
				conflicts = append(conflicts, fmt.Sprintf("#%d has two different players sitting counter-clockwise: #%d and #%d", to, existing, from))
				reportedPred[to] = true
			}
			return
		}
		succ[from] = to
		pred[to] = from
	}

	for _, c := range sorted {
		// See the orientation invariant above: left = next clockwise, right =
		// previous clockwise.
		if c.RightID != 0 {
			addEdge(c.ID, c.RightID, c.ID)
		}
		if c.LeftID != 0 {
			addEdge(c.ID, c.ID, c.LeftID)
		}
	}

	// Walk the successor graph and collect its cycles. succ is injective (pred
	// enforces a single predecessor), so every node belongs to at most one cycle.
	visited := make(map[int]bool, len(sorted))
	var cycles [][]int
	for _, id := range ids {
		if visited[id] {
			continue
		}
		var path []int
		pos := make(map[int]int)
		n := id
		for {
			if visited[n] {
				break
			}
			if p, ok := pos[n]; ok {
				cycles = append(cycles, path[p:])
				break
			}
			pos[n] = len(path)
			path = append(path, n)
			next, ok := succ[n]
			if !ok {
				break
			}
			n = next
		}
		for _, p := range path {
			visited[p] = true
		}
	}

	// A circle that closes before covering everyone cannot be a piece of one
	// table: some pick in it is wrong.
	for _, c := range cycles {
		if len(c) < len(sorted) {
			conflicts = append(conflicts, fmt.Sprintf("these players form a closed circle that leaves the others out: %s", joinSeatingIDs(rotateToLowest(c))))
		}
	}
	if len(conflicts) > 0 {
		return nil, false, conflicts
	}

	if len(cycles) == 1 {
		// The short circles were just refused, so this one covers everyone — and
		// a cycle over all players leaves room for no other.
		return rotateToLowest(cycles[0]), true, nil
	}

	return chainOrder(ids, succ, pred), false, nil
}

// chainOrder builds the best-effort seating out of picks that agree but leave the
// circle open: each maximal chain of consecutive seats in clockwise order, chains
// ordered by their smallest registration id, then the players nobody's picks
// placed, by id.
//
// ids must be ascending, and succ must hold no cycle — computeSeating refuses
// cycles that leave players out before it gets here, so a chain can never run
// into one.
func chainOrder(ids []int, succ, pred map[int]int) []int {
	var chains [][]int
	var unplaced []int
	for _, id := range ids {
		if _, hasPred := pred[id]; hasPred {
			continue // sits inside a chain, reached by walking it from the start
		}
		if _, hasSucc := succ[id]; !hasSucc {
			unplaced = append(unplaced, id)
			continue
		}

		chain := []int{id}
		for n := id; ; {
			next, ok := succ[n]
			if !ok {
				break
			}
			chain = append(chain, next)
			n = next
		}
		chains = append(chains, chain)
	}

	// By smallest member, not by the first seat: which end of a chain comes first
	// is just where its picks happened to start.
	sort.Slice(chains, func(i, j int) bool { return slices.Min(chains[i]) < slices.Min(chains[j]) })

	order := make([]int, 0, len(ids))
	for _, c := range chains {
		order = append(order, c...)
	}
	return append(order, unplaced...)
}

// rotateToLowest rotates a cycle so it starts at its lowest id, keeping the
// successor order intact.
func rotateToLowest(cycle []int) []int {
	start := 0
	for i, id := range cycle {
		if id < cycle[start] {
			start = i
		}
	}
	out := make([]int, 0, len(cycle))
	out = append(out, cycle[start:]...)
	out = append(out, cycle[:start]...)
	return out
}

func joinSeatingIDs(ids []int) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = "#" + strconv.Itoa(id)
	}
	return strings.Join(parts, ", ")
}

// nameSeatingConflicts replaces the "#<id>" references produced by
// computeSeating with the players' display names.
func nameSeatingConflicts(conflicts []string, names map[int]string) []string {
	if len(conflicts) == 0 {
		return nil
	}
	out := make([]string, len(conflicts))
	for i, c := range conflicts {
		out[i] = seatingIDRef.ReplaceAllStringFunc(c, func(ref string) string {
			id, err := strconv.Atoi(strings.TrimPrefix(ref, "#"))
			if err != nil {
				return ref
			}
			if name, ok := names[id]; ok {
				return name
			}
			return "an unknown player"
		})
	}
	return out
}
