package app

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gocast/internal/presets"
	"gocast/internal/settings"
	"gocast/internal/shortcut"
	"gocast/internal/usage"
	"gocast/locales"
)

func TestGocastItems(t *testing.T) {
	inLocale(t, "zh-Hans")
	var did []string
	do := gocastDo{
		clipboard: func() { did = append(did, "clipboard") },
		terminal:  func() { did = append(did, "terminal") },
		addPreset: func() { did = append(did, "add") },
		settings:  func() { did = append(did, "settings") },
		quit:      func() { did = append(did, "quit") },
		notes:     func() { did = append(did, "notes") },
		newNote:   func() { did = append(did, "new-note") },
		notesKey:  "Alt+Cmd+N",
	}
	// copy.md 2.2a: the order; design.md 2.5a: the symbols and the keys,
	// none for Add Preset and Quit Gocast (no ⌘Q).
	var got []string
	for _, o := range gocastItems("Ctrl+`", "Ctrl+Cmd+V", do) {
		got = append(got, o.id+" "+o.symbol+" "+o.label+" "+strings.Join(shortcut.Caps(o.accel), ""))
		o.run()
	}
	want := []string{
		"gocast:clipboard doc.on.clipboard 剪贴板历史 ⌃⌘V",
		"gocast:notes note.text 笔记 ⌥⌘N",
		"gocast:terminal apple.terminal 打开悬浮终端 ⌃`",
		"gocast:add-preset plus 添加预设 ",
		"gocast:settings gearshape 设置… ⌘,",
		"gocast:quit power 退出 Gocast ",
		"gocast:new-note square.and.pencil 新建笔记 ",
		"gocast:search-notes doc.text.magnifyingglass 搜索笔记 ",
	}
	if !slices.Equal(got, want) {
		t.Errorf("items:\n got %q\nwant %q", got, want)
	}
	if want := []string{"clipboard", "notes", "terminal", "add", "settings", "quit", "new-note"}; !slices.Equal(did, want) {
		t.Errorf("ran %q, want %q", did, want)
	}
	// The shortcuts as the user set them; none registered, no keys.
	if o := gocastItems("Cmd+Alt+T", "Cmd+Shift+V", do); o[0].accel != "Cmd+Shift+V" || o[2].accel != "Cmd+Alt+T" {
		t.Errorf("shortcuts %q, %q", o[0].accel, o[2].accel)
	}
	if o := gocastItems("", "", do); o[0].accel != "" || o[2].accel != "" {
		t.Errorf("shortcuts without one: %q, %q", o[0].accel, o[2].accel)
	}
	// Settings… with a hotkey of its own: that, not ⌘, (design.md 4.4).
	do.settingsKey, do.addPresetKey = "Alt+Cmd+,", "Ctrl+Alt+P"
	if o := gocastItems("", "", do); o[ownIndex(o, settingsItemID)].accel != "Alt+Cmd+," || o[ownIndex(o, addPresetItemID)].accel != "Ctrl+Alt+P" {
		t.Errorf("hotkeys of Settings… and Add Preset: %+v", o)
	}
}

func TestSuggestionsCopy(t *testing.T) {
	// copy.md 2.2a, 2.3 and 4.3.
	for _, c := range []struct {
		locale string
		want   []string
	}{
		{"zh-Hans", []string{"Gocast", "打开", "退出", "在悬浮终端打开",
			"剪贴板历史", "打开悬浮终端", "添加预设", "设置…", "退出 Gocast",
			"打开时只显示搜索框", "打开后，按 ⌘Space 只出现搜索框，打字才出结果。"}},
		{"en", []string{"Gocast", "Open", "Quit", "Open in Floating Terminal",
			"Clipboard History", "Open Floating Terminal", "Add Preset", "Settings…", "Quit Gocast",
			"Show only the search field", "When on, ⌘Space shows just the search field. Results appear as you type."}},
	} {
		inLocale(t, c.locale)
		got := []string{locales.SearchTagGocast(),
			primaryName(entry{own: clipItemID}), primaryName(entry{own: quitItemID}), primaryName(entry{preset: "a"}),
			locales.ClipboardHistory(), locales.MenuTerminal(), locales.PresetsAdd(), locales.TraySettings(), locales.TrayQuit(),
			locales.GeneralSearchOnly(), searchOnlyHint("", "Cmd+Space")}
		if !slices.Equal(got, c.want) {
			t.Errorf("%s:\n got %q\nwant %q", c.locale, got, c.want)
		}
	}
	inLocale(t, "en")
	if got := searchOnlyHint("Alt+Shift+Space", "Cmd+Space"); !strings.Contains(got, "⌥⇧Space") {
		t.Errorf("hint with ⌥⇧Space registered: %q", got)
	}
}

