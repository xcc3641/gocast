package app

// Keep Awake (design.md 2.10, copy.md 18): its rows in the search panel,
// the tray's icon and lines, and the switch in Settings › General, over
// package keepawake's Keeper, which holds the power assertion.

import (
	"log"
	"slices"
	"strconv"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"

	"gocast/icon"
	"gocast/internal/keepawake"
	"gocast/internal/search"
	"gocast/internal/sysfmt"
	"gocast/locales"
)

var (
	// sysAwakeBackend holds Keep Awake's power assertions: nil for the
	// system's; the tests hold none.
	sysAwakeBackend keepawake.Backend
	// sysClock says a time of day on the system's 12- or 24-hour clock
	// (copy.md 18.0); the tests on a fixed one.
	sysClock = sysfmt.Clock
	// sysAwakePost runs fn on the main thread, after a change the
	// Keeper tells of from its own goroutine (Ended, Ticked) or from the
	// caller's; the panel draws again after. The tests run it at once.
	sysAwakePost = func(g *gocast, fn func()) { g.launcher.win.Update(fn) }
)

// The item IDs of Keep Awake's rows: Keep Awake (Stop Keeping Awake while
// on), its presets, and the row of what is typed ("keep awake 1h30m",
// "until 5pm"), which the search finds by no name.
const (
	awakePrefix   = "awake:"
	awakeToggleID = awakePrefix + "toggle"
	awakeTypedID  = awakePrefix + "typed"
)

// awakeRow is what a Keep Awake row is.
type awakeRow int

const (
	awakeNone  awakeRow = iota
	awakeStart          // Keep Awake, a preset, or a length or time typed: starts entry.request
	awakeStop           // Stop Keeping Awake, in Keep Awake's place while on
	awakeHint           // "keep awake until" with no time yet: does nothing
)

// awakeTrayPNG is the tray icon while keeping awake: the mark filled, its
// bar cut out (design.md 2.10).
var awakeTrayPNG = icon.TrayAwake

// awakeWords say Keep Awake's state in Gocast's language.
func awakeWords() keepawake.Words {
	loc := i18n.Locale()
	return keepawake.Words{Msgs: locales.In(loc), Clock: func(t time.Time) string { return sysClock(t, loc) }}
}

// newAwake makes g's Keeper, the setting as it is, events brought to the
// main thread.
func (g *gocast) newAwake(o keepawake.Options) {
	if g.awakeNow == nil {
		g.awakeNow = time.Now
	}
	o.Now = g.awakeNow
	o.AllowDisplaySleep = g.settings.AwakeScreenOff
	g.awake = keepawake.New(o)
	g.awake.OnChange(func(keepawake.Event) { sysAwakePost(g, g.awakeChanged) })
}

// setUpAwake makes the Keeper: it ends as Gocast quits (copy.md 18.6),
// and checks the clock as the Mac wakes, a session past its end ending at
// once.
func (g *gocast) setUpAwake() {
	g.newAwake(keepawake.Options{Backend: sysAwakeBackend})
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) { g.closeAwake() })
	mygo.Power.OnResume(g.awake.Check)
}

// closeAwake ends Keep Awake as Gocast quits, the tray icon back to
// normal at once, as Close tells of no change.
func (g *gocast) closeAwake() {
	g.awake.Close()
	g.awakeChanged()
}

// awakeActive reports whether Gocast keeps the Mac awake.
func (g *gocast) awakeActive() bool { return g.awake != nil && g.awake.State().Active() }

// keepAwake starts r (Forever with no end) in place of the session
// running; r invalid, or the assertion not made, nothing changes. It
// reports whether it started.
func (g *gocast) keepAwake(r keepawake.Request) bool {
	if err := g.awake.Start(r); err != nil {
		log.Printf("keep awake: %v", err)
		return false
	}
	g.awakeChanged()
	return true
}

// stopAwake stops keeping awake.
func (g *gocast) stopAwake() {
	g.awake.Stop()
	g.awakeChanged()
}

// setAwakeScreenOff applies the switch "Let the screen turn off while
// keeping awake" at once, to a session running too, saying nothing
// (design.md 2.10); it reports whether it could.
func (g *gocast) setAwakeScreenOff(on bool) bool {
	if g.awake != nil {
		if err := g.awake.SetAllowDisplaySleep(on); err != nil {
			log.Printf("keep awake: %v", err)
			return false
		}
	}
	g.settings.AwakeScreenOff = on
	g.saveSettings()
	return true
}

// awakeChanged shows Keep Awake as it is now: the tray's icon and menu,
// and the panel's rows, whose names change as it starts or ends. Run
// again for the same change, it changes nothing.
func (g *gocast) awakeChanged() {
	on := g.awakeActive()
	changed := on != g.awakeOn
	g.awakeOn = on
	if changed {
		g.showTrayAwake()
	}
	g.updateTray() // what is left, by the minute
	if l := g.launcher; l != nil {
		if changed {
			l.rebuild()
		}
		l.redraw()
	}
}

