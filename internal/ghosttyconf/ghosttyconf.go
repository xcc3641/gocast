// Package ghosttyconf reads what the user's Ghostty configuration says the
// terminal looks like: its font, colors, cursor, padding and opacity
// (design.md 3.3b, the floating terminal's Ghostty look). It reads the
// files Ghostty 1.3 reads on macOS, in its order, with the files they
// include and the theme they name, and keeps their appearance keys alone;
// it never fails: a file it can't read is skipped, as are lines it can't
// parse and keys it doesn't know. It uses the standard library only.
//
// What Ghostty does, and so this package (src/config/Config.zig,
// file_load.zig, theme.zig, cli/args.zig of Ghostty 1.3.1):
//
//   - The files: $XDG_CONFIG_HOME/ghostty/config, then config.ghostty
//     beside it ($XDG_CONFIG_HOME is ~/.config when unset), then
//     ~/Library/Application Support/com.mitchellh.ghostty/config, then
//     config.ghostty there; later ones override earlier ones.
//   - Lines: key = value, whitespace and a CR trimmed around both; a line
//     starting with # is a comment (there are no comments after a value);
//     a value in double quotes loses them; key = with nothing resets the
//     key to its default; a key without = is a flag (true). A UTF-8 byte
//     order mark at the start of a file is skipped.
//   - config-file = path adds a file, relative to the directory of the
//     file that names it (or ~/…); ? before it makes a missing file fine.
//     The files added are read after all four, in the order added, those
//     they add after them; each once. Gocast reads no deeper than
//     maxDepth, and no file larger than maxFileSize.
//   - theme = name, or light:name,dark:name, picked by the system's
//     appearance: an absolute path, or a file of that name in
//     $XDG_CONFIG_HOME/ghostty/themes, else in Ghostty's resources'
//     themes. Its keys apply first, and the config's after, over them.
package ghosttyconf

