//go:build darwin

package keepawake

import (
	"fmt"
	"sync"

	"github.com/ebitengine/purego"
)

// System returns macOS's power assertions (IOKit's IOPMAssertion…).
func System() Backend { return iokit{} }

type iokit struct{}

var lib struct {
	once sync.Once
	ok   bool

	cfString  func(alloc uintptr, s string, encoding uint32) uintptr
	cfRelease func(ref uintptr)
	// IOPMAssertionCreateWithName(type, level, name, &id) and
	// IOPMAssertionRelease(id); both return an IOReturn.
	create  func(typ uintptr, level uint32, name uintptr, id *uint32) int32
	release func(id uint32) int32
}

const (
	cfUTF8        = 0x08000100 // kCFStringEncodingUTF8
	assertLevelOn = 255        // kIOPMAssertionLevelOn
)

func load() bool {
	lib.once.Do(func() {
		cf, _ := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_GLOBAL|purego.RTLD_NOW)
		iok, _ := purego.Dlopen("/System/Library/Frameworks/IOKit.framework/IOKit", purego.RTLD_GLOBAL|purego.RTLD_NOW)
		if cf == 0 || iok == 0 {
			return
		}
		purego.RegisterLibFunc(&lib.cfString, cf, "CFStringCreateWithCString")
		purego.RegisterLibFunc(&lib.cfRelease, cf, "CFRelease")
		purego.RegisterLibFunc(&lib.create, iok, "IOPMAssertionCreateWithName")
		purego.RegisterLibFunc(&lib.release, iok, "IOPMAssertionRelease")
		lib.ok = true
	})
	return lib.ok
}

func (iokit) Create(t AssertionType, name string) (Assertion, error) {
	if !load() {
		return 0, ErrUnsupported
	}
	typ := lib.cfString(0, string(t), cfUTF8)
	if typ == 0 {
		return 0, fmt.Errorf("keepawake: no CFString for %q", t)
	}
	defer lib.cfRelease(typ)
	n := lib.cfString(0, name, cfUTF8)
	if n == 0 {
		return 0, fmt.Errorf("keepawake: no CFString for %q", name)
	}
	defer lib.cfRelease(n)
	var id uint32
	if r := lib.create(typ, assertLevelOn, n, &id); r != 0 {
		return 0, fmt.Errorf("keepawake: IOPMAssertionCreateWithName(%s): 0x%x", t, uint32(r))
	}
	return Assertion(id), nil
}

func (iokit) Release(a Assertion) error {
	if !load() {
		return ErrUnsupported
	}
	if r := lib.release(uint32(a)); r != 0 {
		return fmt.Errorf("keepawake: IOPMAssertionRelease(%d): 0x%x", a, uint32(r))
	}
	return nil
}
