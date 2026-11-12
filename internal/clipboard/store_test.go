package clipboard

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeBoard is a pasteboard in memory.
type fakeBoard struct {
	mu      sync.Mutex
	count   int
	types   []string
	content Content
	writes  []Content
	reads   int
}

func (b *fakeBoard) ChangeCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.count
}

func (b *fakeBoard) Types() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.types
}

func (b *fakeBoard) Read() (Content, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.reads++
	return b.content, nil
}

func (b *fakeBoard) Write(c Content) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.count++
	b.content, b.types = c, []string{"public.utf8-plain-text"}
	b.writes = append(b.writes, c)
	return nil
}

// copy is the user copying c in another app.
func (b *fakeBoard) copy(c Content, types ...string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.count++
	b.content, b.types = c, types
}

// clock is a settable time.
type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newClock() *clock { return &clock{t: time.Date(2026, 11, 12, 15, 0, 0, 0, time.Local)} }

func memStore(t *testing.T) (*Store, *fakeBoard, *clock) {
	t.Helper()
	b, c := &fakeBoard{}, newClock()
	return NewMemoryStore(Options{Pasteboard: b, Now: c.now}), b, c
}

func testPNG(t testing.TB, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.NRGBA{uint8(x), uint8(y), 200, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

var safari = App{BundleID: "com.apple.Safari", Name: "Safari"}

func TestKinds(t *testing.T) {
	img := testPNG(t, 4, 3)
	for _, tc := range []struct {
		c    Content
		want Kind
	}{
		{Content{Text: "hello"}, KindText},
		{Content{Text: "hello", RTF: []byte("{\\rtf1 hello}")}, KindRichText},
		{Content{Text: "hello", HTML: []byte("<b>hello</b>")}, KindRichText},
		{Content{HTML: []byte("<p>from <b>HTML</b> only</p>")}, KindRichText},
		{Content{Text: " https://github.com/egoist/mygo?x=1 \n"}, KindLink},
		{Content{Text: "https://a.com https://b.com"}, KindText},
		{Content{Text: "ftp://a.com/x"}, KindText},
		{Content{Text: "github.com"}, KindText},
		{Content{Text: "https://example.com", HTML: []byte("<a>x</a>")}, KindLink},
		{Content{PNG: img}, KindImage},
		{Content{PNG: img, HTML: []byte(`<img src="x.png">`)}, KindImage},
		{Content{Text: "caption", PNG: img}, KindText},
		{Content{Files: []string{"/a", "/b"}, Text: "a\nb", PNG: img}, KindFiles},
		{Content{Text: " \n\t"}, ""},
		{Content{}, ""},
	} {
		c, k := normalize(tc.c)
		if k != tc.want {
			t.Errorf("normalize(%+v) kind = %q, want %q", tc.c, k, tc.want)
		}
		if k == KindImage && (c.Text != "" || c.HTML != nil) {
			t.Errorf("image kept text: %+v", c)
		}
		if k == KindText && c.PNG != nil {
			t.Errorf("text kept image")
		}
	}
	if got := HTMLText([]byte("<style>p{}</style><p>a &amp; b</p><p>c</p>")); got != "a & b\nc" {
		t.Errorf("HTMLText = %q", got)
	}
}

func TestAddDedup(t *testing.T) {
	s, _, clk := memStore(t)
	a, err := s.Add(Content{Text: "first"}, safari)
	if err != nil {
		t.Fatal(err)
	}
	clk.t = clk.t.Add(time.Minute)
	if _, err := s.Add(Content{Text: "second"}, safari); err != nil {
		t.Fatal(err)
	}
	clk.t = clk.t.Add(time.Minute)
	notes := App{BundleID: "com.apple.Notes", Name: "Notes"}
	again, err := s.Add(Content{Text: "first", RTF: []byte("{\\rtf1 first}")}, notes)
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != a.ID || again.Kind != KindRichText || again.App != notes || !again.Time.Equal(clk.t) {
		t.Errorf("re-copy = %+v, want the first entry, rich, from Notes, now", again)
	}
	got := s.Entries(FilterAll)
	if len(got) != 2 || got[0].ID != a.ID || got[1].Text != "second" {
		t.Fatalf("entries = %+v", got)
	}
	c, err := s.Content(a.ID)
	if err != nil || string(c.RTF) != "{\\rtf1 first}" {
		t.Errorf("content = %+v, %v", c, err)
	}

	if _, err := s.Add(Content{Text: "  "}, safari); !errors.Is(err, ErrEmpty) {
		t.Errorf("blank: %v", err)
	}
	if _, err := s.Add(Content{Text: strings.Repeat("x", MaxSize+1)}, safari); !errors.Is(err, ErrTooLarge) {
		t.Errorf("large: %v", err)
	}
	img := testPNG(t, 120, 60)
	e, err := s.Add(Content{PNG: img}, safari)
	if err != nil || e.Width != 120 || e.Height != 60 || e.Size != int64(len(img)) {
		t.Fatalf("image = %+v, %v", e, err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(e.Thumbnail))
	if err != nil || cfg.Width != 52 || cfg.Height != 26 {
		t.Errorf("thumbnail %v: %dx%d", err, cfg.Width, cfg.Height)
	}
	e2, _ := s.Add(Content{PNG: img}, safari)
	if e2.ID != e.ID || s.Len() != 3 {
		t.Errorf("same image twice: %d entries", s.Len())
	}
	f, _ := s.Add(Content{Files: []string{"/tmp/a.txt", "/tmp/b.txt"}}, safari)
	if f.Kind != KindFiles || f.Text != "/tmp/a.txt\n/tmp/b.txt" || len(f.Files) != 2 {
		t.Errorf("files = %+v", f)
	}
}

func TestSearchAndOrder(t *testing.T) {
	s, _, clk := memStore(t)
	wechat := App{BundleID: "com.tencent.xinWeChat", Name: "微信"}
	add := func(c Content, app App) Entry {
		t.Helper()
		clk.t = clk.t.Add(time.Second)
		e, err := s.Add(c, app)
		if err != nil {
			t.Fatal(err)
		}
		return e
	}
	text := add(Content{Text: "Hello World"}, safari)
	zh := add(Content{Text: "今天下午三点开会"}, wechat)
	link := add(Content{Text: "https://GitHub.com/egoist/mygo"}, safari)
	files := add(Content{Files: []string{"/Users/me/Documents/Report Q3.pdf"}}, App{Name: "Finder"})
	img := add(Content{PNG: testPNG(t, 2, 2)}, App{Name: "Preview"})
	if err := s.SetPinned(text.ID, true); err != nil {
		t.Fatal(err)
	}

	ids := func(es []Entry) []string {
		var out []string
		for _, e := range es {
			out = append(out, e.ID)
		}
		return out
	}
	check := func(query string, f Filter, want ...Entry) {
		t.Helper()
		got := ids(s.Search(query, f))
		if fmt.Sprint(got) != fmt.Sprint(ids(want)) {
			t.Errorf("Search(%q, %d) = %v, want %v", query, f, got, ids(want))
		}
	}
	check("", FilterAll, text, img, files, link, zh)
	check("hello", FilterAll, text)
	check("WORLD hello", FilterAll, text)
	check("下午三点", FilterAll, zh)
	check("微信", FilterAll, zh)
	check("xiawu", FilterAll)
	check("github", FilterAll, link)
	check("report q3", FilterAll, files)
	check("preview", FilterAll, img)
	check("", FilterText, text, zh)
	check("", FilterLinks, link)
	check("", FilterImages, img)
	check("", FilterFiles, files)
	if i := FirstUnpinned(s.Entries(FilterAll)); i != 1 {
		t.Errorf("FirstUnpinned = %d", i)
	}
	if i := FirstUnpinned(nil); i != -1 {
		t.Errorf("FirstUnpinned(nil) = %d", i)
	}
	if i := FirstUnpinned(s.Search("hello", FilterAll)); i != 0 {
		t.Errorf("FirstUnpinned(pinned only) = %d", i)
	}

	// Touch moves to the top; pinned stay first.
	clk.t = clk.t.Add(time.Second)
	if err := s.Touch(zh.ID); err != nil {
		t.Fatal(err)
	}
	check("", FilterAll, text, zh, img, files, link)
}

func TestSearch1000Fast(t *testing.T) {
	s, _, clk := memStore(t)
	for i := 0; i < 1000; i++ {
		clk.t = clk.t.Add(time.Second)
		text := fmt.Sprintf("entry %d: %s 中文内容 %d", i, strings.Repeat("lorem ipsum dolor sit amet ", 40), i*7)
		if _, err := s.Add(Content{Text: text}, safari); err != nil {
			t.Fatal(err)
		}
	}
	start := time.Now()
	for i := 0; i < 20; i++ {
		s.Search("中文 ZZZ", FilterAll)
		s.Search("dolor", FilterAll)
	}
	if per := time.Since(start) / 40; per > 10*time.Millisecond {
		t.Errorf("a search over 1000 entries takes %v", per)
	}
	if got := s.Search("entry 999:", FilterAll); len(got) != 1 {
		t.Errorf("found %d", len(got))
	}
}

func BenchmarkSearch1000(b *testing.B) {
	s := NewMemoryStore(Options{Pasteboard: &fakeBoard{}})
	for i := 0; i < 1000; i++ {
		s.Add(Content{Text: fmt.Sprintf("entry %d %s", i, strings.Repeat("some words to search through ", 30))}, safari)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Search("through zz", FilterAll)
	}
}

func TestRetentionAndCap(t *testing.T) {
	b, clk := &fakeBoard{}, newClock()
	s := NewMemoryStore(Options{Pasteboard: b, Now: clk.now, MaxEntries: 3, Retention: RetainWeek})
	old, _ := s.Add(Content{Text: "old"}, safari)
	pinned, _ := s.Add(Content{Text: "pinned"}, safari)
	s.SetPinned(pinned.ID, true)
	clk.t = clk.t.Add(8 * 24 * time.Hour)
	if got := s.Entries(FilterAll); len(got) != 1 || got[0].ID != pinned.ID {
		t.Errorf("expired entries listed: %+v", got)
	}
	if n := s.Prune(); n != 1 {
		t.Errorf("pruned %d", n)
	}
	if _, ok := s.Get(old.ID); ok {
		t.Error("old entry kept")
	}
	for i := 0; i < 5; i++ {
		clk.t = clk.t.Add(time.Second)
		s.Add(Content{Text: fmt.Sprint("n", i)}, safari)
	}
	got := s.Entries(FilterAll)
	if len(got) != 4 || got[0].ID != pinned.ID || got[1].Text != "n4" || got[3].Text != "n2" {
		t.Errorf("capped = %+v", got)
	}
	s.SetRetention(RetainForever)
	clk.t = clk.t.Add(10 * 365 * 24 * time.Hour)
	if n := len(s.Entries(FilterAll)); n != 4 {
		t.Errorf("forever kept %d", n)
	}
	s.SetRetention(RetainDay)
	if n := s.Len(); n != 1 {
		t.Errorf("after a day: %d", n)
	}
	for _, r := range Retentions {
		if r != RetainForever && r.Duration() <= 0 {
			t.Errorf("%s: %v", r, r.Duration())
		}
	}
	if Retention("bogus").Duration() != RetainMonth.Duration() {
		t.Error("unknown retention is not a month")
	}
}

func TestDeleteAndClear(t *testing.T) {
	s, _, _ := memStore(t)
	if s.CanClear() {
		t.Error("CanClear on empty")
	}
	a, _ := s.Add(Content{Text: "a"}, safari)
	p, _ := s.Add(Content{Text: "p"}, safari)
	s.SetPinned(p.ID, true)
	if !s.CanClear() {
		t.Error("cannot clear")
	}
	changes := 0
	s.SetOnChange(func() { changes++ })
	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}
	if got := s.Entries(FilterAll); len(got) != 1 || got[0].ID != p.ID {
		t.Errorf("after Clear: %+v", got)
	}
	if s.CanClear() {
		t.Error("CanClear with only pinned left")
	}
	if err := s.Delete(a.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete cleared: %v", err)
	}
	if err := s.Delete(p.ID); err != nil || s.Len() != 0 {
		t.Errorf("delete pinned: %v, %d left", err, s.Len())
	}
	if changes != 2 {
		t.Errorf("%d change notices", changes)
	}
}

func TestSections(t *testing.T) {
	now := time.Date(2026, 11, 12, 0, 30, 0, 0, time.Local)
	for _, tc := range []struct {
		t      time.Time
		pinned bool
		want   Section
	}{
		{now.Add(-10 * time.Minute), false, SectionToday},
		{now.Add(-40 * time.Minute), false, SectionYesterday},
		{now.Add(-24 * time.Hour), false, SectionYesterday},
		{now.Add(-25 * time.Hour), false, SectionEarlier},
		{now.Add(-25 * time.Hour), true, SectionPinned},
	} {
		if got := SectionOf(Entry{Time: tc.t, Pinned: tc.pinned}, now); got != tc.want {
			t.Errorf("SectionOf(%v) = %v, want %v", tc.t, got, tc.want)
		}
	}
}

func TestCopyAndPaste(t *testing.T) {
	s, b, clk := memStore(t)
	rich, _ := s.Add(Content{Text: "bold", RTF: []byte("{\\rtf1 \\b bold}"), HTML: []byte("<b>bold</b>")}, safari)
	clk.t = clk.t.Add(time.Minute)
	s.Add(Content{Text: "newer"}, safari)

	defer func(t func() bool, k func() error) { trusted, keystroke = t, k }(trusted, keystroke)
	allowed, pressed := false, 0
	trusted = func() bool { return allowed }
	keystroke = func() error { pressed++; return nil }

	clk.t = clk.t.Add(time.Minute)
	if err := s.CopyToPasteboard(rich.ID); err != nil {
		t.Fatal(err)
	}
	w := b.writes[len(b.writes)-1]
	if w.Text != "bold" || string(w.RTF) != "{\\rtf1 \\b bold}" || string(w.HTML) != "<b>bold</b>" {
		t.Errorf("wrote %+v", w)
	}
	if got := s.Entries(FilterAll); got[0].ID != rich.ID || !got[0].Time.Equal(clk.t) {
		t.Errorf("not moved to the top: %+v", got[0])
	}
	if pressed != 0 {
		t.Error("copy pressed ⌘V")
	}

	if err := s.Paste(rich.ID); !errors.Is(err, ErrNoAccessibility) || pressed != 0 {
		t.Errorf("untrusted paste: %v, %d", err, pressed)
	}
	allowed = true
	if err := s.Paste(rich.ID); err != nil || pressed != 1 {
		t.Errorf("paste: %v, %d", err, pressed)
	}
	if err := s.PasteAsPlainText(rich.ID); err != nil || pressed != 2 {
		t.Errorf("plain paste: %v", err)
	}
	if w := b.writes[len(b.writes)-1]; w.Text != "bold" || w.RTF != nil || w.HTML != nil {
		t.Errorf("plain wrote %+v", w)
	}
	img, _ := s.Add(Content{PNG: testPNG(t, 2, 2)}, safari)
	if err := s.PasteAsPlainText(img.ID); !errors.Is(err, ErrNoText) {
		t.Errorf("plain image: %v", err)
	}
	gone, _ := s.Add(Content{Files: []string{filepath.Join(t.TempDir(), "gone.txt")}}, safari)
	if err := s.CopyToPasteboard(gone.ID); !errors.Is(err, ErrFileMissing) {
		t.Errorf("missing file: %v", err)
	}
	if err := s.Paste("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown: %v", err)
	}
	if s.Len() != 4 {
		t.Errorf("%d entries", s.Len())
	}
}

func TestWatcher(t *testing.T) {
	s, b, clk := memStore(t)
	front := safari
	w := NewWatcher(s, WatcherOptions{Frontmost: func() App { return front }})
	b.copy(Content{Text: "before start"}, "public.utf8-plain-text")
	w.last = b.ChangeCount() // as Start does

	if w.Poll() || b.reads != 0 {
		t.Error("read an unchanged pasteboard")
	}
	b.copy(Content{Text: "one"}, "public.utf8-plain-text")
	if !w.Poll() || s.Len() != 1 {
		t.Fatal("did not record a copy")
	}
	if w.Poll() || b.reads != 1 {
		t.Error("read twice for one change")
	}

	// Marked contents are not read.
	for _, mark := range []string{"org.nspasteboard.ConcealedType", "org.nspasteboard.TransientType", "org.nspasteboard.AutoGeneratedType"} {
		b.copy(Content{Text: "secret " + mark}, "public.utf8-plain-text", mark)
		if w.Poll() {
			t.Errorf("recorded %s", mark)
		}
	}
	if b.reads != 1 {
		t.Errorf("read marked contents (%d reads)", b.reads)
	}

	// Ignored apps.
	front = App{BundleID: "com.1password.1password", Name: "1Password"}
	b.copy(Content{Text: "hunter2"}, "public.utf8-plain-text")
	if w.Poll() {
		t.Error("recorded from an ignored app")
	}
	w.SetIgnoredApps([]string{"com.apple.Safari"})
	front = safari
	b.copy(Content{Text: "from safari"}, "public.utf8-plain-text")
	if w.Poll() {
		t.Error("recorded from a newly ignored app")
	}
	w.SetIgnoredApps(nil)

	// Off.
	w.SetEnabled(false)
	b.copy(Content{Text: "while off"}, "public.utf8-plain-text")
	if w.Poll() {
		t.Error("recorded while off")
	}
	w.SetEnabled(true)
	if w.Poll() {
		t.Error("recorded what was copied while off")
	}

	// Own writes are not recorded again.
	clk.t = clk.t.Add(time.Minute)
	b.copy(Content{Text: "two"}, "public.utf8-plain-text")
	w.Poll()
	one := s.Search("one", FilterAll)[0]
	clk.t = clk.t.Add(time.Minute)
	reads := b.reads
	if err := s.CopyToPasteboard(one.ID); err != nil {
		t.Fatal(err)
	}
	if w.Poll() || b.reads != reads {
		t.Error("recorded or read the store's own write")
	}
	if got := s.Entries(FilterAll); len(got) != 2 || got[0].ID != one.ID {
		t.Errorf("entries = %+v", got)
	}

	// A copy of the same content elsewhere moves the entry up.
	b.copy(Content{Text: "two"}, "public.utf8-plain-text")
	if !w.Poll() || s.Len() != 2 || s.Entries(FilterAll)[0].Text != "two" {
		t.Errorf("re-copy: %+v", s.Entries(FilterAll))
	}

	// Empty and oversized contents.
	b.copy(Content{}, "public.utf8-plain-text")
	b2 := b.ChangeCount()
	if w.Poll() || w.last != b2 {
		t.Error("empty contents")
	}

	// A write under way: wait for it.
	s.selfWriting.Add(1)
	b.copy(Content{Text: "three"}, "public.utf8-plain-text")
	if w.Poll() {
		t.Error("polled during a write")
	}
	s.selfWriting.Add(-1)
	if !w.Poll() {
		t.Error("missed the change after a write")
	}
}

func TestWatcherRuns(t *testing.T) {
	s, b, _ := memStore(t)
	w := NewWatcher(s, WatcherOptions{Interval: 5 * time.Millisecond, Frontmost: func() App { return safari }})
	added := make(chan struct{}, 1)
	s.SetOnChange(func() {
		select {
		case added <- struct{}{}:
		default:
		}
	})
	w.Start()
	defer w.Stop()
	b.copy(Content{Text: "async"}, "public.utf8-plain-text")
	select {
	case <-added:
	case <-time.After(2 * time.Second):
		t.Fatal("the watcher did not record a copy")
	}
	w.Stop()
	w.Stop()
	if got := s.Entries(FilterAll); len(got) != 1 || got[0].Text != "async" {
		t.Errorf("entries = %+v", got)
	}
	if !slicesEqual(NewWatcher(s, WatcherOptions{}).IgnoredApps(), DefaultIgnoredApps) {
		t.Error("default ignored apps")
	}
}

func slicesEqual(a, b []string) bool { return fmt.Sprint(a) == fmt.Sprint(b) }

func TestMain(m *testing.M) {
	// The hooks must never press real keys in tests.
	keystroke = func() error { panic("a test pressed ⌘V") }
	trusted = func() bool { return false }
	os.Exit(m.Run())
}