import (
	"bytes"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// maxFileSize is the largest file read: a larger one is skipped.
const maxFileSize = 1 << 20

// maxDepth is how deep files are included (config-file): the four files
// Ghostty reads are 0, a file they include 1…
const maxDepth = 10

// Color is a color the config sets, opaque.
type Color struct{ R, G, B uint8 }

// Padding is a window-padding-x or -y: left and right, or top and bottom,
// in points.
type Padding struct{ Start, End int }

// Cursor styles, as cursor-style names them; block_hollow is CursorBlock.
const (
	CursorBlock     = "block"
	CursorBar       = "bar"
	CursorUnderline = "underline"
)

// Config is what the configuration sets of the terminal's appearance,
// unset (nil, zero, empty) where it sets nothing or reset it.
type Config struct {
	// FontFamily is the font-family list, the first first.
	FontFamily []string
	// FontSize is the font-size in points, 0 when unset.
	FontSize float64
	// The colors: the background and the text; the 16 colors of the
	// palette (palette = N=color for N up to 15; Ghostty's further ones
	// are not kept); the cursor and the text under it; the selection's
	// background and text.
	Background, Foreground                   *Color
	Palette                                  [16]*Color
	CursorColor, CursorText                  *Color
	SelectionBackground, SelectionForeground *Color
	// CursorStyle is one of CursorBlock, CursorBar and CursorUnderline,
	// "" when unset; CursorBlink is cursor-style-blink.
	CursorStyle string
	CursorBlink *bool
	// PaddingX and PaddingY are window-padding-x and -y.
	PaddingX, PaddingY *Padding
	// BackgroundOpacity is background-opacity, from 0 to 1.
	BackgroundOpacity *float64
	// Unrecognized are the appearance keys whose value could not be used,
	// sorted: a color of a form not read (X11 names, cell-foreground), a
	// number that isn't one, a theme not found… Their earlier value, or
	// their default, stays.
	Unrecognized []string
}

// Applied reports whether the configuration set any of the terminal's
// appearance.
func (c *Config) Applied() bool {
	if len(c.FontFamily) > 0 || c.FontSize > 0 || c.Background != nil || c.Foreground != nil ||
		c.CursorColor != nil || c.CursorText != nil || c.SelectionBackground != nil || c.SelectionForeground != nil ||
		c.CursorStyle != "" || c.CursorBlink != nil || c.PaddingX != nil || c.PaddingY != nil ||
		c.BackgroundOpacity != nil {
		return true
	}
	return slices.ContainsFunc(c.Palette[:], func(p *Color) bool { return p != nil })
}

// Env is where the files are.
type Env struct {
	// Home is the user's home folder; XDGConfigHome is $XDG_CONFIG_HOME,
	// Home/.config when "".
	Home, XDGConfigHome string
	// Resources are the folders Ghostty may have its resources in
	// (Ghostty.app/Contents/Resources/ghostty), whose themes folder has
	// the themes it ships: the first that has the theme gives it.
	Resources []string
}

func (e Env) xdg() string {
	if e.XDGConfigHome != "" {
		return e.XDGConfigHome
	}
	return filepath.Join(e.Home, ".config")
}

// Files are the files Ghostty reads, in its order.
func (e Env) Files() []string {
	support := filepath.Join(e.Home, "Library", "Application Support", "com.mitchellh.ghostty")
	return []string{
		filepath.Join(e.xdg(), "ghostty", "config"),
		filepath.Join(e.xdg(), "ghostty", "config.ghostty"),
		filepath.Join(support, "config"),
		filepath.Join(support, "config.ghostty"),
	}
}

// Result is the configuration as read: its appearance in light and dark
// mode, the same unless its theme has one for each.
type Result struct {
	Light, Dark *Config
	// Conditional tells that the theme is light:…,dark:…, of two themes.
	Conditional bool
	// stamps are every file looked at, there or not, as it was.
	stamps []stamp
}

// For is the appearance in dark mode, or in light mode.
func (r *Result) For(dark bool) *Config {
	if dark {
		return r.Dark
	}
	return r.Light
}

// stamp is a file looked at: whether it was there, its size and time.
type stamp struct {
	path  string
	there bool
	size  int64
	mod   time.Time
}

func stampOf(path string) stamp {
	st := stamp{path: path}
	if fi, err := os.Stat(path); err == nil {
		st.there, st.size, st.mod = true, fi.Size(), fi.ModTime()
	}
	return st
}

// Changed reports whether a file the result was read from, the files it
// included or its theme, changed since, or one it looked for and didn't
// find is there now: Load again then.
func (r *Result) Changed() bool {
	for _, st := range r.stamps {
		if stampOf(st.path) != st {
			return true
		}
	}
	return false
}

// line is a line of a file: its key, and its value, with has false for a
// line with no =; dir is the folder of the file.
type line struct {
	key, value string
	has        bool
	dir        string
}

// loader reads the files.
type loader struct {
	env    Env
	stamps []stamp
	seen   map[string]bool
}

// read returns the lines of the file at path, and false where it isn't a
// file or can't be read whole, or is larger than maxFileSize.
func (l *loader) read(path string) ([]line, bool) {
	st := stampOf(path)
	l.stamps = append(l.stamps, st)
	if !st.there {
		return nil, false
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	if fi, err := f.Stat(); err != nil || !fi.Mode().IsRegular() || fi.Size() > maxFileSize {
		return nil, false
	}
	data, err := io.ReadAll(io.LimitReader(f, maxFileSize+1))
	if err != nil || len(data) > maxFileSize {
		return nil, false
	}
	return parse(data, filepath.Dir(path)), true
}

// parse returns the lines of a file's data, its folder dir, as Ghostty's
// LineIterator takes them: comments and blank lines skipped.
func parse(data []byte, dir string) []line {
	data = bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF"))
	var lines []line
	for raw := range bytes.SplitSeq(data, []byte("\n")) {
		s := strings.Trim(string(raw), " \t\r")
		if s == "" || s[0] == '#' {
			continue
		}
		k, v, has := strings.Cut(s, "=")
		k = strings.Trim(k, " \t")
		if has {
			v = strings.Trim(v, " \t")
			if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
				v = v[1 : len(v)-1]
			}
		}
		lines = append(lines, line{key: k, value: v, has: has, dir: dir})
	}
	return lines
}

// include is a file config-file names: its path, absolute, and how deep.
type include struct {
	path  string
	depth int
}

// Load reads the configuration env has. It never fails: what it can't read
// it skips.
func Load(env Env) *Result {
	l := &loader{env: env, seen: map[string]bool{}}
	var lines []line
	var includes []include
	// adds takes the config-file lines of a file depth deep: each adds a
	// file, an empty one clears the list (Ghostty's RepeatablePath).
	adds := func(ls []line, depth int) {
		for _, ln := range ls {
			if ln.key != "config-file" || !ln.has {
				continue
			}
			v := ln.value
			if v == "" {
				includes = includes[:0]
				continue
			}
			v = strings.TrimPrefix(v, "?") // a missing file is skipped either way
			if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
				v = v[1 : len(v)-1]
			}
			if v == "" {
				continue
			}
			includes = append(includes, include{l.abs(v, ln.dir), depth + 1})
		}
	}
	for _, path := range env.Files() {
		ls, _ := l.read(path)
		lines = append(lines, ls...)
		adds(ls, 0)
	}
	for i := 0; i < len(includes); i++ {
		in := includes[i]
		if in.depth > maxDepth {
			continue
		}
		key := in.path
		if real, err := filepath.EvalSymlinks(in.path); err == nil {
			key = real
		}
		if l.seen[key] {
			continue // a cycle, or a file named twice: read once
		}
		l.seen[key] = true
		ls, _ := l.read(in.path)
		lines = append(lines, ls...)
		adds(ls, in.depth)
	}

	// The theme: the last theme line's, a reset or one that doesn't parse
	// leaving the one before (Ghostty's ?Theme).
	var theme *themeName
	themeBad := false
	for _, ln := range lines {
		if ln.key != "theme" {
			continue
		}
		if ln.has && ln.value == "" {
			theme, themeBad = nil, false
			continue
		}
		if t, ok := parseTheme(ln.value, ln.has); ok {
			theme, themeBad = &t, false
		} else {
			themeBad = true
		}
	}
	r := &Result{}
	resolve := func(name string) *Config {
		c := &Config{}
		bad := map[string]bool{"theme": themeBad}
		if theme != nil {
			if tl, ok := l.theme(name); ok {
				for _, ln := range tl {
					c.apply(ln, bad, true)
				}
			} else {
				bad["theme"] = true
			}
		}
		for _, ln := range lines {
			c.apply(ln, bad, false)
		}
		for k, b := range bad {
			if b {
				c.Unrecognized = append(c.Unrecognized, k)
			}
		}
		slices.Sort(c.Unrecognized)
		return c
	}
	switch {
	case theme == nil:
		r.Light = resolve("")
		r.Dark = r.Light
	case theme.light == theme.dark:
		r.Light = resolve(theme.light)
		r.Dark = r.Light
	default:
		r.Conditional = true
		r.Light, r.Dark = resolve(theme.light), resolve(theme.dark)
	}
	r.stamps = l.stamps
	return r
}

