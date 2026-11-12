package main

import "testing"

func TestCapNudge(t *testing.T) {
	for label, want := range map[string]float32{
		"K": -0.5, "C": -0.5, "⌘": -0.5, "⇧": -0.5, "⌥": -0.5, "Space": -0.5, "F5": -0.5,
		"↵": 0.75, "↩": 0.75, "↑": 0.75, "→": 0.75,
		"⌃": 0, "`": 0,
	} {
		if got := capNudge(label); got != want {
			t.Errorf("capNudge(%q) = %v, want %v", label, got, want)
		}
	}
}
