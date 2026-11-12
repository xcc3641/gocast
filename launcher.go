package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gocast/internal/apps"
	"gocast/internal/presets"
	"gocast/internal/search"
	"gocast/locales"
)

const (
	launcherWidth, launcherHeight = 680, 420
	iconSize                      = 28 // DIPs
)

var (
	searchIcon   = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/></svg>`))
	terminalIcon = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="3" width="20" height="18" rx="3"/><path d="m7 9 3 3-3 3"/><path d="M13 15h4"/></svg>`))
)

// entry is what a search result opens: an app, or a terminal preset.
type entry struct {
	app      *mygo.InstalledApp
	preset   int // index in gocast.presets, when app is nil
	subtitle string
}

// launcher is the search panel. Its state is read and changed on the main
// thread; the apps come from a goroutine through Window.Update.
type launcher struct {
	g     *gocast
	panel *mygo.Window

	apps    []mygo.InstalledApp
	alts    [][]string // the apps' other names (English, Chinese), by index
	items   []search.Item
	entries []entry // by item
	results []int   // items matching the query, best first
	query   string
	chosen  int
	list    ui.ListState
	input   ui.Handle
	icons   map[string]*ui.Bitmap // by app path; nil where the app has none
	loading bool
	failed  string // the app that last failed to open, said in the footer
}

func newLauncher(g *gocast) *launcher {
	l := &launcher{g: g, icons: map[string]*ui.Bitmap{}}
	l.panel = mygo.NewWindow(mygo.WindowOptions{
		Title:         appName,
		Panel:         true,
		Hidden:        true,
		Frameless:     true,
		Transparent:   true,
		Vibrancy:      mygo.VibrancyGlass,
		CornerRadius:  16,
		Width:         launcherWidth,
		Height:        launcherHeight,
		DisableResize: true,
		Content:       ui.View(l.view),
	})
	// Hide rather than close, on Escape, Cmd-W, and when the user goes
	// elsewhere: the panel shows again at once, as it was.
	g.hideOnClose(l.panel)
	l.panel.OnBlur(l.panel.Hide)
	l.rebuild()
	return l
}

func (l *launcher) view(c *ui.Context) {
	t := c.Theme()
	root := c.Root()
	if c.Vibrancy() {
		root.Background(ui.Transparent) // the material shows through
	} else {
		root.Background(t.Background)
	}
	// An Escape left to the panel closes it, which hides it: with a query,
	// Escape clears it first.
	if l.query != "" && c.Shortcut(0, ui.KeyEscape) {
		l.query = ""
		l.filter()
	}
	if c.Shortcut(0, ui.KeyDown) {
		l.choose(l.chosen + 1)
	}
	if c.Shortcut(0, ui.KeyUp) {
		l.choose(l.chosen - 1)
	}
	// Enter opens the chosen result. The search field has the keyboard,
	// and a single-line text field takes Enter as its submission, so the
	// window's Shortcut never sees it there: the field's Submitted is
	// what a real Enter (and an automation's) reports. The list reports
	// its own Enter, and the window's Shortcut is the Enter pressed with
	// the focus elsewhere. Each opens once.
	open := c.Shortcut(0, ui.KeyEnter)
	c.OnShortcut(ui.Cmd, ui.KeyQ, mygo.App.Quit)

	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Padding(16, 18).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, searchIcon).FontSize(20).TextColor(t.TextMuted)
			field := ui.TextInputBase(c, &l.query).Bind(&l.input).Grow(1).FontSize(22).
				Placeholder(locales.SearchPlaceholder()).Label(locales.SearchPlaceholder())
			if field.Changed() {
				l.filter()
			}
			if field.Submitted() {
				open = true
			}
		})
		ui.Divider(c)
		l.list.Selected = &l.chosen
		l.list.Key = func(i int) any { return l.items[l.results[i]].ID }
		l.list.Label = func(i int) string { return l.items[l.results[i]].Title }
		list := ui.List(c, &l.list, len(l.results), func(i int) { l.row(c, i) }).Grow(1).Padding(6).Children(func() {
			switch {
			case len(l.results) > 0:
			case l.loading:
				ui.Text(c, locales.SearchFinding()).TextColor(t.TextMuted).Padding(16)
			default:
				ui.Column(c).Padding(16).Gap(6).Children(func() {
					ui.Text(c, locales.SearchNoResults(strings.TrimSpace(l.query))).TextColor(t.Text)
					ui.Text(c, locales.SearchNoResultsHint()).FontSize(12).TextColor(t.TextMuted)
					if len(l.g.presets) == 0 {
						ui.Text(c, locales.SearchNoPresetsHint()).FontSize(12).TextColor(t.TextMuted)
					}
				})
			}
		})
		if list.Submitted() {
			open = true
		}
		ui.Divider(c)
		ui.Row(c).Padding(8, 16).Gap(16).Children(func() {
			if l.failed != "" {
				ui.Text(c, locales.SearchOpenFailed(l.failed)).FontSize(11).TextColor(t.Danger).Grow(1).SingleLine()
			} else {
				hint := keys(l.g.launcherKey) + " " + strings.TrimSuffix(locales.TraySearch(), "…")
				if l.g.terminalKey != "" {
					hint += "　" + keys(l.g.terminalKey) + " " + locales.TrayTerminal()
				}
				ui.Text(c, hint).FontSize(11).TextColor(t.TextMuted).Grow(1).SingleLine()
			}
			hint := locales.SearchHintClose()
			if l.query != "" {
				hint = locales.SearchHintClear()
			}
			ui.Text(c, hint).FontSize(11).TextColor(t.TextMuted)
		})
	})
	if open {
		l.open()
	}
}

