//go:build darwin

package sysfmt

import (
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

var (
	selNew        = objc.RegisterName("new")
	selDrain      = objc.RegisterName("drain")
	selString     = objc.RegisterName("stringWithUTF8String:")
	selUTF8       = objc.RegisterName("UTF8String")
	selCurrent    = objc.RegisterName("currentLocale")
	selLocaleID   = objc.RegisterName("localeWithLocaleIdentifier:")
	selLangCode   = objc.RegisterName("languageCode")
	selSetLocale  = objc.RegisterName("setLocale:")
	selSetDate    = objc.RegisterName("setDateStyle:")
	selSetTime    = objc.RegisterName("setTimeStyle:")
	selRelative   = objc.RegisterName("setDoesRelativeDateFormatting:")
	selTemplate   = objc.RegisterName("setLocalizedDateFormatFromTemplate:")
	selFromDate   = objc.RegisterName("stringFromDate:")
	selDateSince  = objc.RegisterName("dateWithTimeIntervalSince1970:")
	selByteString = objc.RegisterName("stringFromByteCount:countStyle:")
	selRelease    = objc.RegisterName("release")
	selGroupSep   = objc.RegisterName("groupingSeparator")
	selDecimalSep = objc.RegisterName("decimalSeparator")
)

var (
	load                                        sync.Once
	classPool, classStr, classLocale, classDate objc.ID
	classDateFmt, classByteFmt                  objc.ID
)

func classes() bool {
	load.Do(func() {
		_, _ = purego.Dlopen("/System/Library/Frameworks/Foundation.framework/Foundation", purego.RTLD_GLOBAL|purego.RTLD_NOW)
		classPool = objc.ID(objc.GetClass("NSAutoreleasePool"))
		classStr = objc.ID(objc.GetClass("NSString"))
		classLocale = objc.ID(objc.GetClass("NSLocale"))
		classDate = objc.ID(objc.GetClass("NSDate"))
		classDateFmt = objc.ID(objc.GetClass("NSDateFormatter"))
		classByteFmt = objc.ID(objc.GetClass("NSByteCountFormatter"))
	})
	return classPool != 0 && classStr != 0 && classLocale != 0 && classDate != 0 && classDateFmt != 0 && classByteFmt != 0
}

func goString(s objc.ID) string {
	if s == 0 {
		return ""
	}
	return objc.Send[string](s, selUTF8)
}

// nsLocale is the system's locale when it speaks the language of locale
// (a BCP 47 tag, the app's language), which keeps the user's 12- or
// 24-hour clock; else that language's.
func nsLocale(locale string) objc.ID {
	cur := classLocale.Send(selCurrent)
	lang, _, _ := strings.Cut(locale, "-")
	if locale == "" || strings.EqualFold(goString(cur.Send(selLangCode)), lang) {
		return cur
	}
	return classLocale.Send(selLocaleID, classStr.Send(selString, locale))
}

// NSDateFormatterStyle.
const (
	styleNone   = 0
	styleShort  = 1
	styleMedium = 2
)

// Date says when t was, as of now, in the language of locale: today and
// yesterday in words with the time ("Today at 14:05"), else the date
// without its year (with it before this year) and the time.
func Date(t, now time.Time, locale string) string {
	if !classes() {
		return fallbackDate(t, now)
	}
	var s string
	// An autorelease pool is the thread's: keep to it till it drains.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := classPool.Send(selNew)
	defer pool.Send(selDrain)
	f := classDateFmt.Send(selNew)
	defer f.Send(selRelease)
	f.Send(selSetLocale, nsLocale(locale))
	if relativeDay(t, now) {
		f.Send(selRelative, true)
		f.Send(selSetDate, styleMedium)
		f.Send(selSetTime, styleShort)
	} else {
		tmpl := "MMMdjmm"
		if t.In(now.Location()).Year() != now.Year() {
			tmpl = "yMMMdjmm"
		}
		f.Send(selTemplate, classStr.Send(selString, tmpl))
	}
	date := classDate.Send(selDateSince, float64(t.UnixNano())/1e9)
	s = goString(f.Send(selFromDate, date))
	if s == "" {
		return fallbackDate(t, now)
	}
	return s
}

// Bytes is a size as Finder writes a file's: "84 KB", "5.3 MB".
func Bytes(n int64) string {
	if !classes() {
		return fallbackBytes(n)
	}
	// An autorelease pool is the thread's: keep to it till it drains.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := classPool.Send(selNew)
	defer pool.Send(selDrain)
	const countStyleFile = 0
	return goString(classByteFmt.Send(selByteString, n, countStyleFile))
}

// Day says the day of t, as of now, in the language of locale, without
// the time: today and yesterday in words ("今天", "Yesterday"), else the
// date without its year (with it before this year): "11月10日", "Nov 10";
// long spells the month ("November 10"), for VoiceOver.
func Day(t, now time.Time, locale string, long bool) string {
	if !classes() {
		return fallbackDay(t, now)
	}
	// An autorelease pool is the thread's: keep to it till it drains.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := classPool.Send(selNew)
	defer pool.Send(selDrain)
	f := classDateFmt.Send(selNew)
	defer f.Send(selRelease)
	f.Send(selSetLocale, nsLocale(locale))
	if relativeDay(t, now) {
		f.Send(selRelative, true)
		f.Send(selSetDate, styleMedium)
		f.Send(selSetTime, styleNone)
	} else {
		tmpl := "MMMd"
		if long {
			tmpl = "MMMMd"
		}
		if t.In(now.Location()).Year() != now.Year() {
			tmpl = "y" + tmpl
		}
		f.Send(selTemplate, classStr.Send(selString, tmpl))
	}
	date := classDate.Send(selDateSince, float64(t.UnixNano())/1e9)
	if s := goString(f.Send(selFromDate, date)); s != "" {
		return s
	}
	return fallbackDay(t, now)
}

// Separators are the grouping separator and the decimal point of the
// system's number format (System Settings › General › Language & Region):
// "," and "." in the US, "." and "," in Germany, a space and "," in
// France.
func Separators() (group, decimal string) {
	if !classes() {
		return ",", "."
	}
	// An autorelease pool is the thread's: keep to it till it drains.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := classPool.Send(selNew)
	defer pool.Send(selDrain)
	cur := classLocale.Send(selCurrent)
	group = goString(cur.Send(selGroupSep))
	decimal = goString(cur.Send(selDecimalSep))
	if decimal == "" {
		return ",", "."
	}
	return group, decimal
}
