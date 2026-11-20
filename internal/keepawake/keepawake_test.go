package keepawake

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// fakeBackend holds assertions in memory and logs what it is asked.
type fakeBackend struct {
	mu   sync.Mutex
	next Assertion
	live map[Assertion]AssertionType
	log  []string
	// emptied: the live assertions went from some to none, which a
	// replacement must never do.
	emptied int
	fail    bool
}

func newFakeBackend() *fakeBackend { return &fakeBackend{live: map[Assertion]AssertionType{}} }

func (b *fakeBackend) Create(t AssertionType, name string) (Assertion, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.fail {
		return 0, errors.New("no assertion")
	}
	if name != AssertionName {
		return 0, fmt.Errorf("name %q", name)
	}
	b.next++
	b.live[b.next] = t
	b.log = append(b.log, fmt.Sprintf("create %d %s", b.next, t))
	return b.next, nil
}

func (b *fakeBackend) Release(a Assertion) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.live[a]; !ok {
		return fmt.Errorf("release of %d, not held", a)
	}
	delete(b.live, a)
	if len(b.live) == 0 {
		b.emptied++
	}
	b.log = append(b.log, fmt.Sprintf("release %d", a))
	return nil
}

// held is the one assertion live, or "" when none; it fails the test
// when there are more.
func (b *fakeBackend) held(t *testing.T) AssertionType {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.live) > 1 {
		t.Fatalf("%d assertions held: %v", len(b.live), b.log)
	}
	for _, typ := range b.live {
		return typ
	}
	return ""
}

// fakeClock is a wall clock the test moves.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = t
}

func (c *fakeClock) Add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// t0 is the fake clock's start: a Thursday, 14:00:00 in Shanghai.
var (
	shanghai = time.FixedZone("CST", 8*3600)
	t0       = time.Date(2026, 11, 19, 14, 0, 0, 0, shanghai)
)

// keeperTest is a Keeper on a fake backend and clock, whose events it
// keeps. Its goroutine never runs out of the test's control: After hands
// the test a channel it never fires unless asked.
type keeperTest struct {
	*Keeper
	backend *fakeBackend
	clock   *fakeClock

	mu     sync.Mutex
	events []Event
}

func newKeeperTest(t *testing.T) *keeperTest {
	kt := &keeperTest{backend: newFakeBackend(), clock: &fakeClock{t: t0}}
	kt.Keeper = New(Options{
		Backend: kt.backend,
		Now:     kt.clock.Now,
		After:   func(time.Duration) <-chan time.Time { return nil },
	})
	kt.OnChange(func(e Event) {
		kt.mu.Lock()
		defer kt.mu.Unlock()
		kt.events = append(kt.events, e)
	})
	t.Cleanup(kt.Close)
	return kt
}

// changes are the changes since the last call.
func (kt *keeperTest) changes() []Change {
	kt.mu.Lock()
	defer kt.mu.Unlock()
	var cs []Change
	for _, e := range kt.events {
		cs = append(cs, e.Change)
	}
	kt.events = nil
	return cs
}

func sameChanges(a, b []Change) bool { return fmt.Sprint(a) == fmt.Sprint(b) }

var (
	forever  = Request{Kind: Forever}
	for90    = Request{Kind: For, Duration: 90 * time.Minute}
	until530 = Request{Kind: Until, Hour: 17, Minute: 30}
)

// Starting holds one display-sleep assertion under Gocast's name, and
// stopping releases it.
func TestStartStop(t *testing.T) {
	kt := newKeeperTest(t)
	if err := kt.Start(forever); err != nil {
		t.Fatal(err)
	}
	if got := kt.backend.held(t); got != PreventDisplaySleep {
		t.Fatalf("held %q, want %q", got, PreventDisplaySleep)
	}
	if s := kt.State(); s.Kind != Forever || !s.Started.Equal(t0) || !s.Deadline.IsZero() || s.Type != PreventDisplaySleep {
		t.Errorf("state %+v", s)
	}
	if _, ok := kt.Remaining(); ok {
		t.Error("Remaining of Forever is ok")
	}
	if !kt.Stop() {
		t.Error("Stop reports it wasn't keeping awake")
	}
	if got := kt.backend.held(t); got != "" {
		t.Errorf("still held %q", got)
	}
	if kt.Stop() {
		t.Error("Stop when off reports it was keeping awake")
	}
	if kt.State().Active() {
		t.Error("still active")
	}
	if got := kt.changes(); !sameChanges(got, []Change{Started, Stopped}) {
		t.Errorf("changes %v", got)
	}
}

