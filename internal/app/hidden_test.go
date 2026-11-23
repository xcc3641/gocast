package app

import (
	"image"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gocast/internal/keepawake"
	"gocast/internal/settings"
	"gocast/locales"
)

// withSystemApps adds Calendar and Chess, of /System/Applications, to the
// search panel's apps, named in Chinese with zh; with more, Maps and
// Photos too.
func withSystemApps(l *launcher, zh, more bool) {
	apps := []mygo.InstalledApp{
		{Name: "Calendar", ID: "com.apple.iCal", Path: "/System/Applications/Calendar.app"},
		{Name: "Chess", ID: "com.apple.Chess", Path: "/System/Applications/Chess.app"},
	}
	if more {
		apps = append(apps,
			mygo.InstalledApp{Name: "Maps", ID: "com.apple.Maps", Path: "/System/Applications/Maps.app"},
			mygo.InstalledApp{Name: "Photos", ID: "com.apple.Photos", Path: "/System/Applications/Photos.app"})
	}
	cn := map[string]string{"Calendar": "日历", "Chess": "国际象棋", "Maps": "地图", "Photos": "照片"}
	for _, a := range apps {
		if zh {
			a.Name = cn[a.Name]
		}
		l.apps = append(l.apps, a)
		l.alts = append(l.alts, nil)
		l.places = append(l.places, "")
		l.icons[a.Path] = nil
	}
	l.rebuild()
}

// found reports whether typing q finds the item with the ID.
func found(l *launcher, q, id string) bool {
	typeQuery(l, q)
	return slices.Contains(rowIDs(l), id)
}

// An item hidden is not found by typing, nor suggested, at once; shown
// again, it is back at once, as the next key typed.
func TestSearchLeavesOutHidden(t *testing.T) {
	l, _, _ := hotkeyPanel(t, "en")
	g := l.g
	const safari = "app:com.apple.Safari"
	typeQuery(l, "")
	if !slices.Contains(rowIDs(l), safari) {
		t.Fatalf("not suggested before: %q", shown(l))
	}
	g.setSearchHidden(safari, settings.GroupOtherApps, true)
	if found(l, "saf", safari) || found(l, "safari", safari) {
		t.Errorf("Safari hidden, found: %q", shown(l))
	}
	typeQuery(l, "")
	if slices.Contains(rowIDs(l), safari) {
		t.Errorf("Safari hidden, suggested: %q", shown(l))
	}
	if !slices.Contains(rowIDs(l), "app:com.apple.finder") {
		t.Errorf("the next suggestion did not come up: %q", shown(l))
	}
	g.setSearchHidden(safari, settings.GroupOtherApps, false)
	if !found(l, "saf", safari) {
		t.Errorf("Safari shown again, not found: %q", shown(l))
	}
	// Kept in settings.json.
	if s, err := settings.Load(g.settingsPath()); err != nil || len(s.HiddenItems) != 0 {
		t.Errorf("saved: %v, %v", s.HiddenItems, err)
	}
}

// System Apps hidden whole, Calendar shown alone: only Calendar of them
// is found; the group shown again, all but Chess (hidden before) are,
// Calendar too (design.md 4.4).
func TestSearchHiddenGroup(t *testing.T) {
	l, _, _ := hotkeyPanel(t, "en")
	withSystemApps(l, false, false)
	g := l.g
	const (
		cal   = "app:com.apple.iCal"
		chess = "app:com.apple.Chess"
		notes = "app:com.apple.Notes"
		term  = "app:com.apple.Terminal" // in Utilities
	)
	g.setSearchHidden(chess, settings.GroupSystemApps, true)
	g.setGroupHidden(settings.GroupSystemApps, true)
	g.setSearchHidden(cal, settings.GroupSystemApps, false)
	for q, id := range map[string]string{"notes": notes, "terminal": term, "chess": chess} {
		if found(l, q, id) {
			t.Errorf("group off: %s found", id)
		}
	}
	if !found(l, "calendar", cal) || !found(l, "safari", "app:com.apple.Safari") {
		t.Errorf("group off: Calendar or Safari not found")
	}
	typeQuery(l, "")
	if ids := rowIDs(l); slices.Contains(ids, notes) || slices.Contains(ids, term) {
		t.Errorf("group off: suggested %q", ids)
	}

	g.setGroupHidden(settings.GroupSystemApps, false)
	for q, id := range map[string]string{"notes": notes, "terminal": term, "calendar": cal} {
		if !found(l, q, id) {
			t.Errorf("group on: %s not found", id)
		}
	}
	if found(l, "chess", chess) {
		t.Error("group on: Chess found")
	}
}

