package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"gocast/internal/clipboard"
	"gocast/locales"
)

// inLocale runs the test with the app's language set to locale.
func inLocale(t *testing.T, locale string) {
	t.Helper()
	i18n.SetLocaleFunc(func() string { return locale })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
}

var clipNow = time.Date(2026, 11, 12, 15, 0, 0, 0, time.Local)

// history is a history as the store lists it: pinned first, then newest
// first.
func history() []clipboard.Entry {
	return []clipboard.Entry{
		{ID: "p1", Kind: clipboard.KindText, Text: "ssh deploy@host", Pinned: true, Time: clipNow.AddDate(0, 0, -3)},
		{ID: "t1", Kind: clipboard.KindText, Text: "\n\n  周五发布清单 \t 1.\nmore", Time: clipNow.Add(-time.Hour)},
		{ID: "t2", Kind: clipboard.KindLink, Text: "https://www.github.com/xcc3641/gocast", Time: clipNow.Add(-2 * time.Hour)},
		{ID: "y1", Kind: clipboard.KindImage, Width: 756, Height: 133, Time: clipNow.AddDate(0, 0, -1)},
		{ID: "e1", Kind: clipboard.KindFiles, Files: []string{"/x/a.pdf", "/x/b.png", "/x/c"}, Time: clipNow.AddDate(0, 0, -5)},
	}
}

// spell spells the rows: "[section]" for a title, else the entry's ID.
func spell(rows []clipRow, es []clipboard.Entry) []string {
	var out []string
	for _, r := range rows {
		if r.title {
			out = append(out, "["+sectionTitle(r.section)+"]")
		} else {
			out = append(out, es[r.entry].ID)
		}
	}
	return out
}

func TestClipRows(t *testing.T) {
	inLocale(t, "en")
	es := history()
	rows := clipRows(es, clipNow)
	want := []string{"[Pinned]", "p1", "[Today]", "t1", "t2", "[Yesterday]", "y1", "[Earlier]", "e1"}
	if got := spell(rows, es); !slices.Equal(got, want) {
		t.Fatalf("rows %q, want %q", got, want)
	}
	// No title for a section without items.
	if got := spell(clipRows(es[1:3], clipNow), es[1:3]); !slices.Equal(got, []string{"[Today]", "t1", "t2"}) {
		t.Errorf("today only: %q", got)
	}
	if rows := clipRows(nil, clipNow); len(rows) != 0 {
		t.Errorf("no history: %v", rows)
	}

	// A fresh view chooses the newest unpinned item; all pinned, the first.
	if r := freshChoice(rows, es); r != 3 {
		t.Errorf("fresh choice: row %d, want 3 (t1)", r)
	}
	pinned := []clipboard.Entry{es[0]}
	if r := freshChoice(clipRows(pinned, clipNow), pinned); r != 1 {
		t.Errorf("all pinned: row %d, want 1", r)
	}
	if r := freshChoice(nil, nil); r != -1 {
		t.Errorf("empty: row %d", r)
	}

	// A query chooses its first item; cleared, the newest unpinned again
	// (design.md 2.8.1), not the first row, which is pinned.
	for _, c := range []struct {
		query string
		want  int
	}{{"ssh", 1}, {"", 3}, {"  ", 3}} {
		if got := queryChoice(rows, es, c.query); got != c.want {
			t.Errorf("queryChoice(%q) = %d, want %d", c.query, got, c.want)
		}
	}
	if r := queryChoice(nil, nil, ""); r != -1 {
		t.Errorf("queryChoice with none: %d", r)
	}

	// The first item of a section scrolls into view with its title.
	for _, c := range []struct{ chosen, first, last, want int }{
		{1, 1, 6, 0},  // the pinned title scrolled off above
		{1, 0, 6, 0},  // the title cut at the top
		{3, 3, 8, 2},  // Today's title off above
		{3, 0, 8, 3},  // in view: nothing to add
		{6, 0, 4, 6},  // below: the item, its title over it
		{4, 4, 8, 4},  // not the first of its section
		{3, 0, -1, 3}, // before the list was laid out
		{-1, 0, 8, -1},
	} {
		if got := revealRow(rows, c.chosen, c.first, c.last); got != c.want {
			t.Errorf("revealRow(%d) showing %d–%d = %d, want %d", c.chosen, c.first, c.last, got, c.want)
		}
	}

	// Moving skips the titles and stops at the ends.
	for _, c := range []struct{ from, step, want int }{
		{3, 1, 4}, {4, 1, 6}, {6, -1, 4}, {1, -1, 1}, {8, 1, 8}, {3, -1, 1}, {1, 3, 6}, {-1, 1, 1},
	} {
		if got := moveChoice(rows, c.from, c.step); got != c.want {
			t.Errorf("moveChoice(%d, %d) = %d, want %d", c.from, c.step, got, c.want)
		}
	}

	// Deleting chooses the next item, or the one before the last.
	for _, c := range []struct {
		row  int
		want string
	}{{3, "t2"}, {4, "y1"}, {8, "y1"}, {1, "t1"}} {
		if got := nextAfterDelete(rows, es, c.row); got != c.want {
			t.Errorf("deleting row %d chooses %q, want %q", c.row, got, c.want)
		}
	}
	one := es[1:2]
	if got := nextAfterDelete(clipRows(one, clipNow), one, 1); got != "" {
		t.Errorf("deleting the only item chooses %q", got)
	}
	if r := rowOfID(rows, es, "y1"); r != 6 {
		t.Errorf("rowOfID(y1) = %d", r)
	}
}