func TestSuggested(t *testing.T) {
	there := map[string]bool{}
	exists := func(id string) bool { return there[id] }
	add := func(ids ...string) {
		for _, id := range ids {
			there[id] = true
		}
	}
	pad := []string{"preset:p", "app:com.apple.finder", "app:com.apple.Safari", "app:com.apple.systempreferences", "app:com.apple.Notes"}

	// Nothing opened yet: the padding, skipping what isn't there.
	add("preset:p", "app:com.apple.finder", "app:com.apple.systempreferences", "app:com.apple.Notes")
	if got, want := suggested(nil, pad, exists), []string{"preset:p", "app:com.apple.finder", "app:com.apple.systempreferences", "app:com.apple.Notes"}; !slices.Equal(got, want) {
		t.Errorf("unused: %q, want %q", got, want)
	}
	add("app:com.apple.Safari", "app:x", "app:y", "gocast:terminal")

	// Fewer than 5: padded after them to 5, without repeating them; what
	// was uninstalled (app:gone) is skipped.
	ranked := []string{"app:x", "app:gone", "app:com.apple.finder", "gocast:terminal"}
	want := []string{"app:x", "app:com.apple.finder", "gocast:terminal", "preset:p", "app:com.apple.Safari"}
	if got := suggested(ranked, pad, exists); !slices.Equal(got, want) {
		t.Errorf("few: %q, want %q", got, want)
	}

	// 5 or more: no padding; at most 8.
	var many []string
	for i := range 12 {
		id := "app:" + string(rune('a'+i))
		add(id)
		many = append(many, id)
	}
	if got := suggested(many[:6], pad, exists); !slices.Equal(got, many[:6]) {
		t.Errorf("six: %q", got)
	}
	if got := suggested(many, pad, exists); !slices.Equal(got, many[:suggestMost]) {
		t.Errorf("twelve: %q, want the first 8", got)
	}
	// One gone among them: the next comes up.
	delete(there, many[2])
	if got, want := suggested(many, pad, exists), append(slices.Clone(many[:2]), many[3:9]...); !slices.Equal(got, want) {
		t.Errorf("one gone: %q, want %q", got, want)
	}
}

func TestPadIDs(t *testing.T) {
	apps := []string{"app:com.apple.finder", "app:com.apple.Safari", "app:com.apple.systempreferences", "app:com.apple.Notes"}
	ps := []presets.Preset{{ID: "a"}, {ID: "b", Default: true}}
	if got, want := padIDs(ps), append([]string{"preset:b"}, apps...); !slices.Equal(got, want) {
		t.Errorf("with a default: %q, want %q", got, want)
	}
	ps[1].Default = false
	if got, want := padIDs(ps), append([]string{"preset:a"}, apps...); !slices.Equal(got, want) {
		t.Errorf("without a default: %q, want %q", got, want)
	}
	if got := padIDs(nil); !slices.Equal(got, apps) {
		t.Errorf("without presets: %q, want %q", got, apps)
	}
}

// spellRows spells rows as they read: an item's ID, Gocast's own item's
// ID, "*" after one not chosen as the list shows.
func spellRows(rows []panelRow, items []string, own []ownItem) []string {
	var out []string
	for _, r := range rows {
		s := ""
		if r.own >= 0 {
			s = own[r.own].id
		} else {
			s = items[r.item]
		}
		if r.later {
			s += "*"
		}
		out = append(out, s)
	}
	return out
}

