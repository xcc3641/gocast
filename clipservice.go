package main

// clipservice.go keeps the clipboard history for the app: the store
// (internal/clipboard), read on a goroutine as Gocast starts, the watcher
// recording what the user copies, as Settings › Clipboard says, and the
// hooks that paste, which tests and the development modes replace.
//
// The history is kept encrypted in clipboard/ of Gocast's folder, its key
// in keys/clipboard.key there (clipboard.KeyFile), outside the history
// and out of backups. The Keychain is not used: an update of an ad-hoc
// signed app would have to ask for its item.
//
// Development hooks (dev-notes.md), so that the history can be tried
// without the user's history, pasteboard or keystrokes:
//
//   - GOCAST_CLIPBOARD_DIR=<dir> keeps the history in dir, its key in
//     memory (a history left there by an earlier run can't be read, as
//     with a key file gone). Nothing is written to the system pasteboard
//     and ⌘V is never pressed: copies and pastes go to a pasteboard in
//     memory.
//   - GOCAST_CLIPBOARD_SEED=1, only with GOCAST_CLIPBOARD_DIR, fills the
//     history with demo items, over whatever was there, the watcher off,
//     and the pasteboard in memory; =empty leaves it empty. Pasting is
//     taken as allowed. Without GOCAST_CLIPBOARD_DIR it is ignored, so
//     that it never touches the user's history.
//   - GOCAST_CLIPBOARD_FORCE_NO_AX=1 takes pasting as not allowed.

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"gocast/internal/apps"
	"gocast/internal/clipboard"
	"gocast/internal/running"
)

// clipService is the clipboard history of the app.
type clipService struct {
	g       *gocast
	store   *clipboard.Store // nil until read
	watcher *clipboard.Watcher
	loaded  bool

	// dry: a development run, which never writes the system pasteboard
	// nor presses ⌘V (GOCAST_CLIPBOARD_DIR).
	dry  bool
	seed string // GOCAST_CLIPBOARD_SEED: "", "1" or "empty"
	noAX bool   // GOCAST_CLIPBOARD_FORCE_NO_AX

	// asked: Return wanted to paste without the permission, which the
	// banner asks for from then on (design.md 2.8.7).
	asked bool
}

// The hooks of pasting, which tests replace: whether Gocast may press ⌘V
// for the user, the app frontmost, bringing an app to the front and
// waiting for it, and opening System Settings › Accessibility.
var (
	clipTrusted      = clipboard.AccessibilityTrusted
	clipFrontmost    = clipboard.FrontmostApp
	clipActivate     = clipboard.ActivateApp
	clipWaitFront    = clipboard.WaitFrontmost
	clipOpenSettings = clipboard.OpenAccessibilitySettings
)

func newClipService(g *gocast) *clipService {
	s := &clipService{
		g:    g,
		dry:  os.Getenv("GOCAST_CLIPBOARD_DIR") != "",
		noAX: os.Getenv("GOCAST_CLIPBOARD_FORCE_NO_AX") == "1",
	}
	if s.dry { // never seed over the user's history
		s.seed = os.Getenv("GOCAST_CLIPBOARD_SEED")
	}
	return s
}

// keyPath is the file of the history's key: in Gocast's folder, outside
// the history's.
func (s *clipService) keyPath() string {
	return filepath.Join(s.g.dir, "keys", "clipboard.key")
}

// dir is where the history is kept: in Gocast's folder, or the
// development one.
func (s *clipService) dir() string {
	if d := os.Getenv("GOCAST_CLIPBOARD_DIR"); d != "" {
		return d
	}
	return filepath.Join(s.g.dir, "clipboard")
}

// trusted reports whether Gocast may press ⌘V for the user. A seeded run
// takes it as allowed, as it pastes nowhere.
func (s *clipService) trusted() bool {
	if s.noAX {
		return false
	}
	if s.seed != "" {
		return true
	}
	return clipTrusted()
}