func TestClipTitlesAndLabels(t *testing.T) {
	es := history()
	when := "Today at 14:05"
	for _, c := range []struct {
		locale string
		want   [][2]string // title, label
	}{
		{"en", [][2]string{
			{"ssh deploy@host", "ssh deploy@host, Text, " + when + ", Pinned"},
			{"周五发布清单 1.", "周五发布清单 1., Text, " + when},
			{"https://www.github.com/xcc3641/gocast", "https://www.github.com/xcc3641/gocast, Link, " + when},
			{"Image 756×133", "Image, 756 by 133, " + when},
			{"a.pdf and 2 more", "a.pdf and 2 more, Files, " + when},
		}},
		{"zh-Hans", [][2]string{
			{"ssh deploy@host", "ssh deploy@host，文字，" + when + "，已置顶"},
			{"周五发布清单 1.", "周五发布清单 1.，文字，" + when},
			{"https://www.github.com/xcc3641/gocast", "https://www.github.com/xcc3641/gocast，链接，" + when},
			{"图片 756×133", "图片，756乘133，" + when},
			{"a.pdf 等 3 个文件", "a.pdf等 3 个文件，" + when},
		}},
	} {
		inLocale(t, c.locale)
		for i, e := range es {
			if got := clipTitle(e); got != c.want[i][0] {
				t.Errorf("%s: title of %s = %q, want %q", c.locale, e.ID, got, c.want[i][0])
			}
			if got := clipLabel(e, when); got != c.want[i][1] {
				t.Errorf("%s: label of %s = %q, want %q", c.locale, e.ID, got, c.want[i][1])
			}
		}
		file := clipboard.Entry{Kind: clipboard.KindFiles, Files: []string{"/x/季度报告.pdf"}}
		if got := clipTitle(file); got != "季度报告.pdf" {
			t.Errorf("one file's title %q", got)
		}
	}
}

