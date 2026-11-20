package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"gocast/internal/presets"
	"gocast/locales"
)

// prefsWindowTester is Settings on page, at the window's own size, with
// after run at the end of each build, if not nil, given Settings.
func prefsWindowTester(t *testing.T, locale, page string, after func(c *ui.Context, p *prefsWindow)) (*prefsWindow, *ui.Tester) {
	t.Helper()
	p, _, _ := hotkeyPrefs(t, locale)
	p.g.term = &termPanel{g: p.g} // hidden
	p.goTo(page)
	tt := ui.NewTester(func(c *ui.Context) {
		p.view(c)
		if after != nil {
			after(c, p)
		}
	}, prefsWidth, prefsHeight)
	tt.Frame()
	return p, tt
}

// Settings moves as a window does, its title bar hidden (design.md 4.1):
// by the band across its top, over the sidebar and the pages, and by the
// sidebar under its tabs, to the window's bottom; not by the pages under
// the band, with their sliders and lists. Before, nothing moved it.
func TestPrefsDragRegions(t *testing.T) {
	for _, page := range []string{pageShortcuts, pagePresets, pageGeneral} {
		var bar, side ui.Rect
		p, tt := prefsWindowTester(t, "zh-Hans", page, func(c *ui.Context, p *prefsWindow) {
			bar, side = p.bar.Bounds(c), p.sideBar.Bounds(c)
		})
		tt.Frame()
		if want := (ui.Rect{W: prefsWidth, H: prefsBarHeight}); bar != want {
			t.Errorf("%s: the band at %+v, want %+v", page, bar, want)
		}
		last, ok := tt.AccessByRole(ui.RoleListItem, locales.PageAbout())
		if !ok {
			t.Fatalf("%s: no last tab: %+v", page, tt.AccessAll())
		}
		if end := last.Bounds.Y + last.Bounds.H; side.X != 0 || side.W != 200 || side.Y != end || side.Y+side.H != prefsHeight {
			t.Errorf("%s: the sidebar moves the window at %+v, its last tab ending at %v", page, side, end)
		}
		// The tabs still choose their page.
		if err := tt.Click(locales.PageAbout()); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if p.page != pageAbout {
			t.Errorf("from %s: the tab chose %s", page, p.page)
		}
	}
}

// Over a dialog's dimmed window the band still moves the window, where a
// press used to close the dialog; the dialog, at its tallest, stays under
// the band, and a press on the dimmed window below the band still closes
// it.
func TestPrefsDragOverDialog(t *testing.T) {
	for _, locale := range []string{"zh-Hans", "en"} {
		p, tt := prefsWindowTester(t, locale, pagePresets, nil)
		p.editID, p.draft, p.editing = "", presets.Preset{}, true // a new one
		p.envOpen, p.envText = true, "A=1\nB=2\nC=3\nD=4"
		p.nameProblem = locales.PresetNameTaken("a name long enough to take two lines, if the error wraps at all")
		tt.Frame()
		var dialog ui.Rect
		for _, a := range tt.AccessAll() {
			if a.Role == ui.RoleDialog {
				dialog = a.Bounds
			}
		}
		if dialog.H == 0 {
			t.Fatalf("%s: no dialog: %+v", locale, tt.AccessAll())
		}
		if dialog.Y < prefsBarHeight {
			t.Errorf("%s: the dialog at %+v, under the band", locale, dialog)
		}
		tt.ClickAt(dialog.X/2, prefsBarHeight/2)
		tt.Frame()
		if !p.editing {
			t.Errorf("%s: a press on the band closed the dialog", locale)
		}
		tt.ClickAt(dialog.X/2, prefsBarHeight+10)
		tt.Frame()
		if p.editing {
			t.Errorf("%s: a press on the dimmed window below the band left the dialog open", locale)
		}
	}
}
