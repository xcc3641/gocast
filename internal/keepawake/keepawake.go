// Package keepawake keeps the Mac from sleeping while no one uses it
// (design.md 2.10, copy.md 18): until stopped, for a length of time, or
// until a time of day. It is Gocast's Keep Awake without its UI: the
// Keeper, which holds one power assertion at a time and ends it on time;
// parsing what the user types ("1h30m", "90分钟", "until 5pm", "下午5点");
// and the words that say what is left ("还剩 1 小时 23 分", "到 17:30").
//
// The Keeper lives in Gocast's main process only, never in the terminal
// host (`Gocast --terminal-host`): the assertion is the process's, and
// macOS drops it when the process exits, so quitting, crashing or
// relaunching for an update ends Keep Awake (copy.md 18.6). The app still
// calls Close as it quits.
package keepawake

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Kind is how a session of Keep Awake ends.
type Kind int

const (
	// Off: not keeping awake.
	Off Kind = iota
	// Forever: until the user stops it.
	Forever
	// For: for a length of time.
	For
	// Until: until a time of day, today or tomorrow.
	Until
)

// Change is what changed, in an Event.
type Change int

const (
	// Started: a session started, maybe in place of another one.
	Started Change = iota + 1
	// Stopped: the user stopped it.
	Stopped
	// Ended: it reached its end on its own. Nothing tells the user
	// (copy.md 18.2) but the tray icon.
	Ended
	// Switched: the assertion changed type (SetAllowDisplaySleep) while
	// keeping awake.
	Switched
	// Ticked: what is left, as Left says it, changed: a whole minute
	// passed, or "tomorrow" became today.
	Ticked
)

// Event tells of a change. State is the state just after it; Seq counts
// the events, so that one delivered late can be told from a newer one.
type Event struct {
	Change Change
	State  State
	Seq    uint64
}

// State is a snapshot of the Keeper.
type State struct {
	Kind Kind
	// Started is when the session started, Deadline when it ends (zero
	// for Forever), both on the wall clock.
	Started, Deadline time.Time
	// Type is the assertion held, "" while off.
	Type AssertionType
	// AllowDisplaySleep: "Let the screen turn off while keeping awake".
	AllowDisplaySleep bool
}

// Active reports whether the Mac is being kept awake.
func (s State) Active() bool { return s.Kind != Off }

// Remaining is the time left until Deadline as of now: 0 when off or
// Forever, or past it.
func (s State) Remaining(now time.Time) time.Duration {
	if s.Deadline.IsZero() {
		return 0
	}
	if r := s.Deadline.Sub(now.Round(0)); r > 0 {
		return r
	}
	return 0
}

// Left is what is left of a session as the UI says it (copy.md 18.0,
// 18.3): "还剩 1 小时 23 分", "还剩不到 1 分钟", "到 17:30", "到明天
// 08:00", "直到你停止".
type Left struct {
	Kind Kind
	// For: the time left in hours and minutes, rounded up to a whole
	// minute, so that "1 hr 30 min" shows for the first minute of a
	// session of 90 and the text changes on the whole minute; LastMinute
	// (Hours and Minutes 0) under a minute.
	Hours, Minutes int
	LastMinute     bool
	// Until: the time of day it ends, and whether that is on the day
	// after now's.
	At       time.Time
	Tomorrow bool
}

// Left says what is left as of now.
func (s State) Left(now time.Time) Left {
	now = now.Round(0)
	switch s.Kind {
	case For:
		r := s.Deadline.Sub(now)
		if r < time.Minute {
			return Left{Kind: For, LastMinute: true}
		}
		m := int((r + time.Minute - 1) / time.Minute)
		return Left{Kind: For, Hours: m / 60, Minutes: m % 60}
	case Until:
		return Left{Kind: Until, At: s.Deadline, Tomorrow: isTomorrow(s.Deadline, now)}
	}
	return Left{Kind: s.Kind}
}

// isTomorrow reports whether t is on the day after now's, in now's
// location.
func isTomorrow(t, now time.Time) bool {
	y, m, d := now.Date()
	ty, tm, td := t.In(now.Location()).Date()
	return time.Date(ty, tm, td, 0, 0, 0, 0, time.UTC).After(time.Date(y, m, d, 0, 0, 0, 0, time.UTC))
}