// --- The tray ---

// showTrayAwake gives the tray icon its look as Keep Awake is (awakeOn),
// at once, with no transition: filled while on, which assistive
// technology reads as "Gocast, keeping awake" (design.md 2.10, copy.md
// 18.5).
func (g *gocast) showTrayAwake() {
	png, tip := trayPNG, appName
	if g.awakeOn {
		png, tip = awakeTrayPNG, locales.AwakeTrayLabel()
	}
	sysTrayLook(g, png, tip)
}

// sysTrayLook gives the tray icon png and the tooltip tip; the tests
// note the tooltip.
var sysTrayLook = func(g *gocast, png []byte, tip string) {
	if g.tray == nil {
		return
	}
	if err := g.tray.SetIcon(png, true); err != nil {
		log.Printf("tray icon: %v", err)
	}
	g.tray.SetToolTip(tip)
}

// awakeTrayItems are Keep Awake's items over the tray menu's first
// group, while on (design.md 2.10, copy.md 18.3): what is left, greyed,
// Stop Keeping Awake, and Change To ›.
func (g *gocast) awakeTrayItems() []*mygo.MenuItem {
	if !g.awakeActive() {
		return nil
	}
	w := awakeWords()
	return []*mygo.MenuItem{
		{Label: w.Status(g.awake.Left()), Disabled: true},
		{Label: locales.AwakeStop(), Click: func(*mygo.MenuItem, *mygo.Window) { g.stopAwake() }},
		{Label: locales.AwakeChangeTo(), Submenu: g.awakeChoices(w)},
		mygo.Separator(),
	}
}

// awakeTrayMenu is Keep Awake ›, under the clipboard history, while off.
func (g *gocast) awakeTrayMenu() []*mygo.MenuItem {
	if g.awake == nil || g.awakeActive() {
		return nil
	}
	return []*mygo.MenuItem{{Label: locales.AwakeTitle(), Submenu: g.awakeChoices(awakeWords())}}
}

// awakeChoices are the submenu's items: Until I Stop, then the presets,
// no checks (copy.md 18.3).
func (g *gocast) awakeChoices(w keepawake.Words) []*mygo.MenuItem {
	start := func(r keepawake.Request) func(*mygo.MenuItem, *mygo.Window) {
		return func(*mygo.MenuItem, *mygo.Window) { g.keepAwake(r) }
	}
	items := []*mygo.MenuItem{{Label: locales.AwakeUntilIStop(), Click: start(keepawake.Request{Kind: keepawake.Forever})}}
	for _, d := range keepawake.Presets {
		items = append(items, &mygo.MenuItem{Label: w.Length(d), Click: start(keepawake.Request{Kind: keepawake.For, Duration: d})})
	}
	return items
}

// --- The search panel ---

// awakeItems are Keep Awake's search items (copy.md 18.1), with the
// system's actions: Keep Awake or Stop Keeping Awake, the presets, and
// the slot of the row of what is typed, which find fills.
func (l *launcher) awakeItems(now time.Time) {
	g := l.g
	if g.awake == nil {
		return
	}
	en, zh := locales.In("en"), locales.In("zh-Hans")
	names := []string{en.AwakeTitle(), zh.AwakeTitle(), en.AwakeStop(), zh.AwakeStop()}
	names = append(names, keepawake.Aliases...)
	add := func(id, title string, e entry, names []string) {
		it := search.NewItem(id, title, names...)
		// No usage: the rows keep copy.md 18.1's order (awakeRows).
		it.Tier = 1 // the system's (design.md 2.9)
		l.items = append(l.items, it)
		l.entries = append(l.entries, e)
	}
	// Keep Awake, until stopped; Stop Keeping Awake while on, as the
	// panel rebuilds when it starts or ends (awakeChanged).
	if g.awakeActive() {
		add(awakeToggleID, locales.AwakeStop(), entry{awake: awakeStop}, names)
	} else {
		add(awakeToggleID, locales.AwakeTitle(), entry{awake: awakeStart, request: keepawake.Request{Kind: keepawake.Forever}}, names)
	}
	w := awakeWords()
	wen, wzh := keepawake.Words{Msgs: en, Clock: w.Clock}, keepawake.Words{Msgs: zh, Clock: w.Clock}
	for _, d := range keepawake.Presets {
		r := keepawake.Request{Kind: keepawake.For, Duration: d}
		// By their names in both languages, and Keep Awake's words.
		add(awakePrefix+strconv.Itoa(int(d/time.Minute)), w.Title(r, now), entry{awake: awakeStart, request: r},
			append([]string{wen.Title(r, now), wzh.Title(r, now)}, names...))
	}
	// Found by no name: find shows it for what is typed.
	l.items = append(l.items, search.Item{ID: awakeTypedID})
	l.entries = append(l.entries, entry{awake: awakeHint})
}