func TestClipActions(t *testing.T) {
	inLocale(t, "zh-Hans")
	type row struct {
		id    clipActionID
		label string
		caps  string
		div   bool
	}
	spell := func(as []clipAction) []row {
		var out []row
		for _, a := range as {
			out = append(out, row{a.id, a.label, strings.Join(a.caps, ""), a.divided})
		}
		return out
	}
	text := clipboard.Entry{Kind: clipboard.KindText, Text: "hi"}
	got := spell(clipActions(text, true, appTarget("备忘录")))
	want := []row{
		{actPaste, "粘贴到“备忘录”", "↩", false},
		{actPastePlain, "粘贴为纯文本", "", false},
		{actCopy, "拷贝到剪贴板", "⌘↩", false},
		{actPin, "置顶", "⌘.", true},
		{actDelete, "删除", "⌃X", true},
		{actClear, "清空历史…", "⌃⇧X", false},
	}
	if !slices.Equal(got, want) {
		t.Errorf("text, may paste:\n got %v\nwant %v", got, want)
	}
	if as := clipActions(text, true, appTarget("备忘录")); as[1].symbol != "doc.plaintext" {
		t.Errorf("Paste as Plain Text shows %q, want doc.plaintext (design.md 2.8)", as[1].symbol)
	}
	// Without the permission, copying is first, on Return; a link opens;
	// a pinned item unpins.
	link := clipboard.Entry{Kind: clipboard.KindLink, Text: "https://a.b", Pinned: true}
	got = spell(clipActions(link, false, appTarget("Safari")))
	want = []row{
		{actCopy, "拷贝到剪贴板", "↩", false},
		{actOpenLink, "打开链接", "⌘O", false},
		{actPin, "取消置顶", "⌘.", true},
		{actDelete, "删除", "⌃X", true},
		{actClear, "清空历史…", "⌃⇧X", false},
	}
	if !slices.Equal(got, want) {
		t.Errorf("pinned link, may not paste:\n got %v\nwant %v", got, want)
	}
	// An unknown app: Paste. Images don't paste as plain text. Files show
	// in Finder while there.
	as := clipActions(clipboard.Entry{Kind: clipboard.KindImage}, true, pasteTarget{})
	if as[0].label != "粘贴" || as[1].id != actCopy {
		t.Errorf("image, unknown app: %v", spell(as))
	}
	defer func(f func(string) bool) { fileExists = f }(fileExists)
	fileExists = func(p string) bool { return p == "/here" }
	has := func(as []clipAction, id clipActionID) bool {
		return slices.ContainsFunc(as, func(a clipAction) bool { return a.id == id })
	}
	if !has(clipActions(clipboard.Entry{Kind: clipboard.KindFiles, Files: []string{"/here"}}, true, appTarget("x")), actShowInFinder) {
		t.Error("a file there: no Show in Finder")
	}
	if has(clipActions(clipboard.Entry{Kind: clipboard.KindFiles, Files: []string{"/gone"}}, true, appTarget("x")), actShowInFinder) {
		t.Error("a file gone: Show in Finder")
	}
	// The floating terminal: its own label, in the ⌘K menu too.
	if as := clipActions(text, true, pasteTarget{floating: true}); as[0].label != "粘贴到悬浮终端" {
		t.Errorf("floating terminal: %v", spell(as))
	}
	inLocale(t, "en")
	if got := pasteLabel("Notes"); got != "Paste to Notes" {
		t.Errorf("pasteLabel = %q", got)
	}
	if got := (pasteTarget{floating: true}).label("Gocast"); got != "Paste to Floating Terminal" {
		t.Errorf("floating terminal label = %q", got)
	}
	if got := (pasteTarget{app: clipboard.App{BundleID: "com.apple.Terminal", Name: "Terminal"}}).label("Terminal"); got != "Paste to Terminal" {
		t.Errorf("Terminal label = %q", got)
	}
}

func appTarget(name string) pasteTarget { return pasteTarget{app: clipboard.App{Name: name}} }

func TestPasteWay(t *testing.T) {
	app := func(id string) pasteTarget { return pasteTarget{app: clipboard.App{BundleID: id, Name: id}} }
	terminals := []pasteTarget{
		{floating: true},
		app("com.apple.Terminal"), app("com.googlecode.iterm2"), app("com.mitchellh.ghostty"),
		app("dev.warp.Warp-Stable"), app("net.kovidgoyal.kitty"), app("org.alacritty"),
		app("com.github.wez.wezterm"),
	}
	others := []pasteTarget{{}, app("com.apple.Notes"), app("com.apple.Safari"), app("com.microsoft.VSCode"), app("com.apple.terminal")}
	kinds := []clipboard.Kind{clipboard.KindText, clipboard.KindRichText, clipboard.KindLink, clipboard.KindImage, clipboard.KindFiles}
	for _, tg := range terminals {
		if !tg.terminal() {
			t.Errorf("%+v: not a terminal", tg)
		}
	}
	for _, tg := range others {
		if tg.terminal() {
			t.Errorf("%+v: a terminal", tg)
		}
	}
	for _, k := range kinds {
		for _, plain := range []bool{false, true} {
			for _, tg := range terminals {
				want := pasteAll
				switch {
				case k == clipboard.KindImage:
					want = pasteImagePath
				case plain:
					want = pastePlainText
				}
				if got := pasteWayFor(k, plain, tg); got != want {
					t.Errorf("%s, plain %v, into %+v: %d, want %d", k, plain, tg, got, want)
				}
			}
			for _, tg := range others {
				want := pasteAll
				if plain && k != clipboard.KindImage {
					want = pastePlainText
				}
				if got := pasteWayFor(k, plain, tg); got != want {
					t.Errorf("%s, plain %v, into %+v: %d, want %d", k, plain, tg, got, want)
				}
			}
		}
	}
	if len(terminalApps) != 7 {
		t.Errorf("%d terminals", len(terminalApps))
	}
}

