// Package running tells whether an app is running, asks it to quit, hides
// it and opens it, by its bundle identifier, through NSRunningApplication
// and NSWorkspace, which need no Accessibility permission.
package running

import (
	"errors"
	"os/exec"
	"runtime"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

var (
	selString = objc.RegisterName("stringWithUTF8String:")
	selApps   = objc.RegisterName("runningApplicationsWithBundleIdentifier:")
	selCount  = objc.RegisterName("count")
	selAt     = objc.RegisterName("objectAtIndex:")
	selTerm   = objc.RegisterName("terminate")
	selPool   = objc.RegisterName("new")
	selDrain  = objc.RegisterName("drain")
	selTermed = objc.RegisterName("isTerminated")
	selShared = objc.RegisterName("sharedWorkspace")
	selURLFor = objc.RegisterName("URLForApplicationWithBundleIdentifier:")
	selPath   = objc.RegisterName("path")
	selUTF8   = objc.RegisterName("UTF8String")
	selFront  = objc.RegisterName("frontmostApplication")
	selBundle = objc.RegisterName("bundleIdentifier")
	selHidden = objc.RegisterName("isHidden")
	selHide   = objc.RegisterName("hide")
	selConfig = objc.RegisterName("configuration")
	selActs   = objc.RegisterName("setActivates:")
	selOpen   = objc.RegisterName("openApplicationAtURL:configuration:completionHandler:")
)

var (
	load                          sync.Once
	classStr, classApp, classPool objc.ID
	classWorkspace, classConfig   objc.ID
)

// classes looks the classes up once AppKit is loaded, which MyGo has done
// in the app, and which a test loads here.
func classes() bool {
	load.Do(func() {
		_, _ = purego.Dlopen("/System/Library/Frameworks/AppKit.framework/AppKit", purego.RTLD_GLOBAL|purego.RTLD_NOW)
		classStr = objc.ID(objc.GetClass("NSString"))
		classApp = objc.ID(objc.GetClass("NSRunningApplication"))
		classPool = objc.ID(objc.GetClass("NSAutoreleasePool"))
		classWorkspace = objc.ID(objc.GetClass("NSWorkspace"))
		classConfig = objc.ID(objc.GetClass("NSWorkspaceOpenConfiguration"))
	})
	return classStr != 0 && classApp != 0 && classPool != 0
}

// each calls fn with each running instance of the app with bundleID.
func each(bundleID string, fn func(app objc.ID)) {
	if bundleID == "" || !classes() {
		return
	}
	// An autorelease pool is the thread's: keep to it till it drains.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := classPool.Send(selPool)
	defer pool.Send(selDrain)
	str := classStr.Send(selString, bundleID)
	apps := classApp.Send(selApps, str)
	n := objc.Send[uint](apps, selCount)
	for i := uint(0); i < n; i++ {
		app := apps.Send(selAt, i)
		if !objc.Send[bool](app, selTermed) {
			fn(app)
		}
	}
}

// IsRunning reports whether the app with bundleID runs.
func IsRunning(bundleID string) bool {
	found := false
	each(bundleID, func(objc.ID) { found = true })
	return found
}

// Quit asks every instance of the app with bundleID to quit, as its Quit
// menu item would: it may ask to save first.
func Quit(bundleID string) bool {
	asked := false
	each(bundleID, func(app objc.ID) { asked = objc.Send[bool](app, selTerm) || asked })
	return asked
}

// AppPath is where the app with bundleID is installed, as Launch Services
// finds it (NSWorkspace's URLForApplicationWithBundleIdentifier:), or ""
// when it is not.
func AppPath(bundleID string) string {
	if bundleID == "" || !classes() || classWorkspace == 0 {
		return ""
	}
	// An autorelease pool is the thread's: keep to it till it drains.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := classPool.Send(selPool)
	defer pool.Send(selDrain)
	url := classWorkspace.Send(selShared).Send(selURLFor, classStr.Send(selString, bundleID))
	if url == 0 {
		return ""
	}
	path := url.Send(selPath)
	if path == 0 {
		return ""
	}
	return objc.Send[string](path, selUTF8)
}

// ErrNotInstalled is Open's error for an app Launch Services can't find.
var ErrNotInstalled = errors.New("app not installed")

// Frontmost is the bundle ID of the frontmost app, the one with the menu
// bar, and whether it is hidden; "" when there is none or it has no
// bundle ID.
func Frontmost() (bundleID string, hidden bool) {
	if !classes() || classWorkspace == 0 {
		return "", false
	}
	// An autorelease pool is the thread's: keep to it till it drains.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := classPool.Send(selPool)
	defer pool.Send(selDrain)
	app := classWorkspace.Send(selShared).Send(selFront)
	if app == 0 {
		return "", false
	}
	if id := app.Send(selBundle); id != 0 {
		bundleID = objc.Send[string](id, selUTF8)
	}
	return bundleID, objc.Send[bool](app, selHidden)
}

// Hide hides every instance of the app with bundleID, as its Hide menu
// item (Command-H) would; the next app comes forward.
func Hide(bundleID string) bool {
	hid := false
	each(bundleID, func(app objc.ID) { hid = objc.Send[bool](app, selHide) || hid })
	return hid
}

// Open opens the app with bundleID as a click on its Dock icon does: it
// launches it, or brings it forward, unhidden, and the app opens a window
// when it has none (Launch Services sends it the reopen event). This is
// NSWorkspace's openApplicationAtURL:configuration:completionHandler:, as
// macOS 14 may ignore an activation that an app in the background asks
// for itself (NSRunningApplication's activateWithOptions:), but not one
// through Launch Services. It returns before the app has opened: errors
// past finding the app are not known.
func Open(bundleID string) error {
	if bundleID == "" {
		return ErrNotInstalled
	}
	if !classes() || classWorkspace == 0 || classConfig == 0 {
		return openTool(bundleID)
	}
	// An autorelease pool is the thread's: keep to it till it drains.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := classPool.Send(selPool)
	defer pool.Send(selDrain)
	ws := classWorkspace.Send(selShared)
	url := ws.Send(selURLFor, classStr.Send(selString, bundleID))
	if url == 0 {
		return ErrNotInstalled
	}
	cfg := classConfig.Send(selConfig) // autoreleased
	cfg.Send(selActs, true)
	// No completion handler (nil, which the API takes): there is nothing
	// to do once the app opened, and a block is awkward from Go.
	ws.Send(selOpen, url, cfg, objc.ID(0))
	return nil
}

// openTool opens the app with /usr/bin/open -b, which does what Open does,
// where AppKit's classes can't be had.
func openTool(bundleID string) error {
	cmd := exec.Command("/usr/bin/open", "-b", bundleID)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