func TestSuggestionRows(t *testing.T) {
	inLocale(t, "en")
	own := gocastItems("", "", gocastDo{})
	items := []string{"preset:p", clipItemID, "app:a"}
	item := func(id string) int { return slices.Index(items, id) }
	ownRows := []string{"gocast:clipboard", "gocast:notes", "gocast:terminal", "gocast:add-preset", "gocast:settings", "gocast:quit"}

	// One column, no titles (design.md 2.5a): the suggestions, Gocast's
	// own items as theirs, then Gocast's own items not among them, so
	// Settings… and the clipboard history show once; what isn't there is
	// skipped.
	got := spellRows(suggestionRows([]string{"app:a", clipItemID, "app:gone", settingsItemID, "preset:p"}, item, own, allShown), items, own)
	want := []string{"app:a", "gocast:clipboard", "gocast:settings", "preset:p",
		"gocast:notes", "gocast:terminal", "gocast:add-preset", "gocast:quit"}
	if !slices.Equal(got, want) {
		t.Errorf("rows:\n got %q\nwant %q", got, want)
	}
	// No suggestions: Gocast's own items alone, the first chosen.
	rows := suggestionRows(nil, item, own, allShown)
	if got := spellRows(rows, items, own); !slices.Equal(got, ownRows) {
		t.Errorf("no suggestions: %q", got)
	}
	if f := firstRow(rows); f != 0 || rows[f].own != ownIndex(own, clipItemID) {
		t.Errorf("first row %d", f)
	}
	// Hidden: left out, the next coming up (design.md 4.4).
	shown := func(id string) bool { return id != notesItemID && id != quitItemID }
	if got := spellRows(suggestionRows(nil, item, own, shown), items, own); !slices.Equal(got, []string{"gocast:clipboard", "gocast:terminal", "gocast:add-preset", "gocast:settings"}) {
		t.Errorf("hidden: %q", got)
	}
	// Results: Gocast's own items are search items there.
	if got := spellRows(itemRows([]int{2, 0}), items, own); !slices.Equal(got, []string{"app:a", "preset:p"}) {
		t.Errorf("results: %q", got)
	}
}

// Relaunch to Update is first of Gocast's own items (design.md 4.5); first
// of all, it is not chosen as the list shows, the row after it is, unless
// it is alone.
func TestSuggestionRowsUpdate(t *testing.T) {
	inLocale(t, "en")
	own := gocastItems("", "", gocastDo{update: "0.2.0"})
	items := []string{"app:a"}
	item := func(id string) int { return slices.Index(items, id) }

	rows := suggestionRows([]string{"app:a", termItemID}, item, own, allShown)
	want := []string{"app:a", "gocast:terminal", "gocast:update*", "gocast:clipboard", "gocast:notes", "gocast:add-preset", "gocast:settings", "gocast:quit"}
	if got := spellRows(rows, items, own); !slices.Equal(got, want) {
		t.Errorf("rows:\n got %q\nwant %q", got, want)
	}
	if f := firstRow(rows); f != 0 {
		t.Errorf("first row %d, want the first suggestion", f)
	}
	rows = suggestionRows(nil, item, own, allShown)
	if got := spellRows(rows, items, own); len(got) == 0 || got[0] != "gocast:update*" {
		t.Fatalf("no suggestions: %q", got)
	}
	if f := firstRow(rows); f != 1 || own[rows[f].own].id != clipItemID {
		t.Errorf("first row %d, want the clipboard history's, after Relaunch to Update", f)
	}
	// Alone: chosen.
	alone := func(id string) bool { return id == updateItemID }
	if rows := suggestionRows(nil, item, own, alone); len(rows) != 1 || firstRow(rows) != 0 {
		t.Errorf("alone: %q, first %d", spellRows(rows, items, own), firstRow(rows))
	}
	if firstRow(nil) != -1 {
		t.Error("a first row of none")
	}
}