// Options set up a Keeper. The zero value is the real one.
type Options struct {
	// Backend holds the assertions; nil: System().
	Backend Backend
	// Now is the wall clock; nil: time.Now.
	Now func() time.Time
	// After waits; nil: time.After. The Keeper only ever waits a few
	// seconds at a time (poll), then reads Now again.
	After func(time.Duration) <-chan time.Time
	// AllowDisplaySleep is the setting at the start.
	AllowDisplaySleep bool
}

// poll is the longest the Keeper waits before it reads the wall clock
// again. Go's timers run on the monotonic clock, which stops while the
// Mac sleeps (the lid, the Apple menu): after a wake, a session past its
// end ends within poll, and the text left catches up as soon.
const poll = 5 * time.Second

// ErrClosed is the error of Start after Close.
var ErrClosed = errors.New("keepawake: closed")

// Keeper keeps the Mac awake: one session at a time, one assertion for
// it. Its methods are safe to call from any goroutine.
type Keeper struct {
	backend Backend
	now     func() time.Time
	after   func(time.Duration) <-chan time.Time

	mu        sync.Mutex
	allow     bool
	s         State
	held      Assertion
	shown     Left // what Left said at the last event, for Ticked
	seq       uint64
	listeners map[int]func(Event)
	nextID    int
	closed    bool
	loop      bool
	kick      chan struct{}
	done      chan struct{}
}

// New returns a Keeper, keeping nothing awake.
func New(o Options) *Keeper {
	k := &Keeper{
		backend:   o.Backend,
		now:       o.Now,
		after:     o.After,
		allow:     o.AllowDisplaySleep,
		listeners: map[int]func(Event){},
		kick:      make(chan struct{}, 1),
		done:      make(chan struct{}),
	}
	if k.backend == nil {
		k.backend = System()
	}
	if k.now == nil {
		k.now = time.Now
	}
	if k.after == nil {
		k.after = time.After
	}
	k.s.AllowDisplaySleep = k.allow
	return k
}

// clock is the wall clock, without the monotonic reading: comparisons
// with a deadline then go by the wall clock, which goes on while the Mac
// sleeps.
func (k *Keeper) clock() time.Time { return k.now().Round(0) }

// OnChange calls f after each change, on the goroutine that made it (the
// Keeper's own for Ended and Ticked), never with the Keeper locked: f may
// call the Keeper. It returns a func that stops the calls.
func (k *Keeper) OnChange(f func(Event)) (cancel func()) {
	k.mu.Lock()
	defer k.mu.Unlock()
	id := k.nextID
	k.nextID++
	k.listeners[id] = f
	return func() {
		k.mu.Lock()
		defer k.mu.Unlock()
		delete(k.listeners, id)
	}
}

// event makes the Event of c, with k locked, and returns the func that
// sends it, to call once k is unlocked.
func (k *Keeper) event(c Change, now time.Time) func() {
	k.seq++
	k.shown = k.s.Left(now)
	e := Event{Change: c, State: k.s, Seq: k.seq}
	fs := make([]func(Event), 0, len(k.listeners))
	for _, f := range k.listeners {
		fs = append(fs, f)
	}
	return func() {
		for _, f := range fs {
			f(e)
		}
	}
}

// State is the Keeper's state now.
func (k *Keeper) State() State {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.s
}

// Remaining is the time left of a session with an end; false when off or
// Forever.
func (k *Keeper) Remaining() (time.Duration, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.s.Deadline.IsZero() {
		return 0, false
	}
	return k.s.Remaining(k.clock()), true
}

// Left says what is left now.
func (k *Keeper) Left() Left {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.s.Left(k.clock())
}

// Start keeps the Mac awake as r says, in place of the session running,
// if any: the new assertion is made before the old one is released, so
// there is always exactly one. When r is not valid or the assertion
// can't be made, it returns the error and leaves the session running as
// it was.
func (k *Keeper) Start(r Request) error {
	if err := r.Validate(); err != nil {
		return err
	}
	k.mu.Lock()
	if k.closed {
		k.mu.Unlock()
		return ErrClosed
	}
	now := k.clock()
	typ := typeFor(k.allow)
	a, err := k.backend.Create(typ, AssertionName)
	if err != nil {
		k.mu.Unlock()
		return err
	}
	if k.s.Active() {
		_ = k.backend.Release(k.held)
	}
	k.held = a
	k.s = State{Kind: r.Kind, Started: now, Deadline: r.Deadline(now), Type: typ, AllowDisplaySleep: k.allow}
	send := k.event(Started, now)
	k.wake()
	k.mu.Unlock()
	send()
	return nil
}

