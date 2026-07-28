package web

import (
	"fmt"
	"regexp"
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
// Every pick contributes one directed edge "successor = the player sitting
// clockwise": r.RightID = b yields r->b, r.LeftID = a yields a->r. Two players
// agreeing on the same edge contribute it once.
//
// order is returned only when the edges form a single Hamiltonian cycle over
// ALL claims — a partial order is never guessed. In that case order starts at
// the lowest registration id and follows successor (clockwise) edges.
//
// conflicts describes everything that blocks a complete circle: contradictory
// picks, picks referencing unknown players, loops that close before covering
// everyone, and (when the circle is incomplete) the players who have not picked
// yet. Registration ids appear as "#<id>" — see seatingIDRef.
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
		if c.RightID != 0 {
			addEdge(c.ID, c.ID, c.RightID)
		}
		if c.LeftID != 0 {
			addEdge(c.ID, c.LeftID, c.ID)
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

	if len(conflicts) == 0 && len(cycles) == 1 && len(cycles[0]) == len(sorted) {
		return rotateToLowest(cycles[0]), true, nil
	}

	// Incomplete: report loops that close before covering everyone, then the
	// players who have not picked a neighbor at all.
	for _, c := range cycles {
		if len(c) < len(sorted) {
			conflicts = append(conflicts, fmt.Sprintf("these players form a closed circle that leaves the others out: %s", joinSeatingIDs(rotateToLowest(c))))
		}
	}
	var gaps []int
	for _, c := range sorted {
		if c.LeftID == 0 && c.RightID == 0 {
			gaps = append(gaps, c.ID)
		}
	}
	if len(gaps) > 0 {
		conflicts = append(conflicts, fmt.Sprintf("no neighbor picks yet: %s", joinSeatingIDs(gaps)))
	}

	return nil, false, conflicts
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