func TestClipState(t *testing.T) {
	for _, c := range []struct {
		loaded, unreadable, recording bool
		total                         int
		query                         string
		filter                        clipboard.Filter
		shown                         int
		want                          clipState
	}{
		{false, false, true, 0, "", 0, 0, clipLoading},
		{true, true, true, 3, "", 0, 0, clipUnreadable},
		{true, false, false, 0, "", 0, 0, clipOffNone},
		{true, false, true, 0, "", 0, 0, clipNone},
		{true, false, true, 0, "x", 0, 0, clipNone},
		{true, false, false, 4, "", 0, 4, clipShowList},
		{true, false, true, 4, "zzz", 0, 0, clipNoResults},
		{true, false, true, 4, "", clipboard.FilterImages, 0, clipNoneOfKind},
		{true, false, true, 4, "  ", 0, 0, clipNone},
	} {
		if got := clipStateOf(c.loaded, c.unreadable, c.recording, c.total, c.query, c.filter, c.shown); got != c.want {
			t.Errorf("%+v: state %v, want %v", c, got, c.want)
		}
	}
	// Off wins over the permission; the permission shows once asked, with
	// pasting on and not allowed, over the list.
	for _, c := range []struct {
		state                                      clipState
		recording, asked, onReturn, trusted, ready bool
		want                                       int
	}{
		{clipShowList, false, true, true, false, false, bannerOff},
		{clipShowList, true, true, true, false, false, bannerAX},
		{clipShowList, true, false, true, false, false, bannerNone},
		{clipShowList, true, true, false, false, false, bannerNone},
		{clipShowList, true, true, true, true, false, bannerNone},
		{clipShowList, true, false, true, true, true, bannerReady},
		{clipOffNone, false, false, true, false, false, bannerNone},
	} {
		if got := clipBanner(c.state, c.recording, c.asked, c.onReturn, c.trusted, c.ready); got != c.want {
			t.Errorf("%+v: banner %d, want %d", c, got, c.want)
		}
	}
}

func TestClipInfos(t *testing.T) {
	inLocale(t, "zh-Hans")
	spell := func(in []clipInfo) []string {
		var out []string
		for _, i := range in {
			out = append(out, i.label+"="+i.value)
		}
		return out
	}
	size := func(files []string) (int64, bool) { return 84_000, len(files) == 1 }
	at := clipNow.Add(-55 * time.Minute)
	text := clipboard.Entry{Kind: clipboard.KindText, Text: "发布清单 ok", App: clipboard.App{Name: "备忘录"}, Time: at}
	got := spell(clipInfos(text, clipNow, "zh-Hans", size))
	if len(got) != 3 || got[0] != "来源=备忘录" || !strings.HasPrefix(got[1], "拷贝时间=今天") || got[2] != "字数=7" {
		t.Errorf("text: %q", got)
	}
	link := clipboard.Entry{Kind: clipboard.KindLink, Text: "https://www.github.com/x", Time: at}
	got = spell(clipInfos(link, clipNow, "zh-Hans", size))
	if got[0] != "来源=未知应用" || got[2] != "链接=github.com" {
		t.Errorf("link: %q", got)
	}
	img := clipboard.Entry{Kind: clipboard.KindImage, Width: 756, Height: 133, Size: 84_000, Time: at}
	got = spell(clipInfos(img, clipNow, "zh-Hans", size))
	if len(got) != 4 || got[2] != "尺寸=756 × 133" || !strings.HasPrefix(got[3], "大小=84") {
		t.Errorf("image: %q", got)
	}
	files := clipboard.Entry{Kind: clipboard.KindFiles, Files: []string{"/tmp/a", "/tmp/b"}, Time: at}
	got = spell(clipInfos(files, clipNow, "zh-Hans", size))
	if len(got) != 3 || got[2] != "位置=/tmp" {
		t.Errorf("files with a folder among them: %q", got)
	}
	if infos := clipInfos(text, clipNow, "zh-Hans", size); !infos[0].source || infos[1].source {
		t.Error("the source line is not marked")
	}
}

