// Package running tells whether an app is running and asks it to quit,
// by its bundle identifier, through NSRunningApplication.
package running

import (
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
)

var (
	load                          sync.Once
	classStr, classApp, classPool objc.ID
	classWorkspace                objc.ID
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
	})
	return classStr != 0 && classApp != 0 && classPool != 0
}

// each calls fn with each running instance of the app with bundleID.
func each(bundleID string, fn func(app objc.ID)) {
	if bundleID == "" || !classes() {
		return
	}
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