// row shows a result with its icon, its name, and what or where it is.
func (l *launcher) row(c *ui.Context, i int) {
	it := l.items[l.results[i]]
	e := l.entries[l.results[i]]
	t := c.Theme()
	bg := ui.Transparent
	text, muted := t.Text, t.TextMuted
	if i == l.chosen {
		bg = t.Accent.Alpha(0.85)
		text, muted = t.AccentText, t.AccentText.Alpha(0.75)
	}
	ui.Row(c).Padding(6, 10).Gap(12).AlignItems(ui.Center).Radius(8).Background(bg).Children(func() {
		switch {
		case e.app == nil:
			ui.Box(c).Size(iconSize, iconSize).Center().Children(func() {
				ui.Icon(c, terminalIcon).FontSize(22).TextColor(text)
			})
		case l.icon(*e.app) != nil:
			ui.Image(c, l.icon(*e.app)).Size(iconSize, iconSize)
		default:
			ui.Avatar(c, it.Title, nil).Size(iconSize, iconSize) // its initial
		}
		ui.Column(c).Grow(1).Gap(1).Children(func() {
			ui.Text(c, it.Title).TextColor(text).SingleLine()
			ui.Text(c, e.subtitle).FontSize(11).TextColor(muted).SingleLine()
		})
		tag := locales.SearchTagApp()
		if e.app == nil {
			tag = locales.SearchTagPreset()
		}
		ui.Text(c, tag).FontSize(11).TextColor(muted)
	})
}

// rebuild makes the searchable items: the terminal presets, then the
// apps, and filters them again.
func (l *launcher) rebuild() {
	l.items, l.entries = l.items[:0], l.entries[:0]
	for i, p := range l.g.presets {
		id := "preset:" + p.Name
		it := search.NewItem(id, presetTitle(p), p.Command, "Terminal", "终端")
		it.Boost = l.g.usage[id]
		l.items = append(l.items, it)
		l.entries = append(l.entries, entry{preset: i, subtitle: presetSubtitle(p)})
	}
	for i := range l.apps {
		app := &l.apps[i]
		id := "app:" + app.ID
		it := search.NewItem(id, app.Name, l.alts[i]...)
		it.Boost = l.g.usage[id]
		sub := where(*app)
		// The app's name in the other language, as 微信 shows WeChat.
		for _, n := range l.alts[i] {
			if n != app.Name && !strings.EqualFold(n, app.Name) && isOtherScript(n, app.Name) {
				sub = n + " · " + sub
				break
			}
		}
		l.items = append(l.items, it)
		l.entries = append(l.entries, entry{app: app, subtitle: sub})
	}
	l.filter()
}

// isOtherScript reports whether a and b differ in having non-ASCII
// letters, as 微信 and WeChat do, rather than being spellings of one name.
func isOtherScript(a, b string) bool {
	ascii := func(s string) bool { return utf8.RuneCountInString(s) == len(s) }
	return ascii(a) != ascii(b)
}

func (l *launcher) filter() {
	l.results = search.Filter(l.items, l.query)
	l.choose(0)
}

// choose chooses result i, within the results. The list follows the item
// chosen as the results change (by its Key): Choose tells it this is a
// new choice, so that it does not drop it (-1) when the item it had is
// filtered out.
func (l *launcher) choose(i int) {
	l.chosen = max(0, min(i, len(l.results)-1))
	if len(l.results) > 0 {
		l.list.Selected = &l.chosen
		l.list.Choose(l.chosen)
		l.list.ScrollIntoView(l.chosen)
	}
}