// abs is path as config-file names it in a file in dir, absolute: under
// the home folder for ~/…, else under dir.
func (l *loader) abs(path, dir string) string {
	switch {
	case filepath.IsAbs(path):
		return filepath.Clean(path)
	case strings.HasPrefix(path, "~/"):
		return filepath.Join(l.env.Home, path[2:])
	}
	return filepath.Join(dir, path)
}

// themeName is the theme's file in light and dark mode.
type themeName struct{ light, dark string }

// parseTheme parses theme's value v: a name, or light:a,dark:b (both, in
// any order, a value in quotes possibly).
func parseTheme(v string, has bool) (themeName, bool) {
	if !has || v == "" {
		return themeName{}, false
	}
	if !strings.ContainsAny(v, ",=:") {
		n := strings.Trim(v, " \t")
		return themeName{n, n}, n != ""
	}
	var t themeName
	var light, dark bool
	for _, part := range splitComma(v) {
		k, val, ok := strings.Cut(part, ":")
		if !ok {
			return themeName{}, false
		}
		val = strings.Trim(val, " \t")
		if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
			uq, err := strconv.Unquote(val)
			if err != nil {
				return themeName{}, false
			}
			val = uq
		}
		switch strings.Trim(k, " \t") {
		case "light":
			t.light, light = val, true
		case "dark":
			t.dark, dark = val, true
		default:
			return themeName{}, false
		}
	}
	return t, light && dark && t.light != "" && t.dark != ""
}