// start reads the history on a goroutine, then starts recording.
func (s *clipService) start() {
	set := s.g.settings
	opts := clipboard.Options{Retention: clipboard.Retention(set.ClipboardRetention)}
	var keys clipboard.KeyStore = clipboard.NewKeyFile(s.keyPath())
	switch {
	case s.seed != "":
		opts.Pasteboard = &memPasteboard{}
		keys = &clipboard.MemoryKeys{}
	case s.dry:
		opts.Pasteboard = &dryPasteboard{real: clipboard.SystemPasteboard()}
		keys = &clipboard.MemoryKeys{}
	}
	seeding := s.seed != ""
	locale := mygo.App.Locale()
	clock := &seedClock{}
	if seeding {
		opts.Now = clock.now
	}
	go func() {
		store, err := clipboard.Open(s.dir(), keys, opts)
		if err != nil {
			log.Printf("clipboard: %v", err)
		}
		if seeding {
			if store.Unreadable() { // an earlier run's history, its key gone
				_ = store.Clear()
			}
			if s.seed == "1" {
				seedHistory(store, clock, filepath.Join(s.dir(), "seed-files"), locale)
			}
		}
		s.g.launcher.win.Update(func() { s.opened(store) })
	}()
}

// opened takes the history read, and starts recording into it.
func (s *clipService) opened(store *clipboard.Store) {
	s.store, s.loaded = store, true
	set := s.g.settings
	s.watcher = clipboard.NewWatcher(store, clipboard.WatcherOptions{
		Disabled:    set.ClipboardOff,
		IgnoredApps: set.IgnoredApps(clipboard.DefaultIgnoredApps),
		Frontmost:   frontmostOther,
	})
	if s.seed == "" {
		s.watcher.Start()
	}
	store.SetOnChange(func() {
		s.g.launcher.win.Update(func() {
			s.g.launcher.clip.historyChanged()
			s.g.prefs.win.Invalidate()
		})
	})
	s.g.launcher.clip.historyChanged()
	s.g.prefs.win.Invalidate()
}

// stop saves the history as Gocast quits.
func (s *clipService) stop() {
	if s.watcher != nil {
		s.watcher.Stop()
	}
	if s.store != nil {
		if err := s.store.Close(); err != nil {
			log.Printf("clipboard: %v", err)
		}
	}
}

// recording reports whether the history records what is copied.
func (s *clipService) recording() bool { return !s.g.settings.ClipboardOff }

// setRecording turns recording on or off, and saves it.
func (s *clipService) setRecording(on bool) {
	s.g.settings.ClipboardOff = !on
	s.g.saveSettings()
	if s.watcher != nil {
		s.watcher.SetEnabled(on)
	}
	s.g.launcher.clip.historyChanged()
	s.g.prefs.win.Invalidate()
}

// setRetention keeps unpinned items for r from now on, dropping those
// older at once.
func (s *clipService) setRetention(r clipboard.Retention) {
	s.g.settings.ClipboardRetention = string(r)
	if r == clipboard.DefaultRetention {
		s.g.settings.ClipboardRetention = ""
	}
	s.g.saveSettings()
	if s.store != nil {
		s.store.SetRetention(r)
	}
}

// retention is how long unpinned items are kept.
func (s *clipService) retention() clipboard.Retention {
	r := clipboard.Retention(s.g.settings.ClipboardRetention)
	for _, x := range clipboard.Retentions {
		if x == r {
			return r
		}
	}
	return clipboard.DefaultRetention
}

// setPasteOnReturn turns pasting on Return on or off, and saves it; off,
// the banner asking for the permission goes for good.
func (s *clipService) setPasteOnReturn(on bool) {
	s.g.settings.ClipboardCopyOnly = !on
	if !on {
		s.asked = false
	}
	s.g.saveSettings()
	s.g.launcher.win.Invalidate()
	s.g.prefs.win.Invalidate()
}

// ignoredApps are the bundle IDs of the apps whose copies are not kept.
func (s *clipService) ignoredApps() []string {
	return s.g.settings.IgnoredApps(clipboard.DefaultIgnoredApps)
}

// setIgnoredApps keeps ids as the apps whose copies are not kept.
func (s *clipService) setIgnoredApps(ids []string) {
	s.g.settings.SetIgnoredApps(ids, clipboard.DefaultIgnoredApps)
	s.g.saveSettings()
	if s.watcher != nil {
		s.watcher.SetIgnoredApps(s.ignoredApps())
	}
}

// clear clears the history: all of it but the pinned items, or all of
// it when it can't be read.
func (s *clipService) clear() {
	if s.store == nil {
		return
	}
	if err := s.store.Clear(); err != nil {
		log.Printf("clipboard: clearing: %v", err)
	}
}

// errFilesGone is pasting files no longer there.
var errFilesGone = clipboard.ErrFileMissing

// copy puts the item on the pasteboard and moves it to the top.
func (s *clipService) copy(id string) error {
	if s.store == nil {
		return clipboard.ErrNotFound
	}
	return s.store.CopyToPasteboard(id)
}