// isAwake reports whether the row is one of Keep Awake's.
func (l *launcher) isAwake(r panelRow) bool {
	return r.title == "" && !r.calc && r.own < 0 && r.item >= 0 && l.entries[r.item].awake != awakeNone
}

// awakeRows puts Keep Awake's rows in the results (copy.md 18.1): for a
// length or time typed, Stop Keeping Awake first while on, then its row,
// and none of the others, which would say the same; for "keep awake
// until" alone, the hint's row; for Keep Awake's name followed by what
// it can't read, Keep Awake itself. While on, Stop Keeping Awake comes
// first of Keep Awake's rows (design.md 2.10).
func (l *launcher) awakeRows(rows []panelRow) []panelRow {
	if l.g.awake == nil {
		return rows
	}
	toggle, typed := l.itemAt[awakeToggleID], l.itemAt[awakeTypedID]
	on := l.g.awakeActive()
	if m, ok := keepawake.ParseQuery(l.query); ok {
		e := entry{awake: awakeHint}
		title := locales.AwakeUntilBare()
		if !m.NeedTime {
			e = entry{awake: awakeStart, request: m.Request}
			title = awakeWords().Title(m.Request, l.g.awakeNow())
		}
		l.items[typed].Title, l.entries[typed] = title, e
		var front []panelRow
		if on {
			front = append(front, panelRow{item: toggle, own: -1})
		}
		front = append(front, panelRow{item: typed, own: -1})
		return append(front, slices.DeleteFunc(rows, l.isAwake)...)
	}
	// Together where the first of them is, in copy.md 18.1's order (that
	// of their items): Keep Awake or Stop Keeping Awake, then the presets.
	first := slices.IndexFunc(rows, l.isAwake)
	var awake []panelRow
	for _, r := range rows {
		if l.isAwake(r) {
			awake = append(awake, r)
		}
	}
	if !slices.ContainsFunc(awake, func(r panelRow) bool { return r.item == toggle }) && keepawake.Commanded(l.query) {
		awake = append(awake, panelRow{item: toggle, own: -1})
	}
	if len(awake) == 0 {
		return rows
	}
	slices.SortFunc(awake, func(a, b panelRow) int { return a.item - b.item })
	rows = slices.DeleteFunc(rows, l.isAwake)
	return slices.Insert(rows, min(max(first, 0), len(rows)), awake...)
}

// firstRow is the row chosen first: the first row, but for the row of
// what is typed under Stop Keeping Awake, so that Return starts what was
// typed (design.md 2.10).
func (l *launcher) firstRow() int {
	i := firstRow(l.rows)
	if i < 0 || i+1 >= len(l.rows) || l.g.awake == nil || l.rows[i].item < 0 {
		return i
	}
	if r := l.rows[i]; l.isAwake(r) && l.entries[r.item].awake == awakeStop && l.rows[i+1].item == l.itemAt[awakeTypedID] && l.isAwake(l.rows[i+1]) {
		return i + 1
	}
	return i
}

// awakeSymbol is the symbol of a Keep Awake row: the cup filled for Stop
// Keeping Awake, as the tray icon is while on (design.md 2.10).
func awakeSymbol(e entry) string {
	if e.awake == awakeStop {
		return "cup.and.saucer.fill"
	}
	return "cup.and.saucer"
}

// awakeSubtitle is the small text of a Keep Awake row: what is left, on
// Stop Keeping Awake (the only action with one, design.md 2.10); the hint
// on "keep awake until".
func (l *launcher) awakeSubtitle(e entry) string {
	switch {
	case e.awake == awakeHint:
		return locales.AwakeTypeTime()
	case e.awake == awakeStop && l.g.awakeActive():
		return awakeWords().Detail(l.g.awake.Left())
	}
	return ""
}

// awakeAct runs a Keep Awake row: the panel stays, the row taking its
// new name in place, and the notice says what is on now (design.md
// 2.10, copy.md 18.2).
func (l *launcher) awakeAct(e entry) {
	g := l.g
	// The choice stays on the row that ran, the rows rebuilt, or the one
	// in its place (Keep Awake and Stop Keeping Awake): the typed row as
	// Stop Keeping Awake comes over it or goes.
	at := l.chosen
	var was any
	if at >= 0 && at < len(l.rows) {
		was = l.rowKey(at)
	}
	defer func() {
		for i := range l.rows {
			if was != nil && l.rowKey(i) == was {
				l.choose(i)
				return
			}
		}
		l.choose(min(at, len(l.rows)-1))
	}()
	switch e.awake {
	case awakeStop:
		g.stopAwake()
		l.sayAction(actionNotice{icon: "cup.and.saucer", text: locales.AwakeNoticeStopped()})
	case awakeStart:
		if g.keepAwake(e.request) {
			l.sayAction(actionNotice{icon: "cup.and.saucer.fill", text: awakeWords().Notice(g.awake.Left())})
		}
	}
}