// splitComma splits v at its commas outside double quotes.
func splitComma(v string) []string {
	var parts []string
	quoted, start := false, 0
	for i := 0; i < len(v); i++ {
		switch v[i] {
		case '\\':
			if quoted {
				i++
			}
		case '"':
			quoted = !quoted
		case ',':
			if !quoted {
				parts = append(parts, v[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, v[start:])
}

// theme returns the lines of the theme name names, as Ghostty finds it:
// an absolute path, else a file of the user's themes folder, else of the
// themes Ghostty ships; false when there is none, or it can't be read.
func (l *loader) theme(name string) ([]line, bool) {
	var paths []string
	switch {
	case filepath.IsAbs(name):
		paths = []string{name}
	case strings.ContainsAny(name, `/\`) || name == "." || name == "..":
		return nil, false
	default:
		paths = append(paths, filepath.Join(l.env.xdg(), "ghostty", "themes", name))
		for _, r := range l.env.Resources {
			paths = append(paths, filepath.Join(r, "themes", name))
		}
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			l.stamps = append(l.stamps, stampOf(p))
			continue
		}
		// The first there is the theme, even where it can't be read.
		return l.read(p)
	}
	return nil, false
}

// apply applies the line ln to c, if it sets the appearance, marking in
// bad the keys whose value can't be used, and unmarking those set or
// reset: those of a theme's file when theme, which never include files or
// name a theme.
func (c *Config) apply(ln line, bad map[string]bool, theme bool) {
	k, v := ln.key, ln.value
	reset := ln.has && v == ""
	ok := true
	switch k {
	case "font-family":
		switch {
		case !ln.has:
			ok = false
		case reset:
			c.FontFamily = nil
		default:
			c.FontFamily = append(c.FontFamily, v)
		}
	case "font-size":
		switch {
		case reset:
			c.FontSize = 0
		default:
			f, err := strconv.ParseFloat(v, 32)
			if ok = err == nil && f > 0 && !math.IsInf(f, 0) && ln.has; ok {
				c.FontSize = f
			}
		}
	case "background":
		ok = setColor(&c.Background, v, ln.has)
	case "foreground":
		ok = setColor(&c.Foreground, v, ln.has)
	case "cursor-color":
		ok = setColor(&c.CursorColor, v, ln.has)
	case "cursor-text":
		ok = setColor(&c.CursorText, v, ln.has)
	case "selection-background":
		ok = setColor(&c.SelectionBackground, v, ln.has)
	case "selection-foreground":
		ok = setColor(&c.SelectionForeground, v, ln.has)
	case "palette":
		if reset {
			c.Palette = [16]*Color{}
			for key := range bad {
				if strings.HasPrefix(key, "palette") {
					delete(bad, key)
				}
			}
			return
		}
		n, col, has := strings.Cut(v, "=")
		i, err := strconv.ParseUint(strings.Trim(n, " \t"), 0, 8)
		if !ln.has || !has || err != nil {
			bad["palette"] = true
			return
		}
		key := "palette=" + strconv.FormatUint(i, 10)
		cl, good := ParseColor(col)
		if !good {
			if i < 16 {
				bad[key] = true
			}
			return
		}
		if i < 16 {
			c.Palette[i] = &cl
			delete(bad, key)
		}
		return
	case "cursor-style":
		switch {
		case reset:
			c.CursorStyle = ""
		case v == "block" || v == "block_hollow":
			c.CursorStyle = CursorBlock
		case v == "bar":
			c.CursorStyle = CursorBar
		case v == "underline":
			c.CursorStyle = CursorUnderline
		default:
			ok = false
		}
	case "cursor-style-blink":
		switch {
		case reset:
			c.CursorBlink = nil
		case !ln.has:
			t := true
			c.CursorBlink = &t
		default:
			b, good := parseBool(v)
			if ok = good; ok {
				c.CursorBlink = &b
			}
		}
	case "window-padding-x":
		ok = setPadding(&c.PaddingX, v, ln.has)
	case "window-padding-y":
		ok = setPadding(&c.PaddingY, v, ln.has)
	case "background-opacity":
		switch {
		case reset:
			c.BackgroundOpacity = nil
		default:
			f, err := strconv.ParseFloat(v, 64)
			if ok = err == nil && !math.IsNaN(f) && ln.has; ok {
				f = min(max(f, 0), 1)
				c.BackgroundOpacity = &f
			}
		}
	default:
		return // not the appearance's, or not a key: skipped
	}
	if ok {
		delete(bad, k)
	} else {
		bad[k] = true
	}
}

// setColor sets *dst to the color v, or to nil for "": false when v is no
// color read.
func setColor(dst **Color, v string, has bool) bool {
	if !has {
		return false
	}
	if v == "" {
		*dst = nil
		return true
	}
	c, ok := ParseColor(v)
	if ok {
		*dst = &c
	}
	return ok
}

// setPadding sets *dst to the padding v, "a" or "a,b", or to nil for "".
func setPadding(dst **Padding, v string, has bool) bool {
	if !has {
		return false
	}
	if v == "" {
		*dst = nil
		return true
	}
	a, b, two := strings.Cut(v, ",")
	start, err := strconv.ParseUint(strings.Trim(a, " \t"), 10, 31)
	if err != nil {
		return false
	}
	end := start
	if two {
		end, err = strconv.ParseUint(strings.Trim(b, " \t"), 10, 31)
		if err != nil {
			return false
		}
	}
	*dst = &Padding{int(start), int(end)}
	return true
}

// ParseColor parses a color as Ghostty writes one in hex: #rrggbb, rrggbb,
// #rgb or rgb. X11 color names aren't read.
func ParseColor(v string) (Color, bool) {
	v = strings.TrimPrefix(strings.Trim(v, " \t"), "#")
	if len(v) == 3 {
		v = string([]byte{v[0], v[0], v[1], v[1], v[2], v[2]})
	}
	if len(v) != 6 {
		return Color{}, false
	}
	n, err := strconv.ParseUint(v, 16, 32)
	if err != nil {
		return Color{}, false
	}
	return Color{uint8(n >> 16), uint8(n >> 8), uint8(n)}, true
}

// parseBool parses a boolean as Ghostty does.
func parseBool(v string) (bool, bool) {
	switch v {
	case "1", "t", "T", "true":
		return true, true
	case "0", "f", "F", "false":
		return false, true
	}
	return false, false
}
