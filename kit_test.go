package main

import (
	"testing"

	"gocast/locales"
)

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

func TestCapsuleWidth(t *testing.T) {
	// Each character 7 wide, whatever its size.
	measure := func(text string, size float32) float32 { return 7 * float32(len([]rune(text))) }
	open := segment{label: "Open", caps: []string{"↩"}}
	actions := segment{label: "Actions", caps: []string{"⌘", "K"}}
	// 6 + 3 + 2 border; Open: 8 + 28 + 8 + 22 + 6; 4; Actions: 8 + 49 +
	// 8 + 22 + 4 + 22 + 6.
	if got, want := capsuleWidthOf(measure, open, actions), float32(11+72+4+119); got != want {
		t.Errorf("capsule width = %v, want %v", got, want)
	}
	// A key wider than 22: its text, 5 and its border at both sides.
	if got, want := capsuleWidthOf(measure, segment{label: "", caps: []string{"Enter"}}), float32(11+8+8+47+6); got != want {
		t.Errorf("capsule width with a wide key = %v, want %v", got, want)
	}
	// The notice takes what the actions capsule leaves (design.md 2.4):
	// about 370 in Chinese, 300 in English at 640.
	if got := noticeRoom(panelWidth, 316); got != 300 {
		t.Errorf("noticeRoom = %v, want 300", got)
	}
}

func TestListRowWidths(t *testing.T) {
	// design.md 2.3: the subtitle at most 40% of the 624 wide row, about
	// 250; the title still 160 with it, beside the widest tag.
	if listNoteMax != 249.6 {
		t.Errorf("listNoteMax = %v, want 249.6", float32(listNoteMax))
	}
	const tag = 60 // "Preset" is about 36
	room := float32(panelWidth-2*listInset) - 10 - 12 - iconCell - 10 - 8 - 10 - 10 - tag
	if room-listNoteMax < listTitleLeast {
		t.Errorf("the title has %v beside the widest subtitle, under %v", room-listNoteMax, float32(listTitleLeast))
	}
}

func TestAround(t *testing.T) {
	for _, c := range []struct{ s, before, after string }{
		{"没有找到“" + queryMark + "”", "没有找到“", "”"},
		{"No results for “" + queryMark + "”", "No results for “", "”"},
		{"nothing", "nothing", ""},
	} {
		if b, a := around(c.s, queryMark); b != c.before || a != c.after {
			t.Errorf("around(%q) = %q, %q", c.s, b, a)
		}
	}
	// The catalog's line keeps the query in it, in both languages.
	for _, loc := range []string{"zh-Hans", "en"} {
		if b, a := around(locales.In(loc).SearchNoResults(queryMark), queryMark); b == "" || a == "" {
			t.Errorf("%s: no-results line %q · %q", loc, b, a)
		}
	}
}
