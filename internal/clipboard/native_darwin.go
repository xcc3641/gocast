//go:build darwin

package clipboard

import (
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// The frameworks this package calls into. MyGo has loaded AppKit in the
// app; tests load them here.
var (
	loadOnce sync.Once
	libs     struct {
		cf, foundation, appServices, coreGraphics uintptr
	}
)

func load() {
	loadOnce.Do(func() {
		open := func(path string) uintptr {
			h, _ := purego.Dlopen(path, purego.RTLD_GLOBAL|purego.RTLD_NOW)
			return h
		}
		open("/System/Library/Frameworks/AppKit.framework/AppKit")
		libs.cf = open("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation")
		libs.foundation = open("/System/Library/Frameworks/Foundation.framework/Foundation")
		libs.appServices = open("/System/Library/Frameworks/ApplicationServices.framework/ApplicationServices")
		libs.coreGraphics = open("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics")
	})
}

var (
	selNew         = objc.RegisterName("new")
	selDrain       = objc.RegisterName("drain")
	selUTF8        = objc.RegisterName("UTF8String")
	selString      = objc.RegisterName("stringWithUTF8String:")
	selGeneral     = objc.RegisterName("generalPasteboard")
	selChangeCount = objc.RegisterName("changeCount")
	selShared      = objc.RegisterName("sharedWorkspace")
	selFrontmost   = objc.RegisterName("frontmostApplication")
	selBundleID    = objc.RegisterName("bundleIdentifier")
	selName        = objc.RegisterName("localizedName")
	selPID         = objc.RegisterName("processIdentifier")
	selWithPID     = objc.RegisterName("runningApplicationWithProcessIdentifier:")
	selWithBundle  = objc.RegisterName("runningApplicationsWithBundleIdentifier:")
	selCount       = objc.RegisterName("count")
	selAt          = objc.RegisterName("objectAtIndex:")
	selTerminated  = objc.RegisterName("isTerminated")
	selActivate    = objc.RegisterName("activateWithOptions:")
)

func class(name string) objc.ID {
	load()
	return objc.ID(objc.GetClass(name))
}

// withPool runs fn on one OS thread inside an autorelease pool, which
// frees what the calls autorelease.
func withPool(fn func()) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := class("NSAutoreleasePool").Send(selNew)
	defer pool.Send(selDrain)
	fn()
}

func goString(s objc.ID) string {
	if s == 0 {
		return ""
	}
	return objc.Send[string](s, selUTF8)
}

// changeCount is NSPasteboard.generalPasteboard.changeCount. Reading it is
// safe off the main thread and reads no contents.
func changeCount() int {
	n := -1
	withPool(func() {
		if pb := class("NSPasteboard").Send(selGeneral); pb != 0 {
			n = objc.Send[int](pb, selChangeCount)
		}
	})
	return n
}

// FrontmostApp is the app that has the user's attention now
// (NSWorkspace.frontmostApplication), or the zero App.
func FrontmostApp() App {
	var a App
	withPool(func() {
		ws := class("NSWorkspace").Send(selShared)
		if ws == 0 {
			return
		}
		app := ws.Send(selFrontmost)
		if app == 0 {
			return
		}
		a = App{
			BundleID: goString(app.Send(selBundleID)),
			Name:     goString(app.Send(selName)),
			PID:      int(objc.Send[int32](app, selPID)),
		}
	})
	return a
}

// NSApplicationActivateIgnoringOtherApps; macOS 14 and later ignore it and
// activate cooperatively, which works while Gocast is the active app.
const activateIgnoringOtherApps = 1 << 1

// ActivateApp brings the running app with process ID pid to the front:
// the app to paste into, which the caller noted before showing its panel.
func ActivateApp(pid int) bool {
	if pid <= 0 {
		return false
	}
	ok := false
	withPool(func() {
		app := class("NSRunningApplication").Send(selWithPID, int32(pid))
		if app != 0 && !objc.Send[bool](app, selTerminated) {
			ok = objc.Send[bool](app, selActivate, uint(activateIgnoringOtherApps))
		}
	})
	return ok
}

// ActivateBundle brings the first running instance of the app with
// bundleID to the front.
func ActivateBundle(bundleID string) bool {
	if bundleID == "" {
		return false
	}
	ok := false
	withPool(func() {
		apps := class("NSRunningApplication").Send(selWithBundle, class("NSString").Send(selString, bundleID))
		n := objc.Send[uint](apps, selCount)
		for i := uint(0); i < n; i++ {
			app := apps.Send(selAt, i)
			if !objc.Send[bool](app, selTerminated) {
				ok = objc.Send[bool](app, selActivate, uint(activateIgnoringOtherApps))
				return
			}
		}
	})
	return ok
}

// WaitFrontmost waits up to timeout for the app with process ID pid to be
// frontmost, as activation takes a moment, and reports whether it is.
// Call it on a goroutine, not the main thread: AppKit updates the
// frontmost app from the main run loop.
func WaitFrontmost(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if FrontmostApp().PID == pid {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// deref reads the pointer-sized global at addr, such as a CFStringRef
// constant found with Dlsym.
func deref(addr uintptr) uintptr {
	if addr == 0 {
		return 0
	}
	return **(**uintptr)(unsafe.Pointer(&addr))
}

// symbol is the value of the global constant name in lib, or 0.
func symbol(lib uintptr, name string) uintptr {
	if lib == 0 {
		return 0
	}
	addr, err := purego.Dlsym(lib, name)
	if err != nil {
		return 0
	}
	return deref(addr)
}

// CoreFoundation, for the dictionary the Accessibility check takes.
var cf struct {
	once        sync.Once
	ok          bool
	dictCreate  func(alloc uintptr, keys, values *uintptr, n int, keyCB, valueCB uintptr) uintptr
	release     func(ref uintptr)
	keyCB       uintptr
	valueCB     uintptr
	booleanTrue uintptr
}

func loadCF() bool {
	cf.once.Do(func() {
		load()
		if libs.cf == 0 {
			return
		}
		purego.RegisterLibFunc(&cf.dictCreate, libs.cf, "CFDictionaryCreate")
		purego.RegisterLibFunc(&cf.release, libs.cf, "CFRelease")
		cf.keyCB, _ = purego.Dlsym(libs.cf, "kCFTypeDictionaryKeyCallBacks")
		cf.valueCB, _ = purego.Dlsym(libs.cf, "kCFTypeDictionaryValueCallBacks")
		cf.booleanTrue = symbol(libs.cf, "kCFBooleanTrue")
		cf.ok = cf.keyCB != 0 && cf.valueCB != 0 && cf.booleanTrue != 0
	})
	return cf.ok
}

// cfRefs releases the CoreFoundation objects made for a call.
type cfRefs []uintptr

func (r *cfRefs) keep(ref uintptr) uintptr {
	if ref != 0 {
		*r = append(*r, ref)
	}
	return ref
}

// dict makes a CFDictionary of keys and values, in pairs.
func (r *cfRefs) dict(pairs ...uintptr) uintptr {
	n := len(pairs) / 2
	keys, values := make([]uintptr, n), make([]uintptr, n)
	for i := 0; i < n; i++ {
		keys[i], values[i] = pairs[2*i], pairs[2*i+1]
	}
	return r.keep(cf.dictCreate(0, &keys[0], &values[0], n, cf.keyCB, cf.valueCB))
}

func (r *cfRefs) release() {
	for _, ref := range *r {
		cf.release(ref)
	}
	*r = nil
}
