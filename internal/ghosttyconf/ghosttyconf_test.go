package ghosttyconf

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// home is a home folder of the test's own, with the files of files under
// it, by their path from it, and the env that reads it; its Ghostty
// resources in res.
func home(t *testing.T, files map[string]string) Env {
	t.Helper()
	dir := t.TempDir()
	for name, data := range files {
		write(t, filepath.Join(dir, name), data)
	}
	return Env{Home: dir, Resources: []string{filepath.Join(dir, "res")}}
}

func write(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

const (
	xdgLegacy  = ".config/ghostty/config"
	xdgFile    = ".config/ghostty/config.ghostty"
	appLegacy  = "Library/Application Support/com.mitchellh.ghostty/config"
	appSupport = "Library/Application Support/com.mitchellh.ghostty/config.ghostty"
)

func col(h string) *Color {
	c, ok := ParseColor(h)
	if !ok {
		panic(h)
	}
	return &c
}

// No files, empty ones, or ones of comments and other keys set nothing of
// the appearance, and nothing unrecognized.
func TestLoadNothing(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"none":   nil,
		"empty":  {xdgFile: ""},
		"others": {xdgFile: "# a comment\n\n   # indented\nkeybind = ctrl+a=new_tab\nquick-terminal-position = top\nmacos-titlebar-style = hidden\nnot-a-key = 1\n"},
	} {
		r := Load(home(t, files))
		if r.Light != r.Dark || r.Conditional || r.Light.Applied() || len(r.Light.Unrecognized) > 0 {
			t.Errorf("%s: %+v, conditional %v", name, r.Light, r.Conditional)
		}
	}
}

// Each appearance key is read as Ghostty reads it: quotes dropped,
// whitespace and CRs trimmed, a byte order mark skipped, comments only on
// lines of their own.
func TestLoadKeys(t *testing.T) {
	env := home(t, map[string]string{xdgFile: "\xEF\xBB\xBFfont-family = \"Fira Code\"\r\n" +
		"  font-family=Menlo  \r\n" +
		"font-size = 15.5\n" +
		"background = #1e1e2e\n" +
		"foreground = cdd6f4\n" +
		"palette = 0=#45475a\n" +
		"palette = 0x0f = #abc\n" +
		"palette = 200=#ffffff\n" +
		"cursor-color = #f5e0dc\n" +
		"cursor-text = #111111\n" +
		"cursor-style = bar\n" +
		"cursor-style-blink = false\n" +
		"selection-background = #585b70\n" +
		"selection-foreground = #cdd6f4\n" +
		"window-padding-x = 24\n" +
		"window-padding-y = 3, 9\n" +
		"background-opacity = 0.85\n" +
		"adjust-cell-height = 10%\n"}) // not the appearance's here: skipped
	c := Load(env).Light
	want := &Config{
		FontFamily: []string{"Fira Code", "Menlo"}, FontSize: 15.5,
		Background: col("#1e1e2e"), Foreground: col("#cdd6f4"),
		CursorColor: col("#f5e0dc"), CursorText: col("#111111"),
		SelectionBackground: col("#585b70"), SelectionForeground: col("#cdd6f4"),
		CursorStyle: CursorBar, CursorBlink: new(bool),
		PaddingX: &Padding{24, 24}, PaddingY: &Padding{3, 9},
		BackgroundOpacity: ptr(0.85),
	}
	want.Palette[0], want.Palette[15] = col("#45475a"), col("#aabbcc")
	if !reflect.DeepEqual(c, want) {
		t.Errorf("got  %+v\nwant %+v", c, want)
	}
	if !c.Applied() {
		t.Error("not applied")
	}
}

func ptr[T any](v T) *T { return &v }

// A value that can't be used leaves the one before, and its key is
// unrecognized, unless a later line sets or resets it; a key alone is a
// flag; values out of range are clamped or kept as Ghostty keeps them.
func TestLoadBadValues(t *testing.T) {
	c := Load(home(t, map[string]string{xdgFile: "background = #102030\n" +
		"background = black\n" + // an X11 name: not read
		"foreground = #12345\n" +
		"cursor-color = cell-foreground\n" +
		"cursor-style = beam\n" +
		"font-size = big\n" +
		"font-size = -3\n" +
		"window-padding-x = 1,2,3\n" +
		"palette = 3=nope\n" +
		"palette = 999=#ffffff\n" +
		"background-opacity = 3\n" +
		"selection-background = nope\n" +
		"selection-background = #010101\n" +
		"cursor-style-blink\n" +
		"background = # inline comments aren't comments\n"})).Light
	if *c.Background != *col("#102030") || c.Foreground != nil || c.CursorColor != nil || c.CursorStyle != "" || c.FontSize != 0 ||
		c.PaddingX != nil || c.Palette[3] != nil {
		t.Errorf("bad values kept: %+v", c)
	}
	if *c.BackgroundOpacity != 1 || *c.SelectionBackground != *col("#010101") || !*c.CursorBlink {
		t.Errorf("opacity %v, selection %v, blink %v", *c.BackgroundOpacity, c.SelectionBackground, *c.CursorBlink)
	}
	want := []string{"background", "cursor-color", "cursor-style", "font-size", "foreground", "palette", "palette=3", "window-padding-x"}
	if !reflect.DeepEqual(c.Unrecognized, want) {
		t.Errorf("unrecognized %v, want %v", c.Unrecognized, want)
	}
}

