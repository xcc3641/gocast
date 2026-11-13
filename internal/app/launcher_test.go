package app

import "testing"

func TestCapsuleBottom(t *testing.T) {
	// design.md 2.4: centred in the search bar alone, 15 above and below
	// from the panel's edges; 8 from the bottom once the panel shows more,
	// reached without a jump. capsuleBottom counts from within the border.
	for _, c := range []struct{ h, want float32 }{
		{barHeight, 14.5},
		{barHeight + 3, 11.5},
		{barHeight + 6.5, capsuleInset},
		{barHeight + 7, capsuleInset},
		{noResultHeight, capsuleInset},
		{expandedHeight, capsuleInset},
	} {
		if got := capsuleBottom(c.h); got != c.want {
			t.Errorf("capsuleBottom(%v) = %v, want %v", c.h, got, c.want)
		}
	}
	below := capsuleBottom(barHeight) + panelBorderWidth
	above := barHeight - below - capsuleHeight
	if above != 15 || below != 15 {
		t.Errorf("in the bar alone the capsule has %v above and %v below, not 15", above, below)
	}
}
