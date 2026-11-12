package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gocast/internal/presets"
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
	}
	// copy.md 2.2a: the order; design.md 2.5a: the symbols and the keys,
	// none for Add Preset and Quit Gocast (no ⌘Q).
	var got []string
	for _, o := range gocastItems("Ctrl+`", "Ctrl+Cmd+V", do) {
		got = append(got, o.id+" "+o.symbol+" "+o.label+" "+strings.Join(o.caps, ""))
		o.run()
	}
	want := []string{
		"gocast:clipboard doc.on.clipboard 剪贴板历史 ⌃⌘V",
		"gocast:terminal apple.terminal 打开悬浮终端 ⌃`",
		"gocast:add-preset plus 添加预设 ",
		"gocast:settings gearshape 设置… ⌘,",
		"gocast:quit power 退出 Gocast ",
	}
	if !slices.Equal(got, want) {
		t.Errorf("items:\n got %q\nwant %q", got, want)
	}
	if want := []string{"clipboard", "terminal", "add", "settings", "quit"}; !slices.Equal(did, want) {
		t.Errorf("ran %q, want %q", did, want)
	}
	// The shortcuts as the user set them; none registered, no keys.
	if o := gocastItems("Cmd+Alt+T", "Cmd+Shift+V", do); strings.Join(o[0].caps, "") != "⇧⌘V" || strings.Join(o[1].caps, "") != "⌥⌘T" {
		t.Errorf("caps %q, %q", o[0].caps, o[1].caps)
	}
	if o := gocastItems("", "", do); len(o[0].caps) != 0 || len(o[1].caps) != 0 {
		t.Errorf("caps without a shortcut: %q, %q", o[0].caps, o[1].caps)
	}
}