func TestStepRow(t *testing.T) {
	inLocale(t, "en")
	own := gocastItems("", "", gocastDo{})
	rows := suggestionRows([]string{termItemID, settingsItemID}, func(string) int { return -1 }, own, allShown)
	// terminal settings clipboard notes add quit
	for _, c := range []struct{ from, step, want int }{
		{0, 1, 1}, {1, 1, 2}, {2, -1, 1}, {0, -1, 0}, {5, 1, 5}, {5, 3, 5},
		{0, 3, 3}, {-1, 1, 0}, {99, -1, 0}, {4, -9, 0},
	} {
		if got := stepRow(rows, c.from, c.step); got != c.want {
			t.Errorf("stepRow(%d, %d) = %d, want %d", c.from, c.step, got, c.want)
		}
	}
	for _, c := range []struct{ at, want int }{{0, 0}, {3, 3}, {6, 5}, {-1, 0}} {
		if got := nearRow(rows, c.at); got != c.want {
			t.Errorf("nearRow(%d) = %d, want %d", c.at, got, c.want)
		}
	}
	if nearRow(nil, 0) != -1 {
		t.Error("a near row of none")
	}
}

func TestRestoring(t *testing.T) {
	// design.md 2.5a, 2.6: within 90 seconds the panel shows as it was.
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		hidden time.Time
		want   bool
	}{
		{time.Time{}, false},
		{now.Add(-89 * time.Second), true},
		{now.Add(-90 * time.Second), false},
		{now.Add(-time.Hour), false},
	} {
		if got := restoring(c.hidden, now); got != c.want {
			t.Errorf("hidden %v ago: restoring %v, want %v", now.Sub(c.hidden), got, c.want)
		}
	}
}

// testLauncher is a search panel without its window, with apps and
// presets, and usage as given.
func testLauncher(t *testing.T, u *usage.Store, ps []presets.Preset) *launcher {
	t.Helper()
	g := &gocast{usage: u, presets: ps}
	l := &launcher{g: g, icons: map[string]*ui.Bitmap{}, filtered: noQuery}
	for _, a := range []mygo.InstalledApp{
		{Name: "Finder", ID: "com.apple.finder", Path: "/System/Library/CoreServices/Finder.app"},
		{Name: "Notes", ID: "com.apple.Notes", Path: "/System/Applications/Notes.app"},
		{Name: "Safari", ID: "com.apple.Safari", Path: "/Applications/Safari.app"},
		{Name: "Xcode", ID: "com.apple.dt.Xcode", Path: "/Applications/Xcode.app"},
		{Name: "Zed", ID: "dev.zed.Zed", Path: "/Applications/Zed.app"},
	} {
		l.apps = append(l.apps, a)
		l.alts = append(l.alts, nil)
		l.places = append(l.places, "")
	}
	l.rebuild()
	return l
}

// shown spells the panel's rows by their keys, the chosen one marked.
func shown(l *launcher) []string {
	var out []string
	for i := range l.rows {
		s := l.rowKey(i).(string)
		if i == l.chosen {
			s = "> " + s
		}
		out = append(out, s)
	}
	return out
}