// open opens the chosen result and hides the panel first, which gives the
// keyboard back to the app the user was in, before the launched app (or
// the terminal) takes it.
func (l *launcher) open() {
	if l.chosen < 0 && len(l.results) > 0 {
		l.chosen = 0 // the list dropped its choice: open the first
	}
	if l.chosen < 0 || l.chosen >= len(l.results) {
		return
	}
	it := l.items[l.results[l.chosen]]
	e := l.entries[l.results[l.chosen]]
	l.panel.Hide()
	l.failed = ""
	l.g.used(it.ID)
	if e.app == nil {
		l.g.term.openPreset(l.g.presets[e.preset].Name)
		return
	}
	if err := mygo.Shell.LaunchApp(*e.app); err != nil {
		log.Printf("launching %s: %v", e.app.Name, err)
		l.show()
		l.failed = e.app.Name
	}
}

// load lists the installed apps and their other names on a goroutine, as
// at each show, where apps may have come or gone since.
func (l *launcher) load() {
	l.loading = true
	go func() {
		found, err := mygo.Shell.InstalledApps()
		if err != nil {
			log.Print(err)
		}
		type named struct {
			app  mygo.InstalledApp
			alts []string
			key  string
		}
		list := make([]named, len(found))
		locale := mygo.App.Locale()
		for i, app := range found {
			info := apps.Names(app.Path)
			alts := append([]string{app.Name}, info.Names...)
			// The name in the language the app shows, which the system's
			// name may not be (see apps.Info.Localized).
			if n := info.Localized(locale); n != "" {
				app.Name = n
			}
			list[i] = named{app, alts, search.SortKey(app.Name)}
		}
		// By name, Chinese names among the others by their pinyin.
		slices.SortStableFunc(list, func(a, b named) int { return strings.Compare(a.key, b.key) })
		found = found[:0]
		alts := make([][]string, len(list))
		for i, n := range list {
			found = append(found, n.app)
			alts[i] = n.alts
		}
		l.panel.Update(func() {
			l.apps, l.alts, l.loading = found, alts, false
			l.rebuild()
		})
	}()
}

// show shows the panel on the display with the pointer, a third of the way
// down, with an empty query.
func (l *launcher) show() {
	if l.g.loadPresets() {
		l.rebuild()
	}
	d := mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint())
	w, h := l.panel.Size()
	l.panel.SetPosition(d.WorkArea.X+(d.WorkArea.Width-w)/2, d.WorkArea.Y+(d.WorkArea.Height-h)/3)
	l.query = ""
	l.failed = ""
	l.filter()
	l.load()
	l.input.Focus()
	l.panel.Show()
	l.panel.Invalidate() // draw the state changed outside a frame
}

func (l *launcher) toggle() {
	if l.panel.IsVisible() {
		l.panel.Hide()
	} else {
		l.show()
	}
}

// icon returns an app's icon, drawn for the sharpest display the first time
// it shows (a few milliseconds), and kept.
func (l *launcher) icon(app mygo.InstalledApp) *ui.Bitmap {
	if b, ok := l.icons[app.Path]; ok {
		return b
	}
	scale := 1.0
	for _, d := range mygo.Screen.Displays() {
		scale = max(scale, d.ScaleFactor)
	}
	var bitmap *ui.Bitmap
	if data, err := mygo.Shell.FileIcon(app.Path, int(iconSize*scale)); err == nil {
		bitmap, _ = ui.DecodeBitmap(data)
	}
	l.icons[app.Path] = bitmap
	return bitmap
}

// where says where an app is, shortly: its folder.
func where(app mygo.InstalledApp) string {
	dir := filepath.Dir(app.Path)
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(dir, home) {
		return "~" + dir[len(home):]
	}
	for _, system := range []string{"/System/Applications", "/System/Library/CoreServices/Applications", "/System/Library/CoreServices"} {
		if rest, ok := strings.CutPrefix(dir, system); ok {
			return "System" + rest
		}
	}
	return strings.TrimPrefix(dir, "/")
}

// keys spells a shortcut as macOS does: ⌥⇧Space.
func keys(shortcut string) string {
	if runtime.GOOS != "darwin" {
		return shortcut
	}
	return strings.NewReplacer("CmdOrCtrl+", "⌘", "Cmd+", "⌘", "Ctrl+", "⌃", "Alt+", "⌥", "Shift+", "⇧").Replace(shortcut)
}

// presetTitle is a preset's name, or "Terminal" for one without.
func presetTitle(p presets.Preset) string {
	if strings.TrimSpace(p.Name) == "" {
		return locales.TermTabDefault()
	}
	return p.Name
}

// presetSubtitle says where and what a preset runs: "~/Work/x · pi".
func presetSubtitle(p presets.Preset) string {
	if strings.TrimSpace(p.Command) == "" {
		return locales.PresetSubtitleShell(p.ShortDir())
	}
	return locales.PresetSubtitle(p.ShortDir(), p.Command)
}
