package app

// thumbplay.go plays the pictures of 出现位置: the small terminal of one
// shows as the floating terminal does at its position, scaled down, as
// the picture is chosen or the pointer rests on it (design.md 3.7).

import (
	"time"

	"github.com/egoist/mygo/ui"

	"gocast/internal/settings"
)

// How a picture plays (design.md 3.7).
const (
	thumbPlayFor    = 360 * time.Millisecond // the small terminal fading in as it slides
	thumbPointerFor = 120 * time.Millisecond // At Pointer's pointer fading in, before
	thumbFadeFor    = 120 * time.Millisecond // the small terminal fading in alone, with less motion
	thumbRestFor    = 150 * time.Millisecond // the pointer resting on a picture, before it plays
	thumbSlide      = 4                      // how far the small terminal comes from, in points
)

// thumbEase is the floating terminal's own curve as it shows (design.md
// 1.5).
var thumbEase = ui.CubicBezier(0.2, 0.8, 0.2, 1)

// thumbPlay is a picture playing: the position it is of, "" for none,
// since when, and whether its small terminal only fades in, as the
// desktop asks for less motion.
type thumbPlay struct {
	pos   string
	start time.Time
	fade  bool
}

// thumbPose is how a picture draws its small terminal, with its line typed
// on: dx, dy from where it rests, at the opacity term; and At Pointer's
// pointer at the opacity pointer.
type thumbPose struct {
	dx, dy, term, pointer float32
}

// restPose is a picture still.
var restPose = thumbPose{term: 1, pointer: 1}

// length is how long the play lasts.
func (a thumbPlay) length() time.Duration {
	switch {
	case a.fade:
		return thumbFadeFor
	case a.pos == settings.PositionPointer:
		return thumbPointerFor + thumbPlayFor
	}
	return thumbPlayFor
}

// playing reports whether the play goes on at now.
func (a thumbPlay) playing(now time.Time) bool {
	return a.pos != "" && now.Sub(a.start) < a.length()
}

// poseOf is how the picture of pos draws at now: still, but for the one
// playing, whose small terminal comes from thumbSlide away, the way the
// floating terminal comes at that position, fading in; At Pointer's after
// its pointer faded in. With less motion, it only fades in.
func (a thumbPlay) poseOf(pos string, now time.Time) thumbPose {
	if pos != a.pos || !a.playing(now) {
		return restPose
	}
	t := max(now.Sub(a.start), 0)
	if a.fade {
		return thumbPose{term: thumbEase(fraction(t, thumbFadeFor)), pointer: 1}
	}
	pose := thumbPose{pointer: 1}
	if pos == settings.PositionPointer {
		pose.pointer = thumbEase(fraction(t, thumbPointerFor))
		t -= thumbPointerFor
	}
	e := thumbEase(fraction(t, thumbPlayFor))
	pose.term = e
	away := (1 - e) * thumbSlide
	switch pos {
	case settings.PositionTop, settings.PositionCenter:
		pose.dy = -away // falls
	case settings.PositionBottom, settings.PositionPointer:
		pose.dy = away // rises
	case settings.PositionLeft:
		pose.dx = -away
	case settings.PositionRight:
		pose.dx = away
	}
	return pose
}

// fraction is how far t is into d, from 0 to 1.
func fraction(t, d time.Duration) float32 {
	return float32(min(max(t, 0), d)) / float32(d)
}

// choosePicture plays the picture of pos, just chosen, by the pointer or
// the keys: the one playing goes still.
func (p *prefsWindow) choosePicture(c *ui.Context, pos string) {
	p.play = thumbPlay{pos: pos, start: c.Now(), fade: c.Preferences().ReduceMotion}
}

// restOn follows the pointer over the pictures, over that of pos ("" for
// none): coming to another picture, the one playing goes still; resting
// thumbRestFor on one not chosen, it plays, once until the pointer leaves
// it. Not with less motion, where only choosing plays.
func (p *prefsWindow) restOn(c *ui.Context, pos string) {
	now := c.Now()
	if pos != p.restPos {
		p.restPos, p.restSince, p.restPlayed = pos, now, false
		if pos != "" && p.play.pos != pos {
			p.play = thumbPlay{}
		}
	}
	if pos == "" || pos == p.termPos || p.restPlayed || c.Preferences().ReduceMotion {
		return
	}
	if wait := thumbRestFor - now.Sub(p.restSince); wait > 0 {
		c.After(wait)
		return
	}
	p.play = thumbPlay{pos: pos, start: now}
	p.restPlayed = true
}

// playOn asks for the frames of the picture playing, and forgets it once
// played.
func (p *prefsWindow) playOn(c *ui.Context) {
	if p.play.playing(c.Now()) {
		c.AnimationFrame()
	} else {
		p.play = thumbPlay{}
	}
}
