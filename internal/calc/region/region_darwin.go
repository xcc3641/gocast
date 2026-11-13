//go:build darwin

package region

import (
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

var (
	selNew      = objc.RegisterName("new")
	selDrain    = objc.RegisterName("drain")
	selUTF8     = objc.RegisterName("UTF8String")
	selCurrent  = objc.RegisterName("currentLocale")
	selCountry  = objc.RegisterName("countryCode")
	load        sync.Once
	classPool   objc.ID
	classLocale objc.ID
)

func classes() bool {
	load.Do(func() {
		_, _ = purego.Dlopen("/System/Library/Frameworks/Foundation.framework/Foundation", purego.RTLD_GLOBAL|purego.RTLD_NOW)
		classPool = objc.ID(objc.GetClass("NSAutoreleasePool"))
		classLocale = objc.ID(objc.GetClass("NSLocale"))
	})
	return classPool != 0 && classLocale != 0
}

// Code is the system's region code ("JP", "CN"; NSLocale.currentLocale's
// countryCode), else the POSIX locale's; "" when unknown.
func Code() string {
	if classes() {
		pool := classPool.Send(selNew)
		defer pool.Send(selDrain)
		if c := classLocale.Send(selCurrent).Send(selCountry); c != 0 {
			if s := objc.Send[string](c, selUTF8); len(s) == 2 && isUpperASCII(s) {
				return s
			}
		}
	}
	return fromEnv()
}