func TestSuggestionsCopy(t *testing.T) {
	// copy.md v1.15, 2.2a, 2.3 and 4.3.
	for _, c := range []struct {
		locale string
		want   []string
	}{
		{"zh-Hans", []string{"推荐", "Gocast", "推荐，标题", "Gocast，标题", "打开", "退出", "在悬浮终端打开",
			"剪贴板历史", "打开悬浮终端", "添加预设", "设置…", "退出 Gocast",
			"打开时只显示搜索框", "打开后，按 ⌘Space 只出现搜索框，不列推荐，打字才出结果。"}},
		{"en", []string{"Suggestions", "Gocast", "Suggestions, heading", "Gocast, heading", "Open", "Quit", "Open in Floating Terminal",
			"Clipboard History", "Open Floating Terminal", "Add Preset", "Settings…", "Quit Gocast",
			"Show only the search field", "When on, ⌘Space shows just the search field. Results appear as you type."}},
	} {
		inLocale(t, c.locale)
		got := []string{locales.SearchSuggestions(), locales.SearchGocastGroup(),
			locales.SearchHeading(locales.SearchSuggestions()), locales.SearchHeading(locales.SearchGocastGroup()),
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

// spellRows spells rows as they read: "[title]", an item's ID, Gocast's own
// item's ID, "+" in the Gocast group.
func spellRows(rows []panelRow, items []string, own []ownItem) []string {
	var out []string
	for _, r := range rows {
		switch {
		case r.title != "":
			out = append(out, "["+r.title+"]")
		case r.own >= 0 && r.group:
			out = append(out, "+"+own[r.own].id)
		case r.own >= 0:
			out = append(out, own[r.own].id)
		default:
			out = append(out, items[r.item])
		}
	}
	return out
}

func TestSuggestionRows(t *testing.T) {
	inLocale(t, "en")
	own := gocastItems("", "", gocastDo{})
	items := []string{"preset:p", clipItemID, "app:a"}
	item := func(id string) int { return slices.Index(items, id) }
	group := []string{"[Gocast]", "+gocast:clipboard", "+gocast:terminal", "+gocast:add-preset", "+gocast:settings", "+gocast:quit"}

	// Gocast's own items show as theirs, the clipboard history too, and
	// again in the Gocast group; what isn't there is skipped.
	got := spellRows(suggestionRows([]string{"app:a", clipItemID, "app:gone", "gocast:settings", "preset:p"}, item, own), items, own)
	want := append([]string{"[Suggestions]", "app:a", "gocast:clipboard", "gocast:settings", "preset:p"}, group...)
	if !slices.Equal(got, want) {
		t.Errorf("rows:\n got %q\nwant %q", got, want)
	}
	// No suggestions: no title over none; the Gocast group, its first
	// row first.
	rows := suggestionRows(nil, item, own)
	if got := spellRows(rows, items, own); !slices.Equal(got, group) {
		t.Errorf("no suggestions: %q", got)
	}
	if f := firstRow(rows); f != 1 || rows[f].own != ownIndex(own, clipItemID) {
		t.Errorf("first row %d", f)
	}
	// Results: no titles.
	if got := spellRows(itemRows([]int{2, 0}), items, own); !slices.Equal(got, []string{"app:a", "preset:p"}) {
		t.Errorf("results: %q", got)
	}
}

func TestStepRow(t *testing.T) {
	inLocale(t, "en")
	own := gocastItems("", "", gocastDo{})
	rows := suggestionRows([]string{"gocast:terminal", "gocast:settings"}, func(string) int { return -1 }, own)
	// [Suggestions] terminal settings [Gocast] clipboard terminal add settings quit
	for _, c := range []struct{ from, step, want int }{
		{1, 1, 2}, {2, 1, 4}, {4, -1, 2}, {1, -1, 1}, {8, 1, 8}, {8, 3, 8},
		{1, 3, 5}, {-1, 1, 1}, {99, -1, 1},
	} {
		if got := stepRow(rows, c.from, c.step); got != c.want {
			t.Errorf("stepRow(%d, %d) = %d, want %d", c.from, c.step, got, c.want)
		}
	}
	// The first row of a group shows with its title, when the title is
	// above the rows in view.
	for _, c := range []struct{ chosen, first, want int }{
		{1, 0, 0}, {1, 1, 0}, {4, 4, 3}, {4, 1, 4}, {5, 5, 5},
	} {
		if got := revealedRow(rows, c.chosen, c.first); got != c.want {
			t.Errorf("revealedRow(%d, %d) = %d, want %d", c.chosen, c.first, got, c.want)
		}
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
	l := &launcher{g: g, icons: map[string]*ui.Bitmap{}, running: map[string]bool{}, filtered: noQuery}
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
	group := []string{"title:Gocast", "group:gocast:clipboard", "group:gocast:terminal", "group:gocast:add-preset", "group:gocast:settings", "group:gocast:quit"}
	want := append([]string{"title:Suggestions", "> app:dev.zed.Zed", "gocast:terminal", "preset:p", "app:com.apple.finder", "app:com.apple.Safari"}, group...)
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
	want[1], want[3] = "app:dev.zed.Zed", "> preset:p"
	if got := shown(l); !slices.Equal(got, want) {
		t.Errorf("rebuilt:\n got %q\nwant %q", got, want)
	}
	// Down across the groups, over the Gocast title, stopping at the end.
	l.move(3)
	if r, _ := l.chosenRow(); !r.group || l.own[r.own].id != clipItemID {
		t.Errorf("after the suggestions: row %d %+v", l.chosen, r)
	}
	l.move(99)
	if e, _ := l.current(); e.own != quitItemID || primaryName(e) != "Quit" {
		t.Errorf("at the end: %+v", e)
	}

	// Typing: the results, no titles, the first chosen; the height stays.
	l.query = "saf"
	l.filter()
	if got := shown(l); !slices.Equal(got, []string{"> app:com.apple.Safari"}) {
		t.Errorf("typing: %q", got)
	}
	// Cleared: the suggestions again, as ranked when it showed, the
	// first chosen.
	l.query = ""
	l.filter()
	want[1], want[3] = "> app:dev.zed.Zed", "preset:p"
	if got := shown(l); !slices.Equal(got, want) {
		t.Errorf("cleared:\n got %q\nwant %q", got, want)
	}
	// Shown again after 90 seconds: ranked afresh.
	l.ranked = u.Ranked(now)
	l.filtered = noQuery
	l.filter()
	if got := shown(l); len(got) < 3 || got[1] != "> app:com.apple.Notes" || got[2] != "app:dev.zed.Zed" {
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
	if got := shown(l); len(got) < 2 || got[0] != "title:Suggestions" || got[1] != "> app:com.apple.finder" {
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
			if a := l.actions(); len(a) != 1 || a[0].icon != symbols[c.want] {
				t.Errorf("%s %q: actions %+v, want %s alone", locale, c.query, a, symbols[c.want])
			}
			if s := ownSymbol(l.own, e.own); s != symbols[c.want] {
				t.Errorf("%s %q: symbol %s, want %s", locale, c.query, s, symbols[c.want])
			}
			title := ownTitle(locales.In(locale), c.want)
			if got := l.rowLabel(l.chosen); got != title+listSep()+locales.SearchTagGocast() {
				t.Errorf("%s %q: label %q", locale, c.query, got)
			}
		}
	}
	// Found, Quit Gocast is still not scored for the suggestions.
	if scored(quitItemID) || !scored(settingsItemID) {
		t.Error("Quit Gocast scored, or Settings… not")
	}
}