// Stop stops keeping awake; it reports whether it was.
func (k *Keeper) Stop() bool {
	k.mu.Lock()
	if !k.s.Active() {
		k.mu.Unlock()
		return false
	}
	k.end()
	send := k.event(Stopped, k.clock())
	k.wake()
	k.mu.Unlock()
	send()
	return true
}

// end releases the assertion and turns off, with k locked.
func (k *Keeper) end() {
	_ = k.backend.Release(k.held)
	k.held = 0
	k.s = State{AllowDisplaySleep: k.allow}
}

// SetAllowDisplaySleep applies "Let the screen turn off while keeping
// awake" (copy.md 18.4): at once to the session running, if any, whose
// assertion it replaces with one of the other type, the new one made
// before the old one is released. When the new one can't be made, it
// returns the error and nothing changes.
func (k *Keeper) SetAllowDisplaySleep(allow bool) error {
	k.mu.Lock()
	if allow == k.allow {
		k.mu.Unlock()
		return nil
	}
	if !k.s.Active() {
		k.allow = allow
		k.s.AllowDisplaySleep = allow
		k.mu.Unlock()
		return nil
	}
	typ := typeFor(allow)
	a, err := k.backend.Create(typ, AssertionName)
	if err != nil {
		k.mu.Unlock()
		return fmt.Errorf("keepawake: switching to %s: %w", typ, err)
	}
	_ = k.backend.Release(k.held)
	k.held = a
	k.allow = allow
	k.s.Type = typ
	k.s.AllowDisplaySleep = allow
	send := k.event(Switched, k.clock())
	k.mu.Unlock()
	send()
	return nil
}

// Check reads the wall clock: it ends a session past its end (Ended) and
// says when what is left has changed (Ticked). The Keeper checks every
// few seconds on its own; the app may also call it when the Mac wakes.
func (k *Keeper) Check() {
	k.mu.Lock()
	if !k.s.Active() {
		k.mu.Unlock()
		return
	}
	now := k.clock()
	var send func()
	switch {
	case !k.s.Deadline.IsZero() && !now.Before(k.s.Deadline):
		k.end()
		send = k.event(Ended, now)
	case k.s.Left(now) != k.shown:
		send = k.event(Ticked, now)
	}
	k.mu.Unlock()
	if send != nil {
		send()
	}
}

// Close releases the assertion, without an event, and stops the Keeper:
// for when Gocast quits.
func (k *Keeper) Close() {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.closed {
		return
	}
	k.closed = true
	if k.s.Active() {
		k.end()
	}
	close(k.done)
}

// wake has the Keeper's goroutine look at the state again, starting it
// the first time, with k locked.
func (k *Keeper) wake() {
	if !k.loop {
		k.loop = true
		go k.run()
	}
	select {
	case k.kick <- struct{}{}:
	default:
	}
}

// run waits for what is next (wait) and checks, until Close.
func (k *Keeper) run() {
	for {
		k.mu.Lock()
		d, ok := k.wait(k.clock())
		k.mu.Unlock()
		var c <-chan time.Time
		if ok {
			c = k.after(d)
		}
		select {
		case <-c:
			k.Check()
		case <-k.kick:
		case <-k.done:
			return
		}
	}
}

// wait is how long to wait from now before Check, with k locked: until
// the session ends, what is left changes (a whole minute for For,
// midnight for Until), or poll, whichever comes first; false when there
// is nothing to wait for.
func (k *Keeper) wait(now time.Time) (time.Duration, bool) {
	if k.s.Deadline.IsZero() {
		return 0, false
	}
	r := k.s.Deadline.Sub(now)
	if r <= 0 {
		return 0, true
	}
	d := r
	switch k.s.Kind {
	case For:
		if r >= time.Minute {
			// Left rounds up: it says m minutes down to r = (m-1) minutes.
			m := (r + time.Minute - 1) / time.Minute
			d = r - (m-1)*time.Minute
		}
	case Until:
		y, mo, day := now.Date()
		if midnight := time.Date(y, mo, day+1, 0, 0, 0, 0, now.Location()); midnight.Before(k.s.Deadline) {
			d = min(d, midnight.Sub(now))
		}
	}
	return min(d, poll), true
}