func TestLauncherSuggestions(t *testing.T) {
	inLocale(t, "en")
	now := time.Now()
	u := usage.New()
	u.Record("app:dev.zed.Zed", "", now.Add(-time.Hour))
	u.Record("app:dev.zed.Zed", "", now.Add(-time.Hour))
	u.Record(termItemID, "", now.Add(-2*time.Hour))
	u.Record("preset:gone", "", now) // deleted since
	l := testLauncher(t, u, []presets.Preset{{ID: "p", Name: "pi", Dir: "~"}})

	// As it shows: ranked once, the first chosen, padded to 5.
	l.ranked = u.Ranked(now)
	l.filtered = noQuery
	l.filter()
	// One column, no titles; the floating terminal, suggested, not again
	// among Gocast's own items (design.md 2.5a).
	own := []string{"gocast:clipboard", "gocast:notes", "gocast:add-preset", "gocast:settings", "gocast:quit"}
	want := append([]string{"> app:dev.zed.Zed", "gocast:terminal", "preset:p", "app:com.apple.finder", "app:com.apple.Safari"}, own...)
	if got := shown(l); !slices.Equal(got, want) {
		t.Errorf("as it shows:\n got %q\nwant %q", got, want)
	}
	if l.targetHeight() != expandedHeight || l.compact() {
		t.Errorf("height %v, compact %v: want 475 and not", l.targetHeight(), l.compact())
	}
	if e, ok := l.current(); !ok || e.app == nil || e.app.ID != "dev.zed.Zed" {
		t.Errorf("current %+v %v", e, ok)
	}

	// Opened meanwhile, and the apps listed again: no reranking while it
	// shows; the choice stays on its row.
	u.Record("app:com.apple.Notes", "", now)
	u.Record("app:com.apple.Notes", "", now)
	u.Record("app:com.apple.Notes", "", now)
	l.move(2)
	l.rebuild()
	want[0], want[2] = "app:dev.zed.Zed", "> preset:p"
	if got := shown(l); !slices.Equal(got, want) {
		t.Errorf("rebuilt:\n got %q\nwant %q", got, want)
	}
	// Down past the suggestions to Gocast's own items, stopping at the end.
	l.move(3)
	if r, _ := l.chosenRow(); r.own < 0 || l.own[r.own].id != clipItemID {
		t.Errorf("after the suggestions: row %d %+v", l.chosen, r)
	}
	l.move(99)
	if e, _ := l.current(); e.own != quitItemID || primaryName(e) != "Quit" {
		t.Errorf("at the end: %+v", e)
	}

	// Typing: the results, no titles, the first chosen; the height stays.
	l.query = "saf"
	l.filter()
	if got := withoutActions(shown(l)); !slices.Equal(got, []string{"> app:com.apple.Safari"}) {
		t.Errorf("typing: %q", got)
	}
	// Cleared: the suggestions again, as ranked when it showed, the
	// first chosen.
	l.query = ""
	l.filter()
	want[0], want[2] = "> app:dev.zed.Zed", "preset:p"
	if got := shown(l); !slices.Equal(got, want) {
		t.Errorf("cleared:\n got %q\nwant %q", got, want)
	}
	// Shown again after 90 seconds: ranked afresh.
	l.ranked = u.Ranked(now)
	l.filtered = noQuery
	l.filter()
	if got := shown(l); len(got) < 2 || got[0] != "> app:com.apple.Notes" || got[1] != "app:dev.zed.Zed" {
		t.Errorf("reranked: %q", got)
	}
}

func TestLauncherSearchOnly(t *testing.T) {
	inLocale(t, "en")
	l := testLauncher(t, usage.New(), nil)
	l.searchOnly = true
	l.filter()
	if !l.compact() || l.targetHeight() != barHeight {
		t.Fatalf("search only: compact %v, height %v", l.compact(), l.targetHeight())
	}
	if _, ok := l.current(); ok {
		t.Error("a row chosen under the bar alone")
	}
	// Down (or Cmd-K) expands to the suggestions, the first chosen.
	l.move(1)
	if l.compact() || l.targetHeight() != expandedHeight {
		t.Fatalf("expanded: compact %v, height %v", l.compact(), l.targetHeight())
	}
	if got := shown(l); len(got) < 1 || got[0] != "> app:com.apple.finder" {
		t.Errorf("expanded: %q", got)
	}
	// Typing then clearing: the bar alone again.
	l.query = "x"
	l.filter()
	if l.compact() || len(l.rows) == 0 {
		t.Errorf("typing: compact %v, %d rows", l.compact(), len(l.rows))
	}
	l.query = ""
	l.filter()
	if !l.compact() {
		t.Error("cleared: not the bar alone")
	}
}