// A new session replaces the running one: the new assertion is made
// before the old one goes, and one is left.
func TestReplace(t *testing.T) {
	kt := newKeeperTest(t)
	for _, r := range []Request{forever, for90, until530, for90} {
		if err := kt.Start(r); err != nil {
			t.Fatal(err)
		}
		kt.backend.held(t)
	}
	if kt.backend.emptied != 0 {
		t.Errorf("no assertion held during a replacement: %v", kt.backend.log)
	}
	want := []string{
		"create 1 PreventUserIdleDisplaySleep",
		"create 2 PreventUserIdleDisplaySleep", "release 1",
		"create 3 PreventUserIdleDisplaySleep", "release 2",
		"create 4 PreventUserIdleDisplaySleep", "release 3",
	}
	if fmt.Sprint(kt.backend.log) != fmt.Sprint(want) {
		t.Errorf("log %v, want %v", kt.backend.log, want)
	}
	if s := kt.State(); s.Kind != For || !s.Deadline.Equal(t0.Add(90*time.Minute)) {
		t.Errorf("state %+v", s)
	}
	if got := kt.changes(); !sameChanges(got, []Change{Started, Started, Started, Started}) {
		t.Errorf("changes %v", got)
	}
}

// What can't be started, or an assertion the system won't make, leaves
// the running session as it was and says nothing.
func TestStartInvalid(t *testing.T) {
	kt := newKeeperTest(t)
	if err := kt.Start(for90); err != nil {
		t.Fatal(err)
	}
	kt.changes()
	before := kt.State()
	for _, r := range []Request{
		{},
		{Kind: For},
		{Kind: For, Duration: 30 * time.Second},
		{Kind: For, Duration: 90*time.Second + time.Minute},
		{Kind: For, Duration: MaxDuration + time.Minute},
		{Kind: Until, Hour: 24},
		{Kind: Until, Hour: 17, Minute: 60},
		{Kind: Until, Hour: -1},
	} {
		if err := kt.Start(r); !errors.Is(err, ErrInvalid) {
			t.Errorf("Start(%+v) = %v, want ErrInvalid", r, err)
		}
	}
	kt.backend.fail = true
	if err := kt.Start(forever); err == nil {
		t.Error("Start with no assertion made: no error")
	}
	if s := kt.State(); s != before {
		t.Errorf("state %+v, want %+v", s, before)
	}
	if len(kt.backend.log) != 1 || kt.backend.held(t) == "" {
		t.Errorf("assertions touched: %v", kt.backend.log)
	}
	if got := kt.changes(); len(got) != 0 {
		t.Errorf("changes %v", got)
	}
}

// A session ends on its own at its deadline, releasing its assertion,
// and not a moment before.
func TestTimeout(t *testing.T) {
	kt := newKeeperTest(t)
	if err := kt.Start(Request{Kind: For, Duration: 2 * time.Minute}); err != nil {
		t.Fatal(err)
	}
	kt.clock.Add(2*time.Minute - time.Nanosecond)
	kt.Check()
	if !kt.State().Active() {
		t.Fatal("ended early")
	}
	if r, ok := kt.Remaining(); !ok || r != time.Nanosecond {
		t.Errorf("Remaining = %v, %v", r, ok)
	}
	kt.clock.Add(time.Nanosecond)
	kt.Check()
	if kt.State().Active() || kt.backend.held(t) != "" {
		t.Fatalf("not ended: %+v %v", kt.State(), kt.backend.log)
	}
	if got := kt.changes(); !sameChanges(got, []Change{Started, Ticked, Ended}) {
		t.Errorf("changes %v", got)
	}
	kt.Check()
	if got := kt.changes(); len(got) != 0 {
		t.Errorf("changes after the end %v", got)
	}
}

// A Mac that slept past the deadline (the wall clock jumped, the
// monotonic one didn't) ends the session at the first check after it
// wakes; a session until a time of day as well.
func TestWakeAfterDeadline(t *testing.T) {
	for _, r := range []Request{for90, until530} {
		kt := newKeeperTest(t)
		if err := kt.Start(r); err != nil {
			t.Fatal(err)
		}
		kt.clock.Add(10 * time.Hour)
		kt.Check()
		if kt.State().Active() || kt.backend.held(t) != "" {
			t.Errorf("%+v: still keeping awake after the wake", r)
		}
		if got := kt.changes(); !sameChanges(got, []Change{Started, Ended}) {
			t.Errorf("%+v: changes %v", r, got)
		}
	}
}

