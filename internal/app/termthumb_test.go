package app

import (
	"image"
	"testing"
	"time"

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

// A picture plays its small terminal coming in as the floating terminal
// does at its position, scaled down (design.md 3.7): from 4 away, fading
// in, over 360 ms on the terminal's own curve; At Pointer's pointer fades
// in over 120 ms first, then its terminal rises; with less motion, it only
// fades in, over 120 ms. The others stay still.
func TestThumbPlayPoses(t *testing.T) {
	start := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	at := func(d time.Duration) time.Time { return start.Add(d) }
	from := map[string]thumbPose{
		settings.PositionTop:     {dy: -4},
		settings.PositionCenter:  {dy: -4},
		settings.PositionBottom:  {dy: 4},
		settings.PositionLeft:    {dx: -4},
		settings.PositionRight:   {dx: 4},
		settings.PositionPointer: {dy: 4},
	}
	for _, pos := range settings.TerminalPositions {
		a := thumbPlay{pos: pos, start: start}
		lead := time.Duration(0)
		if pos == settings.PositionPointer {
			lead = thumbPointerFor
			if got := a.poseOf(pos, at(0)); got.pointer != 0 || got.term != 0 {
				t.Errorf("%s at 0: %+v, its pointer not yet in", pos, got)
			}
			if got := a.poseOf(pos, at(60*time.Millisecond)); got.pointer <= 0 || got.pointer >= 1 || got.term != 0 {
				t.Errorf("%s at 60 ms: %+v, its pointer half in, the terminal not", pos, got)
			}
		}
		want := from[pos]
		want.pointer = 1
		if got := a.poseOf(pos, at(lead)); got != want {
			t.Errorf("%s at %v: %+v, want %+v", pos, lead, got, want)
		}
		mid := a.poseOf(pos, at(lead+thumbPlayFor/2))
		if mid.term <= 0.5 || mid.term >= 1 || mid.dx*want.dx < 0 || mid.dy*want.dy < 0 || abs32(mid.dx)+abs32(mid.dy) >= 2 {
			t.Errorf("%s half way: %+v, more than half in and on its way from %+v", pos, mid, want)
		}
		if !a.playing(at(lead + thumbPlayFor - time.Millisecond)) {
			t.Errorf("%s: done before %v", pos, lead+thumbPlayFor)
		}
		if got := a.poseOf(pos, at(lead+thumbPlayFor)); got != restPose || a.playing(at(lead+thumbPlayFor)) {
			t.Errorf("%s at %v: %+v, not still", pos, lead+thumbPlayFor, got)
		}
		for _, other := range settings.TerminalPositions {
			if other != pos && a.poseOf(other, at(lead)) != restPose {
				t.Errorf("%s playing: %s moves", pos, other)
			}
		}
		fade := thumbPlay{pos: pos, start: start, fade: true}
		if got := fade.poseOf(pos, at(0)); got != (thumbPose{pointer: 1}) {
			t.Errorf("%s with less motion at 0: %+v", pos, got)
		}
		if got := fade.poseOf(pos, at(60*time.Millisecond)); got.dx != 0 || got.dy != 0 || got.term <= 0 || got.term >= 1 {
			t.Errorf("%s with less motion at 60 ms: %+v, moving", pos, got)
		}
		if fade.playing(at(thumbFadeFor)) {
			t.Errorf("%s with less motion: longer than %v", pos, thumbFadeFor)
		}
	}
}

// The pictures play as design.md 3.7 says: the one chosen, by the pointer
// or the keys, once; one not chosen that the pointer rests on 150 ms,
// once until the pointer leaves it, not one swept over nor the one chosen;
// coming to another, the one playing goes still. With less motion, only
// choosing plays, fading in. Still otherwise, with no frames asked for.
func TestPositionPicturePlays(t *testing.T) {
	for _, locale := range []string{"zh-Hans", "en"} {
		p, tt := positionPrefs(t, locale, settings.PositionTop)
		tt.Frame()
		center := func(pos string) (float32, float32) {
			t.Helper()
			a, ok := tt.AccessByRole(ui.RoleRadio, positionLabel(pos))
			if !ok {
				t.Fatalf("%s: no %s: %+v", locale, positionLabel(pos), tt.AccessAll())
			}
			return a.Bounds.X + a.Bounds.W/2, a.Bounds.Y + ringBoxH/2
		}
		playing := func(when, want string) {
			t.Helper()
			got := ""
			if p.play.playing(tt.Now()) {
				got = p.play.pos
			}
			if got != want {
				t.Errorf("%s: %s: %q playing, want %q", locale, when, got, want)
			}
		}
		playing("opened", "")
		if tt.FramePending() {
			t.Errorf("%s: opened, frames asked for", locale)
		}

		// Chosen by the pointer.
		tt.ClickAt(center(settings.PositionBottom))
		tt.Frame()
		playing("底部 clicked", settings.PositionBottom)
		tt.Advance(thumbPlayFor / 2)
		playing("底部 half way", settings.PositionBottom)
		// Another chosen half way: it starts, the first goes still.
		tt.ClickAt(center(settings.PositionLeft))
		tt.Frame()
		playing("左侧 clicked", settings.PositionLeft)
		tt.Advance(thumbPlayFor)
		playing("played", "")
		tt.Advance(50 * time.Millisecond) // the frame after its last
		if tt.FramePending() {
			t.Errorf("%s: played, frames still asked for", locale)
		}

		// Rested on, not chosen.
		tt.Move(center(settings.PositionRight))
		tt.Frame()
		tt.Advance(thumbRestFor - 20*time.Millisecond)
		playing("右侧 swept over", "")
		tt.Advance(20 * time.Millisecond)
		playing("右侧 rested on", settings.PositionRight)
		tt.Advance(thumbPlayFor / 2)
		tt.Move(center(settings.PositionCenter))
		tt.Frame()
		playing("on to 中央", "")
		tt.Advance(thumbRestFor)
		playing("中央 rested on", settings.PositionCenter)
		tt.Advance(thumbPlayFor)
		tt.Advance(time.Second)
		playing("中央 rested on, played", "")
		// Off and back on: again.
		tt.Move(10, 650)
		tt.Frame()
		tt.Move(center(settings.PositionCenter))
		tt.Frame()
		tt.Advance(thumbRestFor)
		playing("中央 rested on again", settings.PositionCenter)
		tt.Advance(thumbPlayFor)
		// The one chosen does not play.
		tt.Move(center(settings.PositionLeft))
		tt.Frame()
		tt.Advance(time.Second)
		playing("左侧, chosen, rested on", "")

		// Chosen by the keys, At Pointer's lasting its pointer's 120 ms
		// more.
		tt.Move(10, 650)
		tt.Frame()
		for range 40 {
			if tt.Focused(positionLabel(settings.PositionLeft)) {
				break
			}
			tt.Key(0, ui.KeyTab)
		}
		tt.Key(0, ui.KeyRight)
		tt.Key(0, ui.KeyRight)
		tt.Frame()
		playing("鼠标处 by the keys", settings.PositionPointer)
		tt.Advance(thumbPlayFor)
		playing("鼠标处 after 360 ms", settings.PositionPointer)
		tt.Advance(thumbPointerFor)
		playing("鼠标处 played", "")

		// With less motion.
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		tt.ClickAt(center(settings.PositionTop))
		tt.Frame()
		playing("顶部 clicked, less motion", settings.PositionTop)
		if !p.play.fade {
			t.Errorf("%s: with less motion, 顶部 moves", locale)
		}
		tt.Advance(thumbFadeFor)
		playing("顶部 faded in", "")
		tt.Move(center(settings.PositionBottom))
		tt.Frame()
		tt.Advance(time.Second)
		playing("底部 rested on, less motion", "")
	}
}
