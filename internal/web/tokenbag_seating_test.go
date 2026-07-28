package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeSeating(t *testing.T) {
	tests := []struct {
		name          string
		claims        []seatingClaim
		wantOrder     []int
		wantComplete  bool
		wantConflicts []string
	}{
		{
			name:   "no claims at all",
			claims: nil,
		},
		{
			name: "full cycle of five is complete and starts at the lowest id",
			claims: []seatingClaim{
				{ID: 30, RightID: 50},
				{ID: 50, RightID: 20},
				{ID: 20, RightID: 40},
				{ID: 40, RightID: 10},
				{ID: 10, RightID: 30},
			},
			// #50 is on #30's right, so #30 sits clockwise of #50 (right =
			// counter-clockwise). Following that round: 10 -> 40 -> 20 -> 50 -> 30.
			wantOrder:    []int{10, 40, 20, 50, 30},
			wantComplete: true,
		},
		{
			name: "left and right picks describing the same circle agree",
			claims: []seatingClaim{
				{ID: 1, LeftID: 3, RightID: 2},
				{ID: 2, LeftID: 1, RightID: 3},
				{ID: 3, LeftID: 2, RightID: 1},
			},
			// #1's left is #3, so #3 is clockwise of #1: 1 -> 3 -> 2.
			wantOrder:    []int{1, 3, 2},
			wantComplete: true,
		},
		{
			name: "two players facing each other form a circle of two",
			claims: []seatingClaim{
				{ID: 4, LeftID: 7, RightID: 7},
				{ID: 7},
			},
			wantOrder:    []int{4, 7},
			wantComplete: true,
		},
		{
			// Hand-derived, from the physical table. Players face inward, so a
			// player's LEFT-hand neighbor is the next seat CLOCKWISE seen from
			// above, and their right-hand neighbor is the previous one.
			//
			//   Ana(1) says her left is Bob(2)     => Bob is clockwise of Ana  => 1 -> 2
			//   Bob(2) says his right is Ana(1)    => Ana is counter-clockwise
			//                                         of Bob                  => 1 -> 2 (agrees)
			//   Cara(3) says her left is Ana(1)    => Ana is clockwise of Cara => 3 -> 1
			//   Cara(3) says her right is Bob(2)   => Bob is counter-clockwise
			//                                         of Cara                 => 2 -> 3
			//
			// Clockwise from the lowest id: Ana, Bob, Cara.
			name: "asymmetric three-player table, clockwise from above",
			claims: []seatingClaim{
				{ID: 1, LeftID: 2},
				{ID: 2, RightID: 1},
				{ID: 3, LeftID: 1, RightID: 2},
			},
			wantOrder:    []int{1, 2, 3},
			wantComplete: true,
		},
		{
			name: "consistent chain with a player who has not picked yet",
			claims: []seatingClaim{
				{ID: 1, RightID: 2},
				{ID: 2, RightID: 3},
				{ID: 3},
				{ID: 4},
			},
			wantConflicts: []string{"no neighbor picks yet: #3, #4"},
		},
		{
			name: "nobody picked anything",
			claims: []seatingClaim{
				{ID: 1},
				{ID: 2},
				{ID: 3},
			},
			wantConflicts: []string{"no neighbor picks yet: #1, #2, #3"},
		},
		{
			// Both #1 and #2 say #3 sits on their right, which puts each of them
			// clockwise of #3 — #3 cannot have two.
			name: "two players claim the same player on their right",
			claims: []seatingClaim{
				{ID: 1, RightID: 3},
				{ID: 2, RightID: 3},
				{ID: 3},
			},
			wantConflicts: []string{
				"#3 has two different players sitting clockwise: #1 and #2",
				"no neighbor picks yet: #3",
			},
		},
		{
			// #2 on #1's right puts #2 counter-clockwise of #1; #1 on #3's left
			// puts #3 counter-clockwise of #1 too.
			name: "a player ends up with two different counter-clockwise neighbors",
			claims: []seatingClaim{
				{ID: 1, RightID: 2},
				{ID: 2},
				{ID: 3, LeftID: 1},
			},
			wantConflicts: []string{
				"#1 has two different players sitting counter-clockwise: #2 and #3",
				"no neighbor picks yet: #2",
			},
		},
		{
			name: "left and right picks disagree about who sits between them",
			claims: []seatingClaim{
				{ID: 1, RightID: 2},
				{ID: 2, LeftID: 3},
				{ID: 3},
			},
			wantConflicts: []string{
				"#2 has two different players sitting clockwise: #1 and #3",
				"no neighbor picks yet: #3",
			},
		},
		{
			name: "a sub-circle closes early and leaves the others out",
			claims: []seatingClaim{
				{ID: 1, RightID: 2},
				{ID: 2, RightID: 3},
				{ID: 3, RightID: 1},
				{ID: 4, RightID: 5},
				{ID: 5, RightID: 4},
			},
			wantConflicts: []string{
				"these players form a closed circle that leaves the others out: #1, #3, #2",
				"these players form a closed circle that leaves the others out: #4, #5",
			},
		},
		{
			name: "a pick referencing an unregistered player is reported",
			claims: []seatingClaim{
				{ID: 1, RightID: 99},
				{ID: 2},
			},
			wantConflicts: []string{
				"#1 picked someone who is not registered",
				"no neighbor picks yet: #2",
			},
		},
		{
			name: "a player picking themselves is reported",
			claims: []seatingClaim{
				{ID: 1, RightID: 1},
				{ID: 2},
			},
			wantConflicts: []string{
				"#1 picked themselves as a neighbor",
				"no neighbor picks yet: #2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order, complete, conflicts := computeSeating(tt.claims)
			assert.Equal(t, tt.wantComplete, complete)
			assert.Equal(t, tt.wantOrder, order)
			assert.Equal(t, tt.wantConflicts, conflicts)
		})
	}
}

func TestComputeSeating_IgnoresClaimOrdering(t *testing.T) {
	forward := []seatingClaim{
		{ID: 1, RightID: 2},
		{ID: 2, RightID: 3},
		{ID: 3, RightID: 1},
	}
	shuffled := []seatingClaim{forward[2], forward[0], forward[1]}

	orderA, completeA, conflictsA := computeSeating(forward)
	orderB, completeB, conflictsB := computeSeating(shuffled)

	require.True(t, completeA)
	assert.Equal(t, completeA, completeB)
	assert.Equal(t, orderA, orderB)
	assert.Equal(t, conflictsA, conflictsB)
}

func TestNameSeatingConflicts(t *testing.T) {
	names := map[int]string{1: "Alice", 2: "Bob"}

	got := nameSeatingConflicts([]string{
		"#1 has two different players sitting clockwise: #2 and #7",
		"no neighbor picks yet: #2",
	}, names)

	assert.Equal(t, []string{
		"Alice has two different players sitting clockwise: Bob and an unknown player",
		"no neighbor picks yet: Bob",
	}, got)
	assert.Nil(t, nameSeatingConflicts(nil, names))
}