// paste puts the item on the pasteboard (its plain text alone, plain),
// brings the app the user was in to the front and presses ⌘V there, on a
// goroutine: the panel is hidden first. A development run pastes nowhere.
func (s *clipService) paste(id string, plain bool, target clipboard.App) error {
	if s.store == nil {
		return clipboard.ErrNotFound
	}
	if e, ok := s.store.Get(id); ok && missingFile(e.Files) {
		return errFilesGone
	}
	if s.dry {
		err := s.store.CopyToPasteboard(id)
		log.Printf("clipboard: a development run: %s put on the pasteboard in memory, not pasted into %q", id, target.Name)
		return err
	}
	if target.PID > 0 {
		clipActivate(target.PID)
	}
	store := s.store
	go func() {
		if target.PID > 0 && !clipWaitFront(target.PID, time.Second) {
			log.Printf("clipboard: %s did not come to the front", target.Name)
		}
		var err error
		if plain {
			err = store.PasteAsPlainText(id)
		} else {
			err = store.Paste(id)
		}
		if err != nil {
			log.Printf("clipboard: pasting: %v", err)
		}
	}()
	return nil
}

// openAccessibility opens System Settings › Privacy & Security ›
// Accessibility.
func (s *clipService) openAccessibility() {
	if err := clipOpenSettings(); err != nil {
		log.Printf("accessibility settings: %v", err)
	}
}

// frontmostOther is the app frontmost, but none when it is Gocast: the app
// to paste into, the app copied from.
func frontmostOther() clipboard.App {
	a := clipFrontmost()
	if a.PID == os.Getpid() || a.BundleID == identifier {
		return clipboard.App{}
	}
	return a
}

// --- Pasteboards of the development runs ---

// memPasteboard is a pasteboard in memory.
type memPasteboard struct {
	mu    sync.Mutex
	count int
	c     clipboard.Content
}

func (p *memPasteboard) ChangeCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.count
}

func (p *memPasteboard) Types() []string { return nil }

func (p *memPasteboard) Read() (clipboard.Content, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.c, nil
}

func (p *memPasteboard) Write(c clipboard.Content) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.c = c
	p.count++
	return nil
}

// dryPasteboard reads the system pasteboard, to record what the user
// copies, and writes to memory.
type dryPasteboard struct {
	real clipboard.Pasteboard
	mem  memPasteboard
}

func (p *dryPasteboard) ChangeCount() int                 { return p.real.ChangeCount() }
func (p *dryPasteboard) Types() []string                  { return p.real.Types() }
func (p *dryPasteboard) Read() (clipboard.Content, error) { return p.real.Read() }
func (p *dryPasteboard) Write(c clipboard.Content) error  { return p.mem.Write(c) }

// --- Demo history (GOCAST_CLIPBOARD_SEED=1) ---

// seedClock is the store's clock while it is seeded: at set, else now.
type seedClock struct {
	mu  sync.Mutex
	set time.Time
}

func (c *seedClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.set.IsZero() {
		return time.Now()
	}
	return c.set
}

func (c *seedClock) at(t time.Time) {
	c.mu.Lock()
	c.set = t
	c.mu.Unlock()
}