// The deadline is wall-clock time: from a Now with a monotonic reading
// (the real clock's), it keeps only the wall time, which goes on while
// the Mac sleeps.
func TestWallClockDeadline(t *testing.T) {
	clock := &fakeClock{t: time.Now()}
	k := New(Options{Backend: newFakeBackend(), Now: clock.Now, After: func(time.Duration) <-chan time.Time { return nil }})
	defer k.Close()
	if err := k.Start(for90); err != nil {
		t.Fatal(err)
	}
	if s := k.State(); s.Deadline != s.Deadline.Round(0) || s.Started != s.Started.Round(0) {
		t.Error("the deadline has a monotonic reading")
	}
	clock.Add(90 * time.Minute)
	k.Check()
	if k.State().Active() {
		t.Error("not ended")
	}
}

// Letting the screen turn off swaps the running session's assertion for
// one of the other type at once, the new one first; while off it only
// sets the type the next session takes.
func TestSetAllowDisplaySleep(t *testing.T) {
	kt := newKeeperTest(t)
	if err := kt.SetAllowDisplaySleep(true); err != nil {
		t.Fatal(err)
	}
	if len(kt.backend.log) != 0 || len(kt.changes()) != 0 {
		t.Fatalf("while off: %v", kt.backend.log)
	}
	if err := kt.Start(forever); err != nil {
		t.Fatal(err)
	}
	if got := kt.backend.held(t); got != PreventSystemSleep {
		t.Fatalf("held %q", got)
	}
	if err := kt.SetAllowDisplaySleep(false); err != nil {
		t.Fatal(err)
	}
	if got := kt.backend.held(t); got != PreventDisplaySleep {
		t.Errorf("held %q after the switch", got)
	}
	if err := kt.SetAllowDisplaySleep(false); err != nil {
		t.Fatal(err)
	}
	want := []string{"create 1 PreventUserIdleSystemSleep", "create 2 PreventUserIdleDisplaySleep", "release 1"}
	if fmt.Sprint(kt.backend.log) != fmt.Sprint(want) || kt.backend.emptied != 0 {
		t.Errorf("log %v, want %v", kt.backend.log, want)
	}
	if s := kt.State(); s.Type != PreventDisplaySleep || s.AllowDisplaySleep || s.Kind != Forever {
		t.Errorf("state %+v", s)
	}
	if got := kt.changes(); !sameChanges(got, []Change{Started, Switched}) {
		t.Errorf("changes %v", got)
	}

	// The system won't make the other one: the session keeps its own.
	kt.backend.fail = true
	if err := kt.SetAllowDisplaySleep(true); err == nil {
		t.Error("no error")
	}
	if s := kt.State(); s.Type != PreventDisplaySleep || s.AllowDisplaySleep || kt.backend.held(t) != PreventDisplaySleep {
		t.Errorf("after a failed switch: %+v", s)
	}
}

// What is left counts down by whole minutes, rounded up, to "less than a
// minute" in the last one; Ticked comes when it changes, and only then.
func TestLeftByMinute(t *testing.T) {
	kt := newKeeperTest(t)
	if err := kt.Start(for90); err != nil {
		t.Fatal(err)
	}
	kt.changes()
	for _, c := range []struct {
		at     time.Duration
		want   Left
		ticked bool
	}{
		{0, Left{Kind: For, Hours: 1, Minutes: 30}, false},
		{time.Nanosecond, Left{Kind: For, Hours: 1, Minutes: 30}, false},
		{time.Minute - time.Nanosecond, Left{Kind: For, Hours: 1, Minutes: 30}, false},
		{time.Minute, Left{Kind: For, Hours: 1, Minutes: 29}, true},
		{30*time.Minute - time.Second, Left{Kind: For, Hours: 1, Minutes: 1}, true},
		{30 * time.Minute, Left{Kind: For, Hours: 1}, true},
		{30*time.Minute + time.Second, Left{Kind: For, Hours: 1}, false},
		{31 * time.Minute, Left{Kind: For, Minutes: 59}, true},
		{89 * time.Minute, Left{Kind: For, Minutes: 1}, true},
		{89*time.Minute + time.Nanosecond, Left{Kind: For, LastMinute: true}, true},
		{90*time.Minute - time.Nanosecond, Left{Kind: For, LastMinute: true}, false},
	} {
		kt.clock.Set(t0.Add(c.at))
		if got := kt.Left(); got != c.want {
			t.Errorf("at +%v: %+v, want %+v", c.at, got, c.want)
		}
		kt.Check()
		if got := kt.changes(); !sameChanges(got, map[bool][]Change{true: {Ticked}}[c.ticked]) {
			t.Errorf("at +%v: changes %v", c.at, got)
		}
	}
}

