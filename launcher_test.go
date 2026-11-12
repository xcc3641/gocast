package main

import (
	"slices"
	"strings"
	"testing"

	"gocast/internal/presets"
	"gocast/locales"
)

// rows spells Gocast's own actions as they read: "[title]" over the row
// beginning a section, "---" for a divider, then each row's label, note
// and keys.
func rows(as []ownAction) []string {
	var out []string
	for _, a := range as {
		if a.divided {
			out = append(out, "---")
		}
		if a.section != "" {
			out = append(out, "["+a.section+"]")
		}
		row := a.label
		if a.note != "" {
			row += " · " + a.note
		}
		if len(a.caps) > 0 {
			row += " " + strings.Join(a.caps, "")
		}
		out = append(out, row)
	}
	return out
}

func TestGocastActions(t *testing.T) {
	ps := []presets.Preset{
		{ID: "a", Name: "Shell", Dir: "~"},
		{ID: "b", Name: "pi", Dir: "~/Work/my-app", Command: "pi"},
	}
	var did []string
	do := gocastDo{
		terminal:  func() { did = append(did, "terminal") },
		preset:    func(id string) { did = append(did, "preset "+id) },
		addPreset: func() { did = append(did, "add") },
		settings:  func() { did = append(did, "settings") },
		quit:      func() { did = append(did, "quit") },
	}
	as := gocastActions(ps, "Ctrl+`", do)
	want := []string{
		locales.MenuTerminal() + " ⌃`",
		"[" + locales.MenuWithPreset() + "]",
		presetTitle(ps[0]) + " · ~",
		presetTitle(ps[1]) + " · my-app",
		locales.PresetsAdd(),
		"---",
		locales.TraySettings() + " ⌘,",
		locales.TrayQuit(),
	}
	if got := rows(as); !slices.Equal(got, want) {
		t.Errorf("rows:\n got %q\nwant %q", got, want)
	}
	for _, a := range as {
		a.run()
	}
	if want := []string{"terminal", "preset a", "preset b", "add", "settings", "quit"}; !slices.Equal(did, want) {
		t.Errorf("ran %q, want %q", did, want)
	}
	// Presets read as in the results; the other rows, their names.
	if got, want := as[2].label2, presetLabel(ps[1]); got != want || !strings.Contains(got, "~/Work/my-app") {
		t.Errorf("preset row reads %q, want %q", got, want)
	}
	if as[0].label2 != "" || as[0].symbol == "" || as[1].symbol != "" {
		t.Errorf("terminal row %+v, preset row %+v", as[0], as[1])
	}

	// No presets: no section, straight to Add Preset; the terminal's
	// shortcut as the user set it.
	as = gocastActions(nil, "Cmd+Alt+T", do)
	want = []string{locales.MenuTerminal() + " ⌥⌘T", locales.PresetsAdd(), "---", locales.TraySettings() + " ⌘,", locales.TrayQuit()}
	if got := rows(as); !slices.Equal(got, want) {
		t.Errorf("rows without presets:\n got %q\nwant %q", got, want)
	}
	// No shortcut registered: no keys.
	if as := gocastActions(nil, "", do); len(as[0].caps) != 0 {
		t.Errorf("caps without a shortcut: %q", as[0].caps)
	}
}

func TestGocastActionsCopy(t *testing.T) {
	// copy.md v1.7, 2.2 and 2.3.
	for _, c := range []struct{ locale, terminal, section, add, settings, quit, list, heading string }{
		{"zh-Hans", "打开悬浮终端", "用预设打开", "添加预设", "设置…", "退出 Gocast", "Gocast 操作", "用预设打开，标题"},
		{"en", "Open Floating Terminal", "Open with Preset", "Add Preset", "Settings…", "Quit Gocast", "Gocast Actions", "Open with Preset, heading"},
	} {
		m := locales.In(c.locale)
		got := []string{m.MenuTerminal(), m.MenuWithPreset(), m.PresetsAdd(), m.TraySettings(), m.TrayQuit(), m.MenuList(), m.MenuWithPresetHeading()}
		if want := []string{c.terminal, c.section, c.add, c.settings, c.quit, c.list, c.heading}; !slices.Equal(got, want) {
			t.Errorf("%s: %q, want %q", c.locale, got, want)
		}
	}
}

func TestOwnListHeight(t *testing.T) {
	// design.md 2.5a: the panel 351 high with 2 presets, 243 with none,
	// 475 at most.
	ps := []presets.Preset{{ID: "a", Name: "Shell", Dir: "~"}, {ID: "b", Name: "pi", Dir: "~", Command: "pi"}}
	for _, c := range []struct {
		n    int
		want float32
	}{{2, 351}, {0, 243}, {20, expandedHeight}} {
		many := ps
		if c.n > 2 {
			many = make([]presets.Preset, c.n)
		}
		if got := barHeight + ownListHeight(gocastActions(many[:c.n], "Ctrl+`", gocastDo{})); got != c.want {
			t.Errorf("%d presets: panel %v high, want %v", c.n, got, c.want)
		}
	}
}

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
