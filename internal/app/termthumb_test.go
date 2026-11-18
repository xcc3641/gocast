package app

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"gocast/internal/settings"
)

// The thumbnail places its small terminal by the floating terminal's own
// maths on a 72×34 work area, 2 from its edges (design.md 3.7); Near
// Pointer, always under the pointer's tip.
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
		{settings.PositionPointer, 50, 60, r(18, 16, 36, 19)}, // 20 high, under the tip: 19
		// The largest, 100% × 90%: 72 × 31, against the edges.
		{settings.PositionTop, 100, 90, r(0, 3, 72, 31)},
		{settings.PositionBottom, 100, 90, r(0, 6, 72, 31)},
		{settings.PositionLeft, 100, 90, r(0, 4, 72, 31)},
		{settings.PositionPointer, 100, 90, r(0, 16, 72, 19)},
		// The least, 30% × 30%: 22 × 10; never under 12 × 8.
		{settings.PositionRight, 30, 30, r(48, 15, 22, 10)},
		{settings.PositionPointer, 30, 30, r(25, 16, 22, 10)},
		{settings.PositionCenter, 5, 5, r(30, 16, 12, 8)},
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
					if want := placeIn(pos, wa, pw, ph, mygo.Point{}, thumbGap).rect; got != want {
						t.Fatalf("%s at %d%% × %d%%: %+v, the placement's %+v", pos, w, h, got, want)
					}
				}
				if got.X < wa.X || got.Y < wa.Y || got.X+got.Width > wa.X+wa.Width || got.Y+got.Height > wa.Y+wa.Height {
					t.Fatalf("%s at %d%% × %d%%: %+v, out of the work area", pos, w, h, got)
				}
			}
		}
	}
}

// Choosing a position moves the thumbnail's terminal there in 200 ms, from
// wherever it is drawn, a new choice on the way included; the pointer
// fades in or out in 120 ms meanwhile; with Reduce Motion the terminal
// does not move but fades out where it was and in where it goes.
func TestThumbMotion(t *testing.T) {
	t0 := time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)
	at := func(ms int) time.Time { return t0.Add(time.Duration(ms) * time.Millisecond) }
	rect := func(pos string) ui.Rect { return uiRect(thumbRect(pos, 50, 60)) }
	one := func(name string, terms []thumbLayer, want ui.Rect) {
		t.Helper()
		if len(terms) != 1 || terms[0].r != want || terms[0].alpha != 1 {
			t.Errorf("%s: %+v, want %+v", name, terms, want)
		}
	}
	var th posThumb
	th = posThumb{pos: settings.PositionTop} // as the page opens
	terms, tip, moving := th.frame(at(0), 50, 60)
	one("opened", terms, rect(settings.PositionTop))
	if tip != 0 || moving {
		t.Errorf("opened: pointer %v, moving %v", tip, moving)
	}

	th.set(settings.PositionBottom, at(0), 50, 60, false)
	terms, _, moving = th.frame(at(0), 50, 60)
	one("as it changes", terms, rect(settings.PositionTop))
	top, bottom := rect(settings.PositionTop), rect(settings.PositionBottom)
	k := thumbEase(0.5)
	terms, _, moving = th.frame(at(100), 50, 60)
	half := ui.Rect{X: top.X, Y: top.Y + (bottom.Y-top.Y)*k, W: top.W, H: top.H}
	one("half way", terms, half)
	if !moving || k <= 0.5 { // ease-in-out (0.4, 0, 0.2, 1) is past the middle by then
		t.Errorf("half way: moving %v, eased %v", moving, k)
	}
	terms, _, moving = th.frame(at(200), 50, 60)
	one("there", terms, bottom)
	if moving {
		t.Error("still moving at 200 ms")
	}

	// Another choice on the way: from where it was drawn, no jump.
	th.set(settings.PositionTop, at(300), 50, 60, false)
	terms, _, _ = th.frame(at(400), 50, 60)
	from := terms[0].r
	th.set(settings.PositionLeft, at(400), 50, 60, false)
	terms, _, _ = th.frame(at(400), 50, 60)
	one("changed on the way", terms, from)
	terms, _, _ = th.frame(at(600), 50, 60)
	one("then there", terms, rect(settings.PositionLeft))

	// The pointer fades in, as the terminal moves, and out.
	th.set(settings.PositionPointer, at(1000), 50, 60, false)
	if _, tip, _ = th.frame(at(1060), 50, 60); tip != 0.5 {
		t.Errorf("pointer at 60 ms: %v", tip)
	}
	if terms, tip, moving = th.frame(at(1150), 50, 60); tip != 1 || !moving {
		t.Errorf("pointer at 150 ms: %v, moving %v", tip, moving)
	}
	th.set(settings.PositionRight, at(2000), 50, 60, false)
	if _, tip, _ = th.frame(at(2120), 50, 60); tip != 0 {
		t.Errorf("pointer gone at 120 ms: %v", tip)
	}

	// Reduce Motion: a crossfade in place.
	th.set(settings.PositionCenter, at(3000), 50, 60, true)
	terms, _, moving = th.frame(at(3060), 50, 60)
	if len(terms) != 2 || terms[0] != (thumbLayer{rect(settings.PositionRight), 0.5}) || terms[1] != (thumbLayer{rect(settings.PositionCenter), 0.5}) || !moving {
		t.Errorf("crossfading: %+v, moving %v", terms, moving)
	}
	terms, _, moving = th.frame(at(3120), 50, 60)
	one("faded in", terms, rect(settings.PositionCenter))
	if moving {
		t.Error("still fading at 120 ms")
	}

	// The sizes follow the sliders at once, still or moving.
	terms, _, _ = th.frame(at(4000), 80, 40)
	one("resized", terms, uiRect(thumbRect(settings.PositionCenter, 80, 40)))
}

// The 悬浮终端 page of Settings with each position chosen, light and dark,
// and the thumbnail half way from the top to the bottom, for a look
// (GOCAST_SCREENS).
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
		// Moving: top to bottom, 100 ms in.
		p, tt := positionPrefs(t, "zh-Hans", settings.PositionTop)
		tt.SetDark(dark)
		tt.Frame()
		tt.Advance(time.Second)
		p.thumb.set(settings.PositionBottom, tt.Now(), p.termWidth, p.termHeight, false)
		tt.Frame()
		tt.Advance(100 * time.Millisecond)
		img := tt.Image()
		f, err := os.Create(filepath.Join(dir, "termpos-moving-"+mode+".png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		f.Close()
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
	p.termPos, p.thumb = positionLabel(st.TermPosition()), posThumb{pos: st.TermPosition()}
	p.autoUpdate, p.calculator, p.hideBlur = st.AutoUpdates(), st.Calculator(), !st.KeepTerminalOnBlur
	p.fontSize, p.termHeight = float64(st.Font()), float64(st.TermHeight())
	p.termWidth = float64(st.TermWidthOn(1512))
	p.checkDefaultFolder()
	tt.SetScale(2)
	tt.SetSize(prefsWidth, 700)
	return p, tt
}