// A session until a time of day says "tomorrow" until midnight, then
// not, and tells of the change.
func TestLeftUntilTomorrow(t *testing.T) {
	kt := newKeeperTest(t)
	kt.clock.Set(time.Date(2026, 11, 19, 23, 0, 0, 0, shanghai))
	if err := kt.Start(Request{Kind: Until, Hour: 8}); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 20, 8, 0, 0, 0, shanghai)
	if got := kt.Left(); got != (Left{Kind: Until, At: at, Tomorrow: true}) {
		t.Errorf("before midnight %+v", got)
	}
	if d, ok := kt.wait(kt.clock.Now()); !ok || d != poll {
		t.Errorf("wait an hour before midnight: %v %v", d, ok)
	}
	kt.clock.Set(time.Date(2026, 11, 19, 23, 59, 58, 0, shanghai))
	if d, _ := kt.wait(kt.clock.Now()); d != 2*time.Second {
		t.Errorf("wait two seconds before midnight: %v", d)
	}
	kt.changes()
	kt.clock.Set(time.Date(2026, 11, 20, 0, 0, 0, 0, shanghai))
	kt.Check()
	if got := kt.Left(); got != (Left{Kind: Until, At: at}) {
		t.Errorf("after midnight %+v", got)
	}
	if got := kt.changes(); !sameChanges(got, []Change{Ticked}) {
		t.Errorf("changes %v", got)
	}
}

// The Keeper waits until the next whole minute left, or the end, but
// never longer than poll, so that it reads the wall clock soon after a
// wake.
func TestWait(t *testing.T) {
	kt := newKeeperTest(t)
	if _, ok := kt.wait(t0); ok {
		t.Error("waits while off")
	}
	_ = kt.Start(forever)
	if _, ok := kt.wait(t0); ok {
		t.Error("waits for Forever")
	}
	_ = kt.Start(for90)
	for _, c := range []struct{ at, want time.Duration }{
		{0, poll},
		{time.Minute - 1500*time.Millisecond, 1500 * time.Millisecond},
		{time.Minute, poll},
		{90*time.Minute - time.Second, time.Second},
		{90 * time.Minute, 0},
		{91 * time.Minute, 0},
	} {
		if got, ok := kt.wait(t0.Add(c.at)); !ok || got != c.want {
			t.Errorf("at +%v: wait %v, want %v", c.at, got, c.want)
		}
	}
}

// The Keeper's own goroutine ends a session when its wait is over, by
// the wall clock.
func TestLoopEnds(t *testing.T) {
	clock := &fakeClock{t: t0}
	backend := newFakeBackend()
	asked := make(chan time.Duration, 10)
	fire := make(chan time.Time)
	k := New(Options{Backend: backend, Now: clock.Now, After: func(d time.Duration) <-chan time.Time {
		asked <- d
		return fire
	}})
	defer k.Close()
	ended := make(chan Event, 1)
	k.OnChange(func(e Event) {
		if e.Change == Ended {
			ended <- e
		}
	})
	if err := k.Start(Request{Kind: For, Duration: time.Minute}); err != nil {
		t.Fatal(err)
	}
	if d := <-asked; d != poll {
		t.Errorf("waits %v", d)
	}
	clock.Add(time.Minute)
	fire <- clock.Now()
	e := <-ended
	if e.State.Active() || backend.held(t) != "" {
		t.Errorf("ended %+v, %v", e.State, backend.log)
	}
}

// Closing releases the assertion, and nothing starts after it.
func TestClose(t *testing.T) {
	kt := newKeeperTest(t)
	_ = kt.Start(for90)
	kt.Close()
	if kt.backend.held(t) != "" {
		t.Error("held after Close")
	}
	if err := kt.Start(forever); !errors.Is(err, ErrClosed) {
		t.Errorf("Start after Close: %v", err)
	}
	kt.Close()
}

// A time of day already past today, or this very minute, is tomorrow's.
func TestDeadline(t *testing.T) {
	for _, c := range []struct {
		h, m int
		want time.Time
	}{
		{17, 30, time.Date(2026, 11, 19, 17, 30, 0, 0, shanghai)},
		{14, 1, time.Date(2026, 11, 19, 14, 1, 0, 0, shanghai)},
		{14, 0, time.Date(2026, 11, 20, 14, 0, 0, 0, shanghai)},
		{8, 0, time.Date(2026, 11, 20, 8, 0, 0, 0, shanghai)},
		{0, 0, time.Date(2026, 11, 20, 0, 0, 0, 0, shanghai)},
	} {
		r := Request{Kind: Until, Hour: c.h, Minute: c.m}
		if got := r.Deadline(t0); !got.Equal(c.want) {
			t.Errorf("%d:%02d: %v, want %v", c.h, c.m, got, c.want)
		}
		if got, want := r.Tomorrow(t0), c.want.Day() == 20; got != want {
			t.Errorf("%d:%02d: Tomorrow %v", c.h, c.m, got)
		}
	}
}
