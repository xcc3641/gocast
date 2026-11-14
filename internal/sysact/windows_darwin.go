//go:build darwin

package sysact

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// The frameworks and functions the actions call, loaded once.
var lib struct {
	once sync.Once
	ok   bool

	cf, appServices, coreGraphics, coreAudio uintptr

	// CoreFoundation
	cfString    func(alloc uintptr, s string, encoding uint32) uintptr
	cfRelease   func(ref uintptr)
	cfRetain    func(ref uintptr) uintptr
	cfEqual     func(a, b uintptr) bool
	cfHash      func(ref uintptr) uint64
	cfTypeID    func(ref uintptr) uint64
	cfBoolType  func() uint64
	cfBoolValue func(ref uintptr) bool
	cfArrayLen  func(ref uintptr) int
	cfArrayAt   func(ref uintptr, i int) uintptr
	cfTrue      uintptr
	cfFalse     uintptr

	// Accessibility
	trusted    func() bool
	appElement func(pid int32) uintptr
	copyAttr   func(el, attr uintptr, out *uintptr) int32
	setAttr    func(el, attr, value uintptr) int32
	setTimeout func(el uintptr, seconds float32) int32
	valueNew   func(typ int32, ptr unsafe.Pointer) uintptr
	valueGet   func(value uintptr, typ int32, ptr unsafe.Pointer) bool

	// CoreGraphics
	evSource func(state int32) uintptr
	keyEvent func(source uintptr, key uint16, down bool) uintptr
	setFlags func(event uintptr, flags uint64)
	post     func(tap uint32, event uintptr)
}

const cfUTF8 = 0x08000100 // kCFStringEncodingUTF8

func load() bool {
	lib.once.Do(func() {
		open := func(path string) uintptr {
			h, _ := purego.Dlopen(path, purego.RTLD_GLOBAL|purego.RTLD_NOW)
			return h
		}
		lib.cf = open("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation")
		lib.appServices = open("/System/Library/Frameworks/ApplicationServices.framework/ApplicationServices")
		lib.coreGraphics = open("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics")
		lib.coreAudio = open("/System/Library/Frameworks/CoreAudio.framework/CoreAudio")
		if lib.cf == 0 || lib.appServices == 0 || lib.coreGraphics == 0 {
			return
		}
		cf := lib.cf
		purego.RegisterLibFunc(&lib.cfString, cf, "CFStringCreateWithCString")
		purego.RegisterLibFunc(&lib.cfRelease, cf, "CFRelease")
		purego.RegisterLibFunc(&lib.cfRetain, cf, "CFRetain")
		purego.RegisterLibFunc(&lib.cfEqual, cf, "CFEqual")
		purego.RegisterLibFunc(&lib.cfHash, cf, "CFHash")
		purego.RegisterLibFunc(&lib.cfTypeID, cf, "CFGetTypeID")
		purego.RegisterLibFunc(&lib.cfBoolType, cf, "CFBooleanGetTypeID")
		purego.RegisterLibFunc(&lib.cfBoolValue, cf, "CFBooleanGetValue")
		purego.RegisterLibFunc(&lib.cfArrayLen, cf, "CFArrayGetCount")
		purego.RegisterLibFunc(&lib.cfArrayAt, cf, "CFArrayGetValueAtIndex")
		lib.cfTrue = constant(cf, "kCFBooleanTrue")
		lib.cfFalse = constant(cf, "kCFBooleanFalse")

		as := lib.appServices
		purego.RegisterLibFunc(&lib.trusted, as, "AXIsProcessTrusted")
		purego.RegisterLibFunc(&lib.appElement, as, "AXUIElementCreateApplication")
		purego.RegisterLibFunc(&lib.copyAttr, as, "AXUIElementCopyAttributeValue")
		purego.RegisterLibFunc(&lib.setAttr, as, "AXUIElementSetAttributeValue")
		purego.RegisterLibFunc(&lib.setTimeout, as, "AXUIElementSetMessagingTimeout")
		purego.RegisterLibFunc(&lib.valueNew, as, "AXValueCreate")
		purego.RegisterLibFunc(&lib.valueGet, as, "AXValueGetValue")

		cg := lib.coreGraphics
		purego.RegisterLibFunc(&lib.evSource, cg, "CGEventSourceCreate")
		purego.RegisterLibFunc(&lib.keyEvent, cg, "CGEventCreateKeyboardEvent")
		purego.RegisterLibFunc(&lib.setFlags, cg, "CGEventSetFlags")
		purego.RegisterLibFunc(&lib.post, cg, "CGEventPost")
		lib.ok = lib.cfTrue != 0 && lib.cfFalse != 0
	})
	return lib.ok
}

// constant is the value of the global constant (a pointer) name in the
// library.
func constant(h uintptr, name string) uintptr {
	addr, err := purego.Dlsym(h, name)
	if err != nil || addr == 0 {
		return 0
	}
	return **(**uintptr)(unsafe.Pointer(&addr))
}

// cfStrings are the attributes' names, made once and kept.
var cfStrings sync.Map

func cfstr(s string) uintptr {
	if v, ok := cfStrings.Load(s); ok {
		return v.(uintptr)
	}
	ref := lib.cfString(0, s, cfUTF8)
	if v, loaded := cfStrings.LoadOrStore(s, ref); loaded {
		lib.cfRelease(ref)
		return v.(uintptr)
	}
	return ref
}