func TestClipText(t *testing.T) {
	if got := firstLine("\n \t\n  a  b\t\tc \nnext"); got != "a b c" {
		t.Errorf("firstLine = %q", got)
	}
	long := strings.Repeat("字", previewLimit+5)
	if got, cut := previewText(long); !cut || len([]rune(got)) != previewLimit {
		t.Errorf("previewText cut %v to %d", cut, len([]rune(got)))
	}
	if got, cut := previewText("short"); cut || got != "short" {
		t.Errorf("previewText(short) = %q, %v", got, cut)
	}
	measure := func(s string) float32 { return float32(len([]rune(s))) * 10 }
	if got := fitText("Visual Studio Code", 100, measure); got != "Visual St…" {
		t.Errorf("fitText = %q", got)
	}
	if got := fitText("Notes", 100, measure); got != "Notes" {
		t.Errorf("fitText(Notes) = %q", got)
	}
	for _, c := range []struct{ in, want string }{
		{"https://www.github.com/x", "github.com"},
		{"http://example.com:8080/a?b", "example.com"},
		{"not a url", "not a url"},
	} {
		if got := linkDomain(c.in); got != c.want {
			t.Errorf("linkDomain(%q) = %q", c.in, got)
		}
	}
	if w, h := fitSize(756, 133, 26, 26); w != 26 || h > 4.58 || h < 4.56 {
		t.Errorf("fitSize = %v×%v", w, h)
	}
}

func TestClipCopy(t *testing.T) {
	// copy.md 6 and 6.10.
	for _, c := range []struct{ locale, list, placeholder, all, heading, preview, typ string }{
		{"zh-Hans", "剪贴板历史", "搜索剪贴板历史", "全部类型", "今天，标题", "预览", "类型，全部类型"},
		{"en", "Clipboard History", "Search clipboard history", "All Types", "Today, heading", "Preview", "Type, All Types"},
	} {
		m := locales.In(c.locale)
		got := []string{m.ClipboardHistory(), m.ClipPlaceholder(), m.ClipFilterAll(), m.ClipHeading(m.ClipSectionToday()), m.ClipPreview(), m.ClipTypeLabel(m.ClipFilterAll())}
		if want := []string{c.list, c.placeholder, c.all, c.heading, c.preview, c.typ}; !slices.Equal(got, want) {
			t.Errorf("%s: %q, want %q", c.locale, got, want)
		}
	}
}

func TestSeedHistory(t *testing.T) {
	clock := &seedClock{}
	store := clipboard.NewMemoryStore(clipboard.Options{Pasteboard: &memPasteboard{}, Now: clock.now})
	seedHistory(store, clock, t.TempDir(), "en")
	es := store.Entries(clipboard.FilterAll)
	kinds := map[clipboard.Kind]int{}
	pinned := 0
	for _, e := range es {
		kinds[e.Kind]++
		if e.Pinned {
			pinned++
		}
	}
	if len(es) < 12 || pinned != 2 || kinds[clipboard.KindImage] != 2 || kinds[clipboard.KindFiles] != 4 || kinds[clipboard.KindRichText] != 1 || kinds[clipboard.KindLink] != 3 {
		t.Errorf("%d items, %d pinned, kinds %v", len(es), pinned, kinds)
	}
	// Newest unpinned first in today's section.
	rows := clipRows(es, time.Now())
	if r := freshChoice(rows, es); es[rows[r].entry].Kind != clipboard.KindText || !strings.HasPrefix(es[rows[r].entry].Text, "周五发布清单") {
		t.Errorf("fresh choice: %+v", es[rows[r].entry])
	}
}

func TestMemPasteboard(t *testing.T) {
	p := &memPasteboard{}
	if err := p.Write(clipboard.Content{Text: "x"}); err != nil || p.ChangeCount() != 1 {
		t.Fatalf("write: %v, count %d", err, p.ChangeCount())
	}
	if c, _ := p.Read(); c.Text != "x" {
		t.Errorf("read %q", c.Text)
	}
}

func TestClipCardColors(t *testing.T) {
	// design.md 2.5 and 2.8: the cards' border is the capsules'; the clear
	// confirmation's scrim thicker in dark mode.
	for _, c := range []struct {
		dark          bool
		border, scrim ui.Color
	}{
		{false, ui.RGBA(0, 0, 0, 0.08), ui.RGBA(0, 0, 0, 0.12)},
		{true, ui.RGBA(255, 255, 255, 0.12), ui.RGBA(0, 0, 0, 0.30)},
	} {
		k := tokensOf(c.dark, false)
		if k.capsuleBorder != c.border || k.scrim != c.scrim {
			t.Errorf("dark %v: border %v, scrim %v", c.dark, k.capsuleBorder, k.scrim)
		}
	}
}
