package main

import (
	"math"
	"testing"
)

func TestCubicBezier(t *testing.T) {
	for _, ease := range []func(float64) float64{easeIn, easeOut} {
		if ease(0) != 0 || ease(1) != 1 {
			t.Fatalf("ends: %v, %v", ease(0), ease(1))
		}
		prev := 0.0
		for i := 1; i <= 100; i++ {
			v := ease(float64(i) / 100)
			if v < prev-1e-9 {
				t.Fatalf("not monotonic at %d: %v < %v", i, v, prev)
			}
			prev = v
		}
	}
	// Ease-out is ahead of linear time early, ease-in behind it.
	if easeOut(0.25) <= 0.25 || easeIn(0.25) >= 0.25 {
		t.Fatalf("easeOut(.25) = %v, easeIn(.25) = %v", easeOut(0.25), easeIn(0.25))
	}
	// The linear curve is the identity.
	if v := cubicBezier(1.0/3, 1.0/3, 2.0/3, 2.0/3, 0.4); math.Abs(v-0.4) > 1e-6 {
		t.Fatalf("linear(0.4) = %v", v)
	}
}