// The Accessibility attributes and values.
const (
	axFocusedWindow = "AXFocusedWindow"
	axMainWindow    = "AXMainWindow"
	axWindowList    = "AXWindows"
	axPosition      = "AXPosition"
	axSize          = "AXSize"
	axMinimized     = "AXMinimized"
	axFullScreen    = "AXFullScreen"
	axSubrole       = "AXSubrole"
	axEnhancedUI    = "AXEnhancedUserInterface"

	axValuePoint = 1 // kAXValueCGPointType
	axValueSize  = 2 // kAXValueCGSizeType
	axSuccess    = 0

	// axTimeout is how long Gocast waits for an app to answer: a hung
	// app must not hang the panel.
	axTimeout = 1.0
)

type cgPoint struct{ X, Y float64 }
type cgSize struct{ W, H float64 }

// attr is the attribute name of el, retained (release it), or 0.
func attr(el uintptr, name string) uintptr {
	var out uintptr
	if lib.copyAttr(el, cfstr(name), &out) != axSuccess {
		return 0
	}
	return out
}

// boolAttr is the boolean attribute name of el, false when it has none.
func boolAttr(el uintptr, name string) bool {
	v := attr(el, name)
	if v == 0 {
		return false
	}
	defer lib.cfRelease(v)
	return lib.cfTypeID(v) == lib.cfBoolType() && lib.cfBoolValue(v)
}

func setBool(el uintptr, name string, on bool) {
	v := lib.cfFalse
	if on {
		v = lib.cfTrue
	}
	lib.setAttr(el, cfstr(name), v)
}

// axWindows is the Accessibility API's windows: NewWindows.
type axWindows struct{ screens func() []Screen }

// NewWindows returns the system's windows, through the Accessibility
// API; screens tells the displays (MyGo's, in the app).
func NewWindows(screens func() []Screen) Windows { return axWindows{screens} }

func (w axWindows) Trusted() bool { return load() && lib.trusted() }

func (w axWindows) Screens() []Screen { return w.screens() }

func (w axWindows) Focused(pid int) (Window, error) {
	if !load() {
		return nil, ErrUnsupported
	}
	app := lib.appElement(int32(pid))
	if app == 0 {
		return nil, ErrNoWindow
	}
	defer lib.cfRelease(app)
	lib.setTimeout(app, axTimeout)
	win := attr(app, axFocusedWindow)
	if win == 0 {
		win = attr(app, axMainWindow)
	}
	if win == 0 {
		win = firstStandard(app)
	}
	if win == 0 {
		return nil, ErrNoWindow
	}
	switch {
	case boolAttr(win, axMinimized):
		lib.cfRelease(win)
		return nil, ErrNoWindow
	case boolAttr(win, axFullScreen):
		lib.cfRelease(win)
		return nil, ErrFullScreen
	}
	return &axWindow{pid: pid, el: win}, nil
}

// firstStandard is app's first standard window, retained, or 0.
func firstStandard(app uintptr) uintptr {
	list := attr(app, axWindowList)
	if list == 0 {
		return 0
	}
	defer lib.cfRelease(list)
	for i := range lib.cfArrayLen(list) {
		el := lib.cfArrayAt(list, i)
		role := attr(el, axSubrole)
		if role == 0 {
			continue
		}
		standard := lib.cfEqual(role, cfstr("AXStandardWindow"))
		lib.cfRelease(role)
		if standard {
			return lib.cfRetain(el) // the list's: retained for the caller, as AXFocusedWindow's
		}
	}
	return 0
}

// axWindow is a window of another app.
type axWindow struct {
	pid int
	el  uintptr
}

func (w *axWindow) Key() string { return fmt.Sprintf("%d:%x", w.pid, lib.cfHash(w.el)) }

func (w *axWindow) Frame() (Rect, error) {
	var p cgPoint
	var s cgSize
	pv := attr(w.el, axPosition)
	if pv == 0 {
		return Rect{}, ErrNoWindow
	}
	defer lib.cfRelease(pv)
	sv := attr(w.el, axSize)
	if sv == 0 {
		return Rect{}, ErrNoWindow
	}
	defer lib.cfRelease(sv)
	if !lib.valueGet(pv, axValuePoint, unsafe.Pointer(&p)) || !lib.valueGet(sv, axValueSize, unsafe.Pointer(&s)) {
		return Rect{}, ErrNoWindow
	}
	return Rect{p.X, p.Y, s.W, s.H}, nil
}

func (w *axWindow) SetFrame(r Rect) error {
	// An app with the enhanced interface (turned on by assistive apps)
	// animates each change, which leaves the window between them: off
	// while it moves (Rectangle does the same).
	app := lib.appElement(int32(w.pid))
	if app != 0 {
		defer lib.cfRelease(app)
		if boolAttr(app, axEnhancedUI) {
			setBool(app, axEnhancedUI, false)
			defer setBool(app, axEnhancedUI, true)
		}
	}
	p := cgPoint{r.X, r.Y}
	s := cgSize{r.W, r.H}
	pv := lib.valueNew(axValuePoint, unsafe.Pointer(&p))
	sv := lib.valueNew(axValueSize, unsafe.Pointer(&s))
	if pv == 0 || sv == 0 {
		return fmt.Errorf("sysact: cannot make the frame's values")
	}
	defer lib.cfRelease(pv)
	defer lib.cfRelease(sv)
	// Its size, where it is; where it goes; its size again, which may not
	// have fitted where it was (onto another display).
	lib.setAttr(w.el, cfstr(axSize), sv)
	if e := lib.setAttr(w.el, cfstr(axPosition), pv); e != axSuccess {
		return fmt.Errorf("sysact: moving the window: AXError %d", e)
	}
	lib.setAttr(w.el, cfstr(axSize), sv)
	return nil
}

func (w *axWindow) Release() {
	if w.el != 0 {
		lib.cfRelease(w.el)
		w.el = 0
	}
}
