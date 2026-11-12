package clipboard

import (
	"slices"
	"strings"
	"sync"
	"time"
)

// Interval is how often the watcher looks at the pasteboard's change
// count by default. Reading the count is cheap; the contents are read
// only when it changed.
const Interval = 500 * time.Millisecond

// pruneEvery is how often a running watcher drops expired entries.
const pruneEvery = 10 * time.Minute

// WatcherOptions configure a Watcher.
type WatcherOptions struct {
	// Interval between looks at the pasteboard; 0 is Interval.
	Interval time.Duration
	// Disabled starts the watcher with recording off.
	Disabled bool
	// IgnoredApps are bundle IDs whose copies are not kept; nil is
	// DefaultIgnoredApps.
	IgnoredApps []string
	// Frontmost tells the app copied from; nil is FrontmostApp.
	Frontmost func() App
}

// Watcher records what the user copies into a Store, by looking at the
// pasteboard's change count every Interval on a goroutine and reading
// the contents when it changed. It does not record contents marked
// concealed, transient or generated (Skipped), copies made in an ignored
// app, content over MaxSize or empty, the store's own writes, nor
// anything while recording is off.
type Watcher struct {
	store *Store
	pb    Pasteboard
	every time.Duration
	front func() App

	mu      sync.Mutex
	enabled bool
	ignored []string
	last    int
	started bool
	stop    chan struct{}
	done    chan struct{}
}

// NewWatcher returns a watcher recording into store from its pasteboard.
// It does nothing until Start.
func NewWatcher(store *Store, opts WatcherOptions) *Watcher {
	w := &Watcher{store: store, pb: store.opts.Pasteboard, every: opts.Interval, front: opts.Frontmost, enabled: !opts.Disabled}
	if w.every <= 0 {
		w.every = Interval
	}
	if w.front == nil {
		w.front = FrontmostApp
	}
	w.ignored = slices.Clone(opts.IgnoredApps)
	if opts.IgnoredApps == nil {
		w.ignored = slices.Clone(DefaultIgnoredApps)
	}
	return w
}

// Start begins watching. What is on the pasteboard already is not
// recorded: only what is copied from now on.
func (w *Watcher) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.started || w.pb == nil {
		return
	}
	w.started = true
	w.last = w.pb.ChangeCount()
	w.stop, w.done = make(chan struct{}), make(chan struct{})
	go w.run(w.stop, w.done)
}

// Stop ends watching and waits for the goroutine to finish.
func (w *Watcher) Stop() {
	w.mu.Lock()
	if !w.started {
		w.mu.Unlock()
		return
	}
	w.started = false
	stop, done := w.stop, w.done
	w.mu.Unlock()
	close(stop)
	<-done
}

func (w *Watcher) run(stop, done chan struct{}) {
	defer close(done)
	tick := time.NewTicker(w.every)
	defer tick.Stop()
	lastPrune := time.Now()
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
			w.Poll()
			if time.Since(lastPrune) >= pruneEvery {
				lastPrune = time.Now()
				w.store.Prune()
			}
		}
	}
}

// SetEnabled turns recording on or off. While off, copies are not read,
// and turning it back on does not record what was copied meanwhile.
func (w *Watcher) SetEnabled(on bool) {
	w.mu.Lock()
	w.enabled = on
	w.mu.Unlock()
}

// Enabled reports whether recording is on.
func (w *Watcher) Enabled() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.enabled
}

// SetIgnoredApps replaces the bundle IDs whose copies are not kept.
func (w *Watcher) SetIgnoredApps(bundleIDs []string) {
	w.mu.Lock()
	w.ignored = slices.Clone(bundleIDs)
	w.mu.Unlock()
}

// IgnoredApps lists the bundle IDs whose copies are not kept.
func (w *Watcher) IgnoredApps() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.Clone(w.ignored)
}

func (w *Watcher) isIgnored(bundleID string) bool {
	if bundleID == "" {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, id := range w.ignored {
		if strings.EqualFold(id, bundleID) {
			return true
		}
	}
	return false
}

// Poll looks at the pasteboard once, recording a change: what the
// watcher's goroutine does every Interval. It reports whether it added
// or moved an entry.
func (w *Watcher) Poll() bool {
	n := w.pb.ChangeCount()
	w.mu.Lock()
	if n == w.last {
		w.mu.Unlock()
		return false
	}
	own, writing := w.store.ownChange(n)
	if writing {
		// The store is writing: look again once it has.
		w.mu.Unlock()
		return false
	}
	w.last = n
	enabled := w.enabled
	w.mu.Unlock()
	if own || !enabled || w.store.Unreadable() {
		return false
	}
	if Skipped(w.pb.Types()) {
		return false
	}
	app := w.front()
	if w.isIgnored(app.BundleID) {
		return false
	}
	c, err := w.pb.Read()
	if err != nil {
		return false
	}
	_, err = w.store.Add(c, app)
	return err == nil
}