// Gocast's own items hidden: not found, nor in the Gocast group; Keep
// Awake hidden: not brought in by "awake", its lengths still found.
func TestSearchHiddenOwn(t *testing.T) {
	l, _, _ := awakePanel(t, "en")
	g := l.g
	g.setSearchHidden(clipItemID, settings.GroupGocast, true)
	if found(l, "clipboard", clipItemID) {
		t.Errorf("clipboard history hidden, found: %q", shown(l))
	}
	typeQuery(l, "")
	if slices.Contains(shown(l), "group:"+clipItemID) || !slices.Contains(shown(l), "group:"+termItemID) {
		t.Errorf("Gocast group: %q", shown(l))
	}
	// All of the Gocast group hidden: no title over none.
	g.setGroupHidden(settings.GroupGocast, true)
	typeQuery(l, "")
	if slices.ContainsFunc(shown(l), func(s string) bool {
		return strings.HasPrefix(s, "group:") || s == "title:"+locales.SearchGocastGroup()
	}) {
		t.Errorf("Gocast hidden whole: %q", shown(l))
	}
	g.setGroupHidden(settings.GroupGocast, false)

	g.setSearchHidden(awakeToggleID, settings.GroupSystem, true)
	for _, q := range []string{"keep awake", "awake", "caffeinate"} {
		if found(l, q, awakeToggleID) {
			t.Errorf("Keep Awake hidden, found by %q: %q", q, shown(l))
		}
	}
	if !found(l, "keep awake", awakePrefix+"30") {
		t.Errorf("Keep Awake for 30 Minutes not found: %q", shown(l))
	}
}

// Hidden items keep their hotkeys: an app's, the floating terminal's,
// registered still, and run as before.
func TestHiddenKeepsHotkeys(t *testing.T) {
	l, _, f := hotkeyPanel(t, "en")
	g := l.g
	term := settings.Hotkey{App: "com.apple.Terminal"}
	g.setSearchHidden("app:com.apple.Terminal", settings.GroupSystemApps, true)
	g.setSearchHidden(termItemID, settings.GroupGocast, true)
	g.setGroupHidden(settings.GroupSystemApps, true)
	if !f.on["Alt+Cmd+T"] || !f.on["Ctrl+`"] {
		t.Errorf("registered: %v", f.on)
	}
	if st, ok := g.hotkeyStatus(term); !ok || st != hotkeyOn {
		t.Errorf("Terminal's hotkey: %v, %v", st, ok)
	}
	// Set while hidden: registered too.
	if err := g.setHotkey(settings.Hotkey{App: "com.apple.Notes"}, "Ctrl+Alt+N"); err != nil || !f.on["Ctrl+Alt+N"] {
		t.Errorf("Notes' hotkey set while hidden: %v", err)
	}
}

// Keep Awake's hotkey starts it until stopped, and stops it whatever runs;
// a length's starts it for that long, in place of what runs (design.md
// 4.4).
func TestAwakeHotkeys(t *testing.T) {
	l, _, fk := awakePanel(t, "en")
	g := l.g
	toggle := settings.Hotkey{Awake: settings.AwakeToggle}
	hour := settings.Hotkey{Awake: "60"}
	if err := g.setHotkey(toggle, "Ctrl+Alt+K"); err != nil {
		t.Fatal(err)
	}
	if err := g.setHotkey(hour, "Ctrl+Alt+H"); err != nil {
		t.Fatal(err)
	}
	// Hidden in the search: the hotkey works as before.
	g.setSearchHidden(awakeToggleID, settings.GroupSystem, true)
	press := func(h settings.Hotkey) { g.hotkeyFunc(h)() }

	press(toggle)
	if s := g.awake.State(); s.Kind != keepawake.Forever || len(fk.as.held) != 1 {
		t.Fatalf("toggle: %v, held %v", s.Kind, fk.as.held)
	}
	press(toggle)
	if g.awakeActive() || len(fk.as.held) != 0 {
		t.Fatalf("toggle again: still on")
	}
	press(hour)
	if s := g.awake.State(); s.Kind != keepawake.For || s.Deadline.Sub(*fk.now) != time.Hour {
		t.Fatalf("1 hour: %v until %v", s.Kind, s.Deadline)
	}
	fk.advance(g, 10*time.Minute)
	press(hour) // again: from now
	if s := g.awake.State(); s.Deadline.Sub(*fk.now) != time.Hour || len(fk.as.held) != 1 {
		t.Errorf("1 hour again: until %v, held %v", s.Deadline, fk.as.held)
	}
	press(toggle) // stops a length too
	if g.awakeActive() {
		t.Error("toggle did not stop the length")
	}
	if l.shown && l.notice != "" {
		t.Errorf("a hotkey said %q", l.notice)
	}
}