// key = with nothing resets the key to its default, and so the font
// families; = "" too.
func TestLoadReset(t *testing.T) {
	c := Load(home(t, map[string]string{xdgFile: "font-family = A\nfont-family = B\nfont-family = \"\"\nfont-family = C\n" +
		"background = #111111\nbackground =\n" +
		"palette = 1=#111111\npalette = 2=bad\npalette =\npalette = 3=#333333\n" +
		"cursor-style = bar\ncursor-style = \n" +
		"font-size = 20\nfont-size=\n"})).Light
	want := &Config{FontFamily: []string{"C"}}
	want.Palette[3] = col("#333333")
	if !reflect.DeepEqual(c, want) {
		t.Errorf("got %+v, want %+v", c, want)
	}
}

// The four files are read in Ghostty's order, each over the one before.
func TestLoadPrecedence(t *testing.T) {
	files := map[string]string{
		xdgLegacy:  "font-size = 11\nbackground = #000001\nfont-family = A\n",
		xdgFile:    "font-size = 12\nforeground = #000002\nfont-family = B\n",
		appLegacy:  "font-size = 14\ncursor-color = #000003\nfont-family = C\n",
		appSupport: "font-size = 15\nbackground = #000004\n",
	}
	c := Load(home(t, files)).Light
	if c.FontSize != 15 || *c.Background != *col("#000004") || *c.Foreground != *col("#000002") || *c.CursorColor != *col("#000003") ||
		!reflect.DeepEqual(c.FontFamily, []string{"A", "B", "C"}) {
		t.Errorf("%+v", c)
	}
	delete(files, appSupport)
	if c := Load(home(t, files)).Light; c.FontSize != 14 || *c.Background != *col("#000001") {
		t.Errorf("without config.ghostty: %+v", c)
	}
	// XDG_CONFIG_HOME moves the first two.
	env := home(t, map[string]string{"elsewhere/ghostty/config": "font-size = 17\n", xdgFile: "font-size = 12\n"})
	env.XDGConfigHome = filepath.Join(env.Home, "elsewhere")
	if c := Load(env).Light; c.FontSize != 17 {
		t.Errorf("XDG_CONFIG_HOME: font-size %v, want 17", c.FontSize)
	}
}

// config-file includes a file relative to the file naming it, or the home
// folder; after the four files, in the order named, each once, a missing
// one skipped, no deeper than maxDepth.
func TestLoadConfigFile(t *testing.T) {
	env := home(t, map[string]string{
		xdgFile:                 "config-file = inc/a\nconfig-file = ?missing\nconfig-file = missing-too\nconfig-file = ~/abs\nfont-size = 11\n",
		appSupport:              "font-size = 12\nbackground = #000001\n",
		".config/ghostty/inc/a": "background = #000002\nconfig-file = b\n",
		".config/ghostty/inc/b": "foreground = #000003\nconfig-file = a\nconfig-file = ../config.ghostty\n", // cycles
		"abs":                   "font-size = 13\n",
	})
	c := Load(env).Light
	// a, after both files, over App Support's background; ~/abs after a;
	// config.ghostty again, as Ghostty reads it, its font-size last.
	if *c.Background != *col("#000002") || *c.Foreground != *col("#000003") || c.FontSize != 11 {
		t.Errorf("%+v", c)
	}

	// A chain: each file includes the next, setting its depth as font-size.
	files := map[string]string{xdgFile: "config-file = c1\n"}
	for i := 1; i <= 15; i++ {
		files[".config/ghostty/c"+strconv.Itoa(i)] = "font-size = " + strconv.Itoa(i) + "\nconfig-file = c" + strconv.Itoa(i+1) + "\n"
	}
	if c := Load(home(t, files)).Light; c.FontSize != maxDepth {
		t.Errorf("chain: font-size %v, want %v", c.FontSize, maxDepth)
	}

	// config-file = resets the files named so far.
	env = home(t, map[string]string{xdgFile: "config-file = x\nconfig-file =\n", ".config/ghostty/x": "font-size = 19\n"})
	if c := Load(env).Light; c.FontSize != 0 {
		t.Errorf("reset: font-size %v", c.FontSize)
	}
}