// Gocast's own items are found by their titles in both languages, with
// pinyin and initials, whatever the language it shows (copy.md 2.2a).
func TestLauncherSearchesOwnItems(t *testing.T) {
	cases := []struct{ query, want string }{
		{"剪贴板", clipItemID}, {"jtb", clipItemID}, {"clip", clipItemID}, {"history", clipItemID},
		{"终端", termItemID}, {"zhongduan", termItemID}, {"terminal", termItemID}, {"floating", termItemID},
		{"添加预设", addPresetItemID}, {"tjys", addPresetItemID}, {"add preset", addPresetItemID}, {"preset", addPresetItemID},
		{"设置", settingsItemID}, {"shezhi", settingsItemID}, {"sz", settingsItemID}, {"settings", settingsItemID}, {"set", settingsItemID},
		{"退出", quitItemID}, {"tc", quitItemID}, {"quit", quitItemID}, {"gocast", quitItemID},
	}
	symbols := map[string]string{clipItemID: "doc.on.clipboard", termItemID: "apple.terminal", addPresetItemID: "plus", settingsItemID: "gearshape", quitItemID: "power"}
	for _, locale := range []string{"zh-Hans", "en"} {
		inLocale(t, locale)
		l := testLauncher(t, usage.New(), nil)
		for _, c := range cases {
			l.query = c.query
			l.filter()
			if got := shown(l); len(got) == 0 || got[0] != "> "+c.want {
				t.Errorf("%s %q: %q, want %s first", locale, c.query, got, c.want)
				continue
			}
			// As the clipboard history's result: its symbol, the Gocast tag.
			e, _ := l.current()
			// Its action, then Hide from Search alone (design.md 2.5).
			if a := l.actions(); len(a) != 2 || a[0].icon != symbols[c.want] || a[1].label != locales.ActionHideFromSearch() {
				t.Errorf("%s %q: actions %+v, want %s and Hide from Search", locale, c.query, a, symbols[c.want])
			}
			if s := ownSymbol(l.own, e.own); s != symbols[c.want] {
				t.Errorf("%s %q: symbol %s, want %s", locale, c.query, s, symbols[c.want])
			}
			title := ownTitle(locales.In(locale), c.want)
			if got := l.rowLabel(l.chosen); got != title+listSep()+locales.SearchTagGocast()+hotkeySpoken(l.own[ownIndex(l.own, c.want)].accel) {
				t.Errorf("%s %q: label %q", locale, c.query, got)
			}
		}
	}
	// Found, Quit Gocast is still not scored for the suggestions.
	if scored(quitItemID) || !scored(settingsItemID) {
		t.Error("Quit Gocast scored, or Settings… not")
	}
}

// allShown hides none of Gocast's own items.
func allShown(string) bool { return true }

// The list under an empty query as it draws (design.md 2.5a): no titles;
// Gocast's own rows with the Gocast tag, their keys on the row chosen
// alone, their labels saying their shortcuts chosen or not (copy.md 2.3,
// 11.4); Settings…, opened often, once, among the suggestions.
func TestSuggestionsDrawn(t *testing.T) {
	l, tt, _ := hotkeyPanel(t, "en")
	if has(tt, "Suggestions") || !has(tt, locales.SearchTagGocast()) || !has(tt, locales.ClipboardHistory()) {
		t.Fatalf("texts: %q", tt.Texts())
	}
	for i := range l.rows {
		if s := l.rowLabel(i); strings.Contains(s, "heading") {
			t.Errorf("a title: %q", s)
		}
	}
	// Terminal chosen, with ⌥⌘T: the clipboard history's ⌃⌘V not shown.
	if got := shown(l); len(got) == 0 || got[0] != "> app:com.apple.Terminal" {
		t.Fatalf("rows: %q", got)
	}
	if !has(tt, "⌥") || !has(tt, "T") || has(tt, "⌃") || has(tt, "V") {
		t.Errorf("Terminal chosen: %q", tt.Texts())
	}
	clip := slices.Index(shown(l), clipItemID)
	if got := l.rowLabel(clip); got != "Clipboard History, Gocast, shortcut Control Command V" {
		t.Errorf("clipboard history, not chosen: %q", got)
	}
	// The clipboard history chosen: its keys, Terminal's gone.
	l.choose(clip)
	tt.Frame()
	if !has(tt, "⌃") || !has(tt, "⌘") || !has(tt, "V") || has(tt, "⌥") {
		t.Errorf("clipboard history chosen: %q", tt.Texts())
	}
	if got := l.rowLabel(clip); got != "Clipboard History, Gocast, shortcut Control Command V" {
		t.Errorf("clipboard history, chosen: %q", got)
	}
	for id, want := range map[string]string{
		settingsItemID:  "Settings…, Gocast, shortcut Command Comma",
		addPresetItemID: "Add Preset, Gocast",
		quitItemID:      "Quit Gocast, Gocast",
	} {
		if got := l.rowLabel(slices.Index(shown(l), id)); got != want {
			t.Errorf("%s: %q, want %q", id, got, want)
		}
	}

	// Settings… opened most: first, and not again after.
	for range 20 {
		l.g.usage.Record(settingsItemID, "", clipNow)
	}
	l.leave(clipNow)
	l.enterSearch(clipNow.Add(time.Hour))
	tt.Frame()
	got := shown(l)
	if len(got) == 0 || got[0] != "> "+settingsItemID {
		t.Fatalf("Settings… opened most: %q", got)
	}
	n := 0
	for _, s := range got {
		if strings.TrimPrefix(s, "> ") == settingsItemID {
			n++
		}
	}
	if n != 1 || !slices.Contains(got, clipItemID) || !slices.Contains(got, quitItemID) {
		t.Errorf("Settings… %d times: %q", n, got)
	}
	if !has(tt, "⌘") || !has(tt, ",") {
		t.Errorf("Settings… chosen, no ⌘,: %q", tt.Texts())
	}
}