// Hide from Search, last in the Cmd-K menu: the row goes, the next one
// chosen in its place, the panel staying with the notice for 6 seconds;
// none for the row of a length typed after Keep Awake (design.md 2.5).
func TestHideFromSearchMenu(t *testing.T) {
	l, _, _ := awakePanel(t, "en")
	l.shown = true
	typeQuery(l, "s")
	ids := rowIDs(l)
	at := slices.Index(ids, "app:com.apple.Safari")
	if at < 0 || at == len(ids)-1 {
		t.Fatalf("rows: %q", ids)
	}
	chooseRow(t, l, "app:com.apple.Safari")
	labels := menuLabels(l)
	if labels[len(labels)-1] != locales.ActionHideFromSearch() {
		t.Fatalf("Cmd-K: %q", labels)
	}
	runAction(t, l, locales.ActionHideFromSearch())
	if slices.Contains(rowIDs(l), "app:com.apple.Safari") {
		t.Fatalf("Safari still shows: %q", shown(l))
	}
	if got := l.currentID(); got != ids[at+1] {
		t.Errorf("chosen %s, want %s", got, ids[at+1])
	}
	if !l.shown || l.notice != locales.HiddenFromSearchNotice() || l.noticeAct == nil || !l.noticeAct.long || l.noticeAct.icon != "eye.slash" {
		t.Errorf("notice %q, shown %v", l.notice, l.shown)
	}
	if !l.g.settings.Hidden("app:com.apple.Safari", settings.GroupOtherApps) {
		t.Error("not kept hidden")
	}

	// The last row hidden: the one above chosen.
	ids = rowIDs(l)
	last := ids[len(ids)-1]
	chooseRow(t, l, last)
	runAction(t, l, locales.ActionHideFromSearch())
	if got := l.currentID(); got != ids[len(ids)-2] {
		t.Errorf("the last hidden: chosen %s, want %s", got, ids[len(ids)-2])
	}

	typeQuery(l, "keep awake 45")
	chooseRow(t, l, awakeTypedID)
	if slices.Contains(menuLabels(l), locales.ActionHideFromSearch()) {
		t.Errorf("the typed row: %q", menuLabels(l))
	}
}