// A theme's keys apply first, the config's over them; it is the user's
// file, else Ghostty's, else an absolute path; light:…,dark:… gives each
// appearance its own; one not found is unrecognized.
func TestLoadTheme(t *testing.T) {
	theme := "background = #0000aa\nforeground = #0000bb\npalette = 1=#0000cc\nselection-background = #0000dd\nconfig-file = /nope\ntheme = Other\n"
	files := map[string]string{
		xdgFile:                     "palette = 1=#ff0000\ntheme = Bundled\nforeground = #00ff00\n",
		"res/themes/Bundled":        theme,
		"res/themes/Night":          "background = #000000\n",
		"res/themes/Day":            "background = #ffffff\n",
		".config/ghostty/themes/Me": "background = #123456\n",
	}
	c := Load(home(t, files)).Light
	if *c.Background != *col("#0000aa") || *c.Foreground != *col("#00ff00") || *c.Palette[1] != *col("#ff0000") ||
		*c.SelectionBackground != *col("#0000dd") || len(c.Unrecognized) > 0 {
		t.Errorf("bundled: %+v", c)
	}
	// The user's own, of the same name, before Ghostty's.
	files[".config/ghostty/themes/Bundled"] = "background = #111111\n"
	if c := Load(home(t, files)).Light; *c.Background != *col("#111111") || c.SelectionBackground != nil {
		t.Errorf("user's: %+v", c)
	}

	files[xdgFile] = "theme = light:Day, dark : \"Night\"\n"
	r := Load(home(t, files))
	if !r.Conditional || *r.Light.Background != *col("#ffffff") || *r.Dark.Background != *col("#000000") || r.For(true) != r.Dark {
		t.Errorf("light and dark: conditional %v, %+v, %+v", r.Conditional, r.Light, r.Dark)
	}

	env := home(t, files)
	write(t, filepath.Join(env.Home, "x", "mine.theme"), "background = #222222\n")
	write(t, filepath.Join(env.Home, xdgFile), "theme = "+filepath.Join(env.Home, "x", "mine.theme")+"\n")
	if c := Load(env).Light; *c.Background != *col("#222222") {
		t.Errorf("absolute: %+v", c)
	}

	for _, v := range []string{"Missing", "sub/Me", "light:Day", "light:Day,dark:Night,dusk:X"} {
		files[xdgFile] = "theme = " + v + "\n"
		r := Load(home(t, files))
		if r.Light.Background != nil || !reflect.DeepEqual(r.Light.Unrecognized, []string{"theme"}) {
			t.Errorf("theme = %s: %+v", v, r.Light)
		}
	}
	// theme = resets it.
	files[xdgFile] = "theme = Me\ntheme =\n"
	if c := Load(home(t, files)).Light; c.Applied() || len(c.Unrecognized) > 0 {
		t.Errorf("reset: %+v", c)
	}
}

