package app

import (
	"image"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gocast/internal/settings"
)

// The pictures place their small terminal by the floating terminal's own
// maths on a 72×34 work area, 2 from its edges (design.md 3.7); At
// Pointer, centred on the pointer's x, its bottom at 31, the line typed
// on under the pointer's tip.
func TestThumbRect(t *testing.T) {
	r := func(x, y, w, h int) mygo.Rectangle { return mygo.Rectangle{X: x, Y: y, Width: w, Height: h} }
	for _, c := range []struct {
		pos        string
		wPct, hPct float64
		want       mygo.Rectangle
	}{
		// The default 50% × 60%: 36 × 20.
		{settings.PositionTop, 50, 60, r(18, 5, 36, 20)},
		{settings.PositionCenter, 50, 60, r(18, 10, 36, 20)},
		{settings.PositionBottom, 50, 60, r(18, 15, 36, 20)},
		{settings.PositionLeft, 50, 60, r(2, 10, 36, 20)},
		{settings.PositionRight, 50, 60, r(34, 10, 36, 20)},
		{settings.PositionPointer, 50, 60, r(18, 11, 36, 20)}, // its bottom at 31
		// The largest, 100% × 90%: 72 × 31, against the edges.
		{settings.PositionTop, 100, 90, r(0, 3, 72, 31)},
		{settings.PositionBottom, 100, 90, r(0, 6, 72, 31)},
		{settings.PositionLeft, 100, 90, r(0, 4, 72, 31)},
		{settings.PositionPointer, 100, 90, r(0, 5, 72, 26)}, // its top no higher than 5
		// The least, 30% × 30%: 22 × 10; never under 12 × 8.
		{settings.PositionRight, 30, 30, r(48, 15, 22, 10)},
		{settings.PositionPointer, 30, 30, r(25, 21, 22, 10)},
		{settings.PositionCenter, 5, 5, r(30, 16, 12, 8)},
		{settings.PositionPointer, 5, 5, r(30, 23, 12, 8)},
		// At Pointer narrow or wide, kept 2 inside the sides.
		{settings.PositionPointer, 95, 50, r(2, 14, 68, 17)},
	} {
		if got := thumbRect(c.pos, c.wPct, c.hPct); got != c.want {
			t.Errorf("%s at %v%% × %v%%: %+v, want %+v", c.pos, c.wPct, c.hPct, got, c.want)
		}
	}
	// Any size: the five fixed positions are the floating terminal's
	// placement of the scaled size, with the gap scaled down, and inside
	// the screen's work area, as the pointer's.
	wa := thumbWorkArea
	for w := settings.MinTerminalWidth; w <= settings.MaxTerminalWidth; w++ {
		for h := settings.MinTerminalHeight; h <= settings.MaxTerminalHeight; h++ {
			pw, ph := percentOf(wa.Width, w), percentOf(wa.Height, h)
			for _, pos := range settings.TerminalPositions {
				got := thumbRect(pos, float64(w), float64(h))
				if pos != settings.PositionPointer {
					if want := placeIn(pos, wa, pw, ph, mygo.Point{}, 0, thumbGap).rect; got != want {
						t.Fatalf("%s at %d%% × %d%%: %+v, the placement's %+v", pos, w, h, got, want)
					}
				}
				if pos == settings.PositionPointer && (got.Y+got.Height != thumbTermBottom || got.Y < thumbTermTop || got.Y > thumbLineY-1) {
					t.Fatalf("at pointer at %d%% × %d%%: %+v, not under the line", w, h, got)
				}
				if got.X < wa.X || got.Y < wa.Y || got.X+got.Width > wa.X+wa.Width || got.Y+got.Height > wa.Y+wa.Height {
					t.Fatalf("%s at %d%% × %d%%: %+v, out of the work area", pos, w, h, got)
				}
			}
		}
	}
}

// The 悬浮终端 page of Settings with each position chosen, light and dark,
// in both languages, for a look (GOCAST_SCREENS).
func TestScreensTermPosition(t *testing.T) {
	dir := screensDir(t)
	whole := image.Rect(0, 0, prefsWidth, 4000)
	for _, dark := range []bool{false, true} {
		mode := "light"
		if dark {
			mode = "dark"
		}
		for _, locale := range []string{"zh-Hans", "en"} {
			for _, pos := range settings.TerminalPositions {
				_, tt := positionPrefs(t, locale, pos)
				tt.SetDark(dark)
				tt.Frame()
				saveScreen(t, tt, dir, "termpos-"+locale+"-"+pos+"-"+mode, whole)
			}
		}
	}
}

// positionPrefs is Settings on the 悬浮终端 page, in a window tall enough
// not to scroll, at 2×, the terminal at pos and its default size.
func positionPrefs(t *testing.T, locale, pos string) (*prefsWindow, *ui.Tester) {
	t.Helper()
	p, tt, _ := hotkeyPrefs(t, locale)
	p.g.term = &termPanel{g: p.g} // hidden
	p.page = pagePresets
	p.g.settings.SetTermPosition(pos)
	st := p.g.settings
	p.termPos = st.TermPosition()
	p.autoUpdate, p.calculator, p.hideBlur = st.AutoUpdates(), st.Calculator(), !st.KeepTerminalOnBlur
	p.fontSize, p.termHeight = float64(st.Font()), float64(st.TermHeight())
	p.termWidth = float64(st.TermWidthOn(1512))
	p.checkDefaultFolder()
	tt.SetScale(2)
	tt.SetSize(prefsWidth, 700)
	return p, tt
}