// Settings › Search & Shortcuts: a row's switch hides its item at once;
// Hidden lists it, kept there once shown again until the filter changes;
// with none hidden, Hidden says so; Space turns the switch of the line
// chosen, a group's title's too.
func TestHotkeyPrefsSwitches(t *testing.T) {
	p, tt, _ := hotkeyPrefs(t, "en")
	l := p.g.launcher
	safari := p.hkRowByID("app:com.apple.Safari")
	p.setRowShown(safari, false)
	if found(l, "safari", "app:com.apple.Safari") {
		t.Error("switched off, found")
	}
	p.hk.filter = hkFilterHidden
	p.filterHotkeys(true)
	tt.Frame()
	if got := shownIDs(p); !slices.Equal(got, []string{"app:com.apple.Safari"}) {
		t.Fatalf("Hidden: %q", got)
	}
	// Searched by name under Hidden.
	tt.Type("saf")
	tt.Frame()
	if got := shownIDs(p); !slices.Equal(got, []string{"app:com.apple.Safari"}) {
		t.Errorf("Hidden, searching: %q", got)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	// Shown again: it stays until the filter changes.
	p.setRowShown(safari, true)
	tt.Frame()
	if !slices.Contains(shownIDs(p), "app:com.apple.Safari") || !found(l, "safari", "app:com.apple.Safari") {
		t.Errorf("shown again: %q", shownIDs(p))
	}
	p.filterHotkeys(true)
	tt.Frame()
	if len(p.hk.lines) != 0 || !has(tt, locales.HiddenEmpty()) {
		t.Errorf("none hidden: %q", tt.Texts())
	}

	// Space on a group's title: the group hidden whole, its rows off.
	p.hk.filter = hkFilterAll
	p.filterHotkeys(true)
	p.hk.list.Choose(rowIndex(t, p, "[systemApps]"))
	p.toggleShown()
	tt.Frame()
	if !p.g.settings.GroupHidden(settings.GroupSystemApps) || !p.rowHidden(p.hkRowByID("app:com.apple.Notes")) {
		t.Error("group switch: not hidden")
	}
	if got := p.hotkeyRowLabel(rowIndex(t, p, "[systemApps]")); !strings.Contains(got, locales.GroupHiddenCount(2)) {
		t.Errorf("group label %q", got)
	}
	// Space on a row of it: shown alone.
	p.hk.list.Choose(rowIndex(t, p, "app:com.apple.Notes"))
	p.toggleShown()
	if p.rowHidden(p.hkRowByID("app:com.apple.Notes")) || !p.g.settings.GroupHidden(settings.GroupSystemApps) {
		t.Error("Notes not shown alone")
	}
	if !found(l, "notes", "app:com.apple.Notes") || found(l, "terminal", "app:com.apple.Terminal") {
		t.Error("search: Notes alone of System Apps")
	}
}

// A group collapsed shows its title alone, kept in the settings; while
// searching, it shows what is found, the setting as it was.
func TestHotkeyPrefsCollapse(t *testing.T) {
	p, tt, _ := hotkeyPrefs(t, "en")
	i := rowIndex(t, p, "[otherApps]")
	p.submitLine(i)
	tt.Frame()
	if !p.g.settings.Collapsed(settings.GroupOtherApps) || slices.Contains(shownIDs(p), "app:com.apple.Safari") {
		t.Fatalf("collapsed: %q", lineIDs(p))
	}
	if s, err := settings.Load(p.g.settingsPath()); err != nil || !s.Collapsed(settings.GroupOtherApps) {
		t.Errorf("not saved: %v", err)
	}
	tt.Type("saf")
	tt.Frame()
	if got := withoutActions(shownIDs(p)); !slices.Equal(got, []string{"app:com.apple.Safari"}) {
		t.Errorf("searching: %q", got)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if slices.Contains(shownIDs(p), "app:com.apple.Safari") || !p.g.settings.Collapsed(settings.GroupOtherApps) {
		t.Errorf("cleared: %q", lineIDs(p))
	}
	// Left on a row goes to its title; Right there expands.
	p.hk.list.Choose(rowIndex(t, p, "app:com.apple.Notes"))
	p.foldKey(true)
	if g, ok := p.hkTitleAt(p.hk.chosen); !ok || g != settings.GroupSystemApps {
		t.Errorf("Left: %v", lineIDs(p)[p.hk.chosen])
	}
	p.hk.list.Choose(rowIndex(t, p, "[otherApps]"))
	p.foldKey(false)
	if p.g.settings.Collapsed(settings.GroupOtherApps) {
		t.Error("Right did not expand")
	}
}

// --- Screenshots ---

// The page and the search panel with items hidden (design.md 4.4, 2.5):
// GOCAST_SCREENS=<dir> go test -run TestScreensSearchShortcuts.
func TestScreensSearchShortcuts(t *testing.T) {
	dir := screensDir(t)
	full := image.Rect(0, 0, prefsWidth, prefsHeight)
	const (
		cal   = "app:com.apple.iCal"
		chess = "app:com.apple.Chess"
	)
	for _, loc := range []string{"zh-Hans", "en"} {
		for _, dark := range []bool{false, true} {
			name := "ss-"
			if loc == "en" {
				name += "en-"
			}
			mode := "light"
			if dark {
				mode = "dark"
			}
			// The page: Search Notes and Quit Gocast hidden, Chess too.
			shot := func(state string, setup func(p *prefsWindow, tt *ui.Tester)) {
				p, tt, _ := hotkeyPrefs(t, loc)
				withSystemApps(p.g.launcher, loc != "en", true)
				screenIcons(t, p)
				g := p.g
				g.setSearchHidden(noteOwnID(settings.NoteSearch), settings.GroupGocast, true)
				g.setSearchHidden(quitItemID, settings.GroupGocast, true)
				g.setSearchHidden(chess, settings.GroupSystemApps, true)
				p.enterShortcuts()
				tt.SetScale(2)
				tt.SetDark(dark)
				tt.Frame()
				setup(p, tt)
				saveScreen(t, tt, dir, name+state+"-"+mode, full)
			}
			collapse := func(p *prefsWindow, groups ...string) {
				for _, g := range groups {
					p.g.settings.SetCollapsed(g, true)
				}
				p.layoutHotkeys()
			}
			// System Apps off, Calendar on alone.
			groupOff := func(p *prefsWindow) {
				p.g.setGroupHidden(settings.GroupSystemApps, true)
				p.g.setSearchHidden(cal, settings.GroupSystemApps, false)
			}
			shot("all", func(p *prefsWindow, tt *ui.Tester) {})
			shot("gocast", func(p *prefsWindow, tt *ui.Tester) {
				collapse(p, settings.GroupSystem, settings.GroupPresets, settings.GroupSystemApps, settings.GroupOtherApps)
			})
			shot("group-off", func(p *prefsWindow, tt *ui.Tester) {
				groupOff(p)
				collapse(p, settings.GroupGocast, settings.GroupSystem)
				p.hk.list.Choose(rowIndex(t, p, "[systemApps]"))
				tt.Frame()
			})
			if loc == "en" {
				shot("hidden", func(p *prefsWindow, tt *ui.Tester) {
					groupOff(p)
					p.hk.filter = hkFilterHidden
					p.filterHotkeys(true)
				})
				continue
			}
			shot("hidden", func(p *prefsWindow, tt *ui.Tester) {
				groupOff(p)
				p.hk.filter = hkFilterHidden
				p.filterHotkeys(true)
			})
			shot("hidden-empty", func(p *prefsWindow, tt *ui.Tester) {
				p.g.settings.HiddenItems = nil
				p.hk.filter = hkFilterHidden
				p.filterHotkeys(true)
			})
			shot("search", func(p *prefsWindow, tt *ui.Tester) { tt.Type("rili") })
			shot("recording", func(p *prefsWindow, tt *ui.Tester) {
				tt.Type("rili")
				tt.Frame()
				p.startRecording(rowIndex(t, p, cal))
				tt.Frame()
				tt.CodeDown(ui.Alt, ui.CodeAltLeft)
				tt.CodeDown(ui.Alt|ui.Cmd, ui.CodeSuperLeft)
			})
			shot("keep-awake", func(p *prefsWindow, tt *ui.Tester) {
				_ = p.g.setHotkey(settings.Hotkey{Awake: settings.AwakeToggle}, "Ctrl+Alt+K")
				collapse(p, settings.GroupGocast)
				p.hk.list.ScrollTo(rowIndex(t, p, awakeToggleID)-4, ui.Start)
				p.hk.list.Choose(rowIndex(t, p, awakeToggleID))
				tt.Frame()
			})
		}
	}

	// The search panel: Cmd-K on Safari, then after Hide from Search.
	for _, loc := range []string{"zh-Hans", "en"} {
		for _, dark := range []bool{false, true} {
			name := "ss-"
			if loc == "en" {
				name += "en-"
			}
			mode := "light"
			if dark {
				mode = "dark"
			}
			panel := func(state string, setup func(l *launcher, tt *ui.Tester)) {
				l, tt, _ := hotkeyPanel(t, loc)
				l.shown = true
				tt.SetScale(2)
				tt.SetDark(dark)
				chooseRow(t, l, "app:com.apple.Safari")
				tt.Frame()
				setup(l, tt)
				saveScreen(t, tt, dir, name+state+"-"+mode, panelRect)
			}
			panel("menu", func(l *launcher, tt *ui.Tester) { l.toggleMenu() })
			panel("hidden-notice", func(l *launcher, tt *ui.Tester) {
				runAction(t, l, locales.ActionHideFromSearch())
			})
		}
	}
}