// seedHistory fills store with demo items of every kind, copied over the
// last week, some pinned; files made in dir, one of them gone; the apps
// named in the language locale.
func seedHistory(store *clipboard.Store, clock *seedClock, dir, locale string) {
	_ = os.MkdirAll(dir, 0o755)
	file := func(name string, data []byte) string {
		p := filepath.Join(dir, name)
		if data != nil {
			_ = os.WriteFile(p, data, 0o644)
		}
		return p
	}
	// The apps by the names they have in the language Gocast shows, as
	// the system would give them.
	app := func(id string) clipboard.App {
		name := id
		if path := running.AppPath(id); path != "" {
			name = strings.TrimSuffix(filepath.Base(path), ".app")
			if n := apps.Names(path).Localized(locale); n != "" {
				name = n
			}
		}
		return clipboard.App{BundleID: id, Name: name}
	}
	safari, notes, terminal := app("com.apple.Safari"), app("com.apple.Notes"), app("com.apple.Terminal")
	finder, preview, mail := app("com.apple.finder"), app("com.apple.Preview"), app("com.apple.mail")

	banner := demoImage(756, 133, false)
	square := demoImage(480, 480, true)
	notesPDF := file("季度报告-Q3.pdf", bytes.Repeat([]byte("%PDF-1.7 demo\n"), 6000))
	shot := file("截图 2026-11-12 10.24.png", square)
	sketch := file("设计稿 v3.sketch", bytes.Repeat([]byte("sketch"), 40000))
	icon := file("图标.png", banner)
	gone := file("旧版说明.txt", nil) // never made: moved or deleted
	os.Remove(gone)

	now := time.Now()
	items := []struct {
		ago    time.Duration
		c      clipboard.Content
		app    clipboard.App
		pinned bool
	}{
		{6 * 24 * time.Hour, clipboard.Content{Text: "https://www.apple.com/macos/"}, safari, false},
		{5*24*time.Hour + 3*time.Hour, clipboard.Content{Text: "https://github.com/xcc3641/gocast"}, safari, true},
		{4 * 24 * time.Hour, clipboard.Content{Text: "Hi team,\n\nThe 0.1.0 build is ready for a last look before Friday. Please try the clipboard history with your own copies, and tell me anything that feels slow or surprising.\n\nThanks!"}, mail, false},
		{3*24*time.Hour + 2*time.Hour, clipboard.Content{Text: "ssh deploy@10.0.1.24 -p 2222"}, terminal, true},
		{3 * 24 * time.Hour, clipboard.Content{PNG: square}, preview, false},
		{30*time.Hour + 20*time.Minute, clipboard.Content{Files: []string{gone}}, finder, false},
		{28 * time.Hour, clipboard.Content{Files: []string{sketch, icon, gone}}, finder, false},
		{26 * time.Hour, clipboard.Content{Text: "func (s *Store) Paste(id string) error {\n\tif err := s.CopyToPasteboard(id); err != nil {\n\t\treturn err\n\t}\n\treturn pasteNow()\n}"}, terminal, false},
		{3 * time.Hour, clipboard.Content{Files: []string{notesPDF}}, finder, false},
		{2 * time.Hour, clipboard.Content{PNG: banner}, preview, false},
		{70 * time.Minute, clipboard.Content{Files: []string{shot}}, finder, false},
		{55 * time.Minute, clipboard.Content{Text: "https://developer.apple.com/documentation/appkit/nspasteboard/general"}, safari, false},
		{40 * time.Minute, clipboard.Content{Text: "MyGo 是一个用 Go 写原生桌面应用的框架：界面、窗口、菜单和系统集成都在一个 Go 程序里。", HTML: []byte("<p><b>MyGo</b> 是一个用 Go 写原生桌面应用的框架：界面、窗口、菜单和系统集成都在一个 Go 程序里。</p>")}, safari, false},
		{8 * time.Minute, clipboard.Content{Text: "周五发布清单\n\n1. 跑完全部测试，包括剪贴板历史的 1000 条压力测试\n2. 更新 CHANGELOG 和 README 里的「你的数据」\n3. 用固定的签名身份打包，确认更新后不用重新给辅助功能权限\n4. 打 tag 0.1.0 并推送\n\n发布前再看一眼深色和浅色的截图。"}, notes, false},
	}
	for _, it := range items {
		clock.at(now.Add(-it.ago))
		e, err := store.Add(it.c, it.app)
		if err != nil {
			log.Printf("clipboard: seeding: %v", err)
			continue
		}
		if it.pinned {
			_ = store.SetPinned(e.ID, true)
		}
	}
	clock.at(time.Time{})
	if err := store.Flush(); err != nil {
		log.Printf("clipboard: seeding: %v", err)
	}
}

// demoImage draws a PNG for the demo history: a banner of soft color
// bands, or a round mark on a clear ground.
func demoImage(w, h int, round bool) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			fx, fy := float64(x)/float64(w), float64(y)/float64(h)
			if round {
				dx, dy := fx-0.5, fy-0.5
				d := math.Hypot(dx, dy)
				if d > 0.42 {
					continue // clear: the glass shows through
				}
				a := uint8(255)
				if d > 0.41 {
					a = uint8(255 * (0.42 - d) / 0.01)
				}
				img.SetNRGBA(x, y, color.NRGBA{uint8(40 + 120*fy), uint8(110 + 80*fx), 230, a})
				continue
			}
			band := 0.5 + 0.5*math.Sin(fx*7+fy*3)
			img.SetNRGBA(x, y, color.NRGBA{uint8(20 + 60*band), uint8(90 + 100*fy), uint8(200 + 40*band), 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