// Typed, Gocast's own items show their shortcuts as in the list under an
// empty query: the keys on the row chosen alone, the label saying them
// (design.md 2.3, copy.md 11.4).
func TestOwnResultsShortcuts(t *testing.T) {
	l, tt, _ := hotkeyPanel(t, "en")
	for _, c := range []struct {
		query, id, label string
		caps             []string
	}{
		{"clipboard", clipItemID, "Clipboard History, Gocast, shortcut Control Command V", []string{"⌃", "⌘", "V"}},
		{"floating", termItemID, "Open Floating Terminal, Gocast, shortcut Control Grave Accent", []string{"⌃", "`"}},
		{"settings", settingsItemID, "Settings…, Gocast, shortcut Command Comma", []string{"⌘", ","}},
		{"add preset", addPresetItemID, "Add Preset, Gocast", nil},
		{"quit", quitItemID, "Quit Gocast, Gocast", nil},
	} {
		typeQuery(l, c.query)
		tt.Frame()
		if got := shown(l); len(got) == 0 || got[0] != "> "+c.id {
			t.Errorf("%q: %q", c.query, got)
			continue
		}
		if got := l.rowLabel(l.chosen); got != c.label {
			t.Errorf("%q: label %q, want %q", c.query, got, c.label)
		}
		for _, cp := range c.caps {
			if !has(tt, cp) {
				t.Errorf("%q: no %s in %q", c.query, cp, tt.Texts())
			}
		}
		if c.caps == nil && has(tt, "⌃") { // ⌘ is the actions capsule's
			t.Errorf("%q: keys %q", c.query, tt.Texts())
		}
	}
	// Add Preset with a hotkey: its keys; not chosen (under an empty
	// query, Terminal chosen with ⌥⌘T), its label says them still, its
	// keys not shown.
	l.g.settings.SetHotkey(settings.Hotkey{Own: settings.OwnAddPreset}, "Ctrl+Alt+P")
	l.rebuild()
	typeQuery(l, "preset")
	tt.Frame()
	i := slices.Index(withoutActions(shown(l)), "> "+addPresetItemID)
	if i < 0 || l.rowLabel(i) != "Add Preset, Gocast, shortcut Control Option P" || !has(tt, "⌥") || !has(tt, "P") {
		t.Errorf("Add Preset with a hotkey: %q, %q", shown(l), tt.Texts())
	}
	typeQuery(l, "")
	tt.Frame()
	i = slices.Index(shown(l), addPresetItemID)
	if i < 0 || l.rowLabel(i) != "Add Preset, Gocast, shortcut Control Option P" || has(tt, "⌃") || has(tt, "P") {
		t.Errorf("Add Preset not chosen: %q, %q", shown(l), tt.Texts())
	}
}