// A file over maxFileSize is skipped, as one that isn't a file.
func TestLoadLarge(t *testing.T) {
	env := home(t, map[string]string{
		xdgLegacy: "font-size = 12\n",
		xdgFile:   "font-size = 14\n" + strings.Repeat("# padding\n", maxFileSize/10+1),
	})
	if c := Load(env).Light; c.FontSize != 12 {
		t.Errorf("font-size %v, want the smaller file's 12", c.FontSize)
	}
	env = home(t, nil)
	if err := os.MkdirAll(filepath.Join(env.Home, xdgFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if c := Load(env).Light; c.Applied() {
		t.Errorf("a folder: %+v", c)
	}
}

// Changed tells a file read, included or the theme was edited, or one
// looked for is there now.
func TestChanged(t *testing.T) {
	env := home(t, map[string]string{xdgFile: "config-file = inc\ntheme = T\n", ".config/ghostty/inc": "font-size = 12\n", "res/themes/T": "background = #000000\n"})
	touch := func(name, data string) {
		path := filepath.Join(env.Home, name)
		write(t, path, data)
		// A later time, whatever the file system's resolution.
		later := time.Now().Add(time.Hour)
		if err := os.Chtimes(path, later, later); err != nil {
			t.Fatal(err)
		}
	}
	r := Load(env)
	if r.Changed() {
		t.Fatal("changed as read")
	}
	for _, name := range []string{".config/ghostty/inc", "res/themes/T", appSupport, ".config/ghostty/themes/T", xdgFile} {
		touch(name, "font-size = 16\n")
		if !r.Changed() {
			t.Errorf("%s: not changed", name)
		}
		r = Load(env)
		if r.Changed() {
			t.Errorf("%s: changed as read again", name)
		}
	}
}

// A file of the chain that isn't a regular file, a FIFO above all, is
// skipped without being opened: opening a FIFO waits for a writer.
func TestLoadNotRegular(t *testing.T) {
	env := home(t, map[string]string{
		appSupport:         "font-size = 12\nconfig-file = fifo\ntheme = Fifo\n",
		"res/themes/Other": "background = #000001\n",
	})
	fifo := func(name string) {
		path := filepath.Join(env.Home, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Mkfifo(path, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fifo(xdgFile)
	fifo("Library/Application Support/com.mitchellh.ghostty/fifo")
	fifo("res/themes/Fifo")
	done := make(chan *Result)
	go func() { done <- Load(env) }()
	select {
	case r := <-done:
		if c := r.Light; c.FontSize != 12 || c.Background != nil || !reflect.DeepEqual(c.Unrecognized, []string{"theme"}) {
			t.Errorf("%+v", c)
		}
		if r.Changed() {
			t.Error("changed as read")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Load blocked on a FIFO")
	}
}

// A palette's index is read as Ghostty's Zig parseInt of base 0 reads it:
// decimal unless 0x, 0o or 0b, so a leading 0 is no octal.
func TestPaletteIndex(t *testing.T) {
	for s, want := range map[string]int{
		"8": 8, "08": 8, "010": 10, "0x0f": 15, "0X0A": 10, "0o10": 8, "0b11": 3, "+3": 3, "-0": 0, "1_0": 10, "0x_f": -1,
		"0x": -1, "_1": -1, "1_": -1, "-1": -1, "256": -1, "0o9": -1, "": -1, "1.0": -1,
	} {
		n, ok := parseIndex(s)
		if got := int(n); !ok {
			got = -1
		} else if got != want {
			t.Errorf("%q: %d, want %d", s, got, want)
		}
		if ok != (want >= 0) {
			t.Errorf("%q: ok %v, want %d", s, ok, want)
		}
	}
	c := Load(home(t, map[string]string{xdgFile: "palette = 010=#0000aa\npalette = 08=#0000bb\n"})).Light
	if *c.Palette[10] != *col("#0000aa") || *c.Palette[8] != *col("#0000bb") || len(c.Unrecognized) > 0 {
		t.Errorf("010 and 08: %v, %v, %v", c.Palette[10], c.Palette[8], c.Unrecognized)
	}
}

// An empty config-file in an included file clears the list of files as
// Ghostty 1.3's does: Ghostty reads the list by index as it grows, so the
// files named after the clear take the places of those read, and only
// those past them are read; the files named before it, not yet read, are
// not.
func TestLoadConfigFileClearedInInclude(t *testing.T) {
	env := home(t, map[string]string{
		xdgFile:              "config-file = a\nconfig-file = b\n",
		".config/ghostty/a":  "font-size = 11\nconfig-file =\nconfig-file = c\n",
		".config/ghostty/b":  "background = #00000b\n",
		".config/ghostty/c":  "foreground = #00000c\n",
		".config/ghostty/d":  "cursor-color = #00000d\n",
		".config/ghostty/a2": "font-size = 12\nconfig-file =\nconfig-file = c\nconfig-file = d\n",
	})
	c := Load(env).Light
	if c.FontSize != 11 || c.Background != nil || c.Foreground != nil {
		t.Errorf("cleared, one named after: %+v", c)
	}
	// Two named after: the list is c, d, and the second place, d's, is
	// read next.
	write(t, filepath.Join(env.Home, xdgFile), "config-file = a2\nconfig-file = b\n")
	c = Load(env).Light
	if c.FontSize != 12 || c.Background != nil || c.Foreground != nil || *c.CursorColor != *col("#00000d") {
		t.Errorf("cleared, two named after: %+v", c)
	}
}

// One of the four files that a file includes is read again where it is
// included, as Ghostty reads it: its keys over those of the files after
// it; once only, though it include itself.
func TestLoadMainIncluded(t *testing.T) {
	c := Load(home(t, map[string]string{
		xdgFile:    "font-size = 11\nfont-family = A\nconfig-file = config.ghostty\n",
		appSupport: "font-size = 12\n",
	})).Light
	if c.FontSize != 11 || !reflect.DeepEqual(c.FontFamily, []string{"A", "A"}) {
		t.Errorf("itself: font-size %v, families %v", c.FontSize, c.FontFamily)
	}
	c = Load(home(t, map[string]string{
		xdgLegacy:  "font-size = 11\n",
		appSupport: "font-size = 12\nconfig-file = ~/.config/ghostty/config\n",
	})).Light
	if c.FontSize != 11 {
		t.Errorf("another: font-size %v, want 11, read again last", c.FontSize)
	}
}
