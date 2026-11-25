package app

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"gocast/internal/presets"
	"gocast/internal/settings"
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

// Settings moves as a window does, its title bar hidden (design.md 4.1,
// 4.2, 4.3): by its sidebar, but for the tabs, which still choose their
// page, and by the page's header across the top; not by the page under
// the header, with its sliders and lists.
func TestPrefsDragRegions(t *testing.T) {
	for _, page := range []string{pageShortcuts, pagePresets, pageGeneral, pageAbout} {
		var bar, header, side ui.Rect
		p, tt := prefsWindowTester(t, "zh-Hans", page, func(c *ui.Context, p *prefsWindow) {
			bar, header, side = p.bar.Bounds(c), p.header.Bounds(c), p.sideBar.Bounds(c)
		})
		tt.Frame()
		if want := (ui.Rect{X: 200, W: prefsWidth - 200, H: prefsBarHeight}); header != want {
			t.Errorf("%s: the header at %+v, want %+v", page, header, want)
		}
		if want := (ui.Rect{W: 200, H: prefsHeight}); side != want {
			t.Errorf("%s: the sidebar at %+v, want %+v", page, side, want)
		}
		if bar != (ui.Rect{}) {
			t.Errorf("%s: with no dialog, a band at %+v over the page", page, bar)
		}
		last, ok := tt.AccessByRole(ui.RoleListItem, locales.PageAbout())
		if !ok {
			t.Fatalf("%s: no last tab: %+v", page, tt.AccessAll())
		}
		if last.Bounds.Y < prefsBarHeight || last.Bounds.Y+last.Bounds.H > prefsHeight/2 {
			t.Errorf("%s: the last tab at %+v, leaving no sidebar under the tabs", page, last.Bounds)
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

// Over a dialog's dimmed window a band across the window's top moves the
// window, where a press used to close the dialog; the dialog, at its
// tallest, stays under the band, and a press on the dimmed window below
// the band still closes it.
func TestPrefsDragOverDialog(t *testing.T) {
	for _, locale := range []string{"zh-Hans", "en"} {
		var bar ui.Rect
		p, tt := prefsWindowTester(t, locale, pagePresets, func(c *ui.Context, p *prefsWindow) { bar = p.bar.Bounds(c) })
		p.editID, p.draft, p.editing = "", presets.Preset{}, true // a new one
		p.envOpen, p.envText = true, "A=1\nB=2\nC=3\nD=4"
		p.nameProblem = locales.PresetNameTaken("a name long enough to take two lines, if the error wraps at all")
		tt.Frame()
		tt.Frame() // the band laid out, read back
		var dialog ui.Rect
		for _, a := range tt.AccessAll() {
			if a.Role == ui.RoleDialog {
				dialog = a.Bounds
			}
		}
		if dialog.H == 0 {
			t.Fatalf("%s: no dialog: %+v", locale, tt.AccessAll())
		}
		if want := (ui.Rect{W: prefsWidth, H: prefsBarHeight}); bar != want {
			t.Errorf("%s: over the dialog, the band at %+v, want %+v", locale, bar, want)
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

// The header frosts what scrolls under it, fading in over 180 ms as the
// page leaves its top or the pointer comes over the header, and out as
// neither is; at once with less motion; never on Shortcuts, whose page
// does not scroll (design.md 4.3).
func TestPrefsHeaderFrost(t *testing.T) {
	for _, locale := range []string{"zh-Hans", "en"} {
		inLocale(t, locale)
		p, tt := prefsWindowTester(t, locale, pagePresets, nil)
		frost := func(when string, want float32) {
			t.Helper()
			if p.frost != want {
				t.Errorf("%s: %s: the frost at %v, want %v", locale, when, p.frost, want)
			}
		}
		frost("at the top", 0)
		tt.Scroll(500, 300, 0, 120)
		tt.Frame()
		if p.scrollOf(pagePresets).Y <= 0 {
			t.Fatalf("%s: the page did not scroll", locale)
		}
		tt.Advance(90 * time.Millisecond)
		if p.frost <= 0 || p.frost >= 1 {
			t.Errorf("%s: half way in, the frost at %v", locale, p.frost)
		}
		tt.Advance(100 * time.Millisecond)
		frost("scrolled", 1)
		tt.Scroll(500, 300, 0, -1000)
		tt.Frame()
		tt.Advance(90 * time.Millisecond)
		if p.frost <= 0 || p.frost >= 1 {
			t.Errorf("%s: half way out, the frost at %v", locale, p.frost)
		}
		tt.Advance(100 * time.Millisecond)
		frost("back at the top", 0)
		tt.Move(500, prefsBarHeight/2)
		tt.Frame()
		tt.Advance(200 * time.Millisecond)
		frost("the pointer over the header", 1)
		tt.Move(500, 300)
		tt.Frame()
		tt.Advance(200 * time.Millisecond)
		frost("the pointer off the header", 0)

		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Scroll(500, 300, 0, 120)
		tt.Frame()
		frost("scrolled, with less motion", 1)
		tt.Scroll(500, 300, 0, -1000)
		tt.Frame()
		frost("back, with less motion", 0)

		p, tt = prefsWindowTester(t, locale, pageShortcuts, nil)
		tt.Scroll(500, 300, 0, 120)
		tt.Move(500, prefsBarHeight/2)
		tt.Frame()
		tt.Advance(200 * time.Millisecond)
		frost("on Shortcuts", 0)
	}
}

// The Settings style of design.md 4.1–4.3 and 3.7, for a look
// (GOCAST_SCREENS): each page, light and dark, in both languages, at the
// window's size; 悬浮终端 scrolled under its frosted header, with and
// without presets; and the frames of each picture of 出现位置 playing, and
// playing with less motion.
func TestScreensSettingsStyle(t *testing.T) {
	dir := screensDir(t)
	full := image.Rect(0, 0, prefsWidth, prefsHeight)
	some := []presets.Preset{
		{ID: "claude", Name: "Claude Code", Dir: "~/code/gocast", Command: "claude", Default: true},
		{ID: "dev", Name: "开发服务器", Dir: "~/code/site", Command: "bun dev"},
	}
	for _, dark := range []bool{false, true} {
		mode := "light"
		if dark {
			mode = "dark"
		}
		for _, locale := range []string{"zh-Hans", "en"} {
			shot := func(name, page string, setup func(p *prefsWindow, tt *ui.Tester)) {
				p, tt := positionPrefs(t, locale, settings.PositionTop)
				screenIcons(t, p)
				tt.SetSize(prefsWidth, prefsHeight)
				p.goTo(page)
				tt.SetDark(dark)
				tt.Frame()
				if setup != nil {
					setup(p, tt)
					tt.Frame()
				}
				saveScreen(t, tt, dir, "style-"+locale+"-"+name+"-"+mode, full)
			}
			for _, page := range []string{pageShortcuts, pagePresets, pageClipboard, pageGeneral, pageAbout} {
				shot(page, page, nil)
			}
			shot("presets-some", pagePresets, func(p *prefsWindow, tt *ui.Tester) { p.g.presets = some })
			shot("presets-scrolled", pagePresets, func(p *prefsWindow, tt *ui.Tester) {
				p.g.presets = some
				tt.Frame()
				tt.Scroll(500, 300, 0, 260)
			})
			shot("general-scrolled", pageGeneral, func(p *prefsWindow, tt *ui.Tester) { tt.Scroll(500, 300, 0, 1000) })
			shot("header-hovered", pagePresets, func(p *prefsWindow, tt *ui.Tester) { tt.Move(600, prefsBarHeight/2) })
		}
	}

	// The pictures playing: each clicked in turn, frame by frame, at 2×,
	// the row of pictures only.
	for _, reduce := range []bool{false, true} {
		p, tt := positionPrefs(t, "zh-Hans", settings.PositionTop)
		tt.SetPreferences(ui.Preferences{ReduceMotion: reduce})
		tt.Frame()
		group, ok := tt.AccessByRole(ui.RoleRadioGroup, locales.GeneralPosition())
		if !ok {
			t.Fatalf("no 出现位置: %+v", tt.AccessAll())
		}
		row := image.Rect(int(group.Bounds.X), int(group.Bounds.Y), int(group.Bounds.X+group.Bounds.W), int(group.Bounds.Y+group.Bounds.H))
		name := "play"
		if reduce {
			name = "play-reduced"
		}
		order := append([]string{settings.PositionCenter}, settings.TerminalPositions...) // 顶部 chosen first: from 中央 to it
		for _, pos := range order {
			a, _ := tt.AccessByRole(ui.RoleRadio, positionLabel(pos))
			tt.ClickAt(a.Bounds.X+a.Bounds.W/2, a.Bounds.Y+ringBoxH/2)
			tt.Frame()
			for ms := 0; ms <= 480; ms += 40 {
				saveFrame(t, tt, dir, fmt.Sprintf("style-%s-%s-%03d", name, pos, ms), row)
				tt.Advance(40 * time.Millisecond)
			}
			tt.Advance(time.Second)
		}
		if p.termPos != settings.PositionPointer {
			t.Errorf("clicked through to %s", p.termPos)
		}
	}
}

// saveFrame writes the part r of the frame as it is, at 2×, as name.png.
func saveFrame(t *testing.T, tt *ui.Tester, dir, name string, r image.Rectangle) {
	t.Helper()
	img := tt.Image()
	r = image.Rect(r.Min.X*2, r.Min.Y*2, r.Max.X*2, r.Max.Y*2).Intersect(img.Bounds())
	file, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, img.SubImage(r)); err != nil {
		t.Fatal(err)
	}
}

// Settings shows the font size of the user's Ghostty config while they
// never chose one, and the note that the config gives the look; a size
// stepped to is saved as chosen, 13 too, and is the terminal's from then
// on whatever the config says (settings.Settings.FontSize).
func TestPrefsGhosttyFontSize(t *testing.T) {
	useGhosttyConfig(t, map[string]string{ghosttyConfig: "font-size = 14\nbackground = #102030\n"})
	p, tt := prefsWindowTester(t, "en", pagePresets, nil)
	p.showLook() // as Settings shows
	tt.Frame()
	if !tt.HasText("14") || !tt.HasText(locales.GeneralTermLookConfigHint()) {
		t.Fatalf("Ghostty's 14 and the note not shown: %q", tt.Texts())
	}
	stepper, ok := tt.AccessByRole(ui.RoleStepper, locales.GeneralFontSize())
	if !ok {
		t.Fatal("no stepper")
	}
	b := stepper.Bounds
	down := func() { tt.ClickAt(b.X+b.W/2, b.Y+b.H*3/4); tt.Frame() }
	down()
	if p.g.settings.FontSize != 13 || p.g.term.fontSize() != 13 || !tt.HasText("13") {
		t.Errorf("stepped down to 13: saved %v, the terminal's %v", p.g.settings.FontSize, p.g.term.fontSize())
	}
	if s, err := settings.Load(p.g.settingsPath()); err != nil || s.FontSize != 13 {
		t.Errorf("saved %v, %v", s.FontSize, err)
	}
}
