// Package apps lists the installed apps with every name they are known
// by: the name the system shows in the user's language (MyGo's
// Shell.InstalledApps), and the app's English and Chinese names from its
// bundle, so that "WeChat", "微信", "weixin" and "wx" all find WeChat
// whatever language the system is in.
package apps

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"howett.net/plist"
)

// nameLangs are the localizations whose names an app is found by, besides
// the system's language: English and Chinese, as Gocast's users
// type them.
var nameLangs = []string{"en", "English", "Base", "en_US", "en_GB", "zh-Hans", "zh_CN", "zh-CN", "zh_SG", "zh-Hant", "zh_TW", "zh_HK", "zh-HK"}

var (
	mu    sync.Mutex
	cache = map[string]cached{}
)

type cached struct {
	stamp int64
	info  Info
}

// Info is what an app is called.
type Info struct {
	// Names are every name it is found by, without repeats: its file
	// name, its Info.plist's, then its English and Chinese localizations'.
	Names []string
	// ByLang is the name of each localization that has one, by the
	// localization's folder name (en, zh-Hans, zh_CN…).
	ByLang map[string]string
}

// localeLangs are the localizations, best first, that name an app for the
// language the app shows.
var localeLangs = map[string][]string{
	"zh-Hans": {"zh-Hans", "zh_CN", "zh-CN", "zh_SG"},
	"zh-Hant": {"zh-Hant", "zh_TW", "zh_HK", "zh-HK"},
	"en":      {"en", "English", "en_US", "Base"},
}

// Localized returns the app's name in the language locale (a BCP 47 tag,
// as mygo.App.Locale returns, "zh-Hans" or "en"), or "" when its bundle
// has none: the caller keeps the system's name then.
//
// The system's own name (NSFileManager's displayNameAtPath, which MyGo's
// InstalledApps returns) follows the languages of the process asking,
// which are the languages its bundle has: English for a program run with
// go run, whatever the system's language.
func (i Info) Localized(locale string) string {
	key := locale
	switch {
	case strings.HasPrefix(locale, "zh-Hant"), locale == "zh-TW", locale == "zh-HK", locale == "zh-MO":
		key = "zh-Hant"
	case strings.HasPrefix(locale, "zh"):
		key = "zh-Hans"
	case strings.HasPrefix(locale, "en"):
		key = "en"
	}
	for _, lang := range localeLangs[key] {
		if n := i.ByLang[lang]; n != "" {
			return n
		}
	}
	return ""
}

// Names returns the other names of the .app bundle at path: its file name,
// its Info.plist's CFBundleDisplayName and CFBundleName, and those of its
// English and Chinese localizations (InfoPlist.loctable, as Apple's apps
// have, or <lang>.lproj/InfoPlist.strings). It reads the disk once per
// version of the bundle, and is safe from any goroutine.
func Names(path string) Info {
	var stamp int64
	if fi, err := os.Stat(filepath.Join(path, "Contents", "Info.plist")); err == nil {
		stamp = fi.ModTime().UnixNano()
	}
	mu.Lock()
	c, ok := cache[path]
	mu.Unlock()
	if ok && c.stamp == stamp {
		return c.info
	}
	info := read(path)
	mu.Lock()
	cache[path] = cached{stamp, info}
	mu.Unlock()
	return info
}

func clean(n string) string {
	n = strings.TrimSpace(strings.TrimSuffix(n, ".app"))
	if strings.Contains(n, "$(") {
		return ""
	}
	return n
}

func read(path string) Info {
	info := Info{ByLang: map[string]string{}}
	add := func(n string) {
		if n = clean(n); n != "" && !slices.Contains(info.Names, n) {
			info.Names = append(info.Names, n)
		}
	}
	addLang := func(lang string, d map[string]any) {
		display, name := clean(str(d["CFBundleDisplayName"])), clean(str(d["CFBundleName"]))
		if info.ByLang[lang] == "" {
			info.ByLang[lang] = cmpOr(display, name)
		}
		add(display)
		add(name)
	}
	add(filepath.Base(path))
	contents := filepath.Join(path, "Contents")
	if info := decode(filepath.Join(contents, "Info.plist")); info != nil {
		add(str(info["CFBundleDisplayName"]))
		add(str(info["CFBundleName"]))
	}
	res := filepath.Join(contents, "Resources")
	// Apple's apps keep every language's strings in one table.
	if table := decode(filepath.Join(res, "InfoPlist.loctable")); table != nil {
		for _, lang := range nameLangs {
			if d, ok := table[lang].(map[string]any); ok {
				addLang(lang, d)
			}
		}
	}
	for _, lang := range nameLangs {
		if d := decode(filepath.Join(res, lang+".lproj", "InfoPlist.strings")); d != nil {
			addLang(lang, d)
		}
	}
	return info
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// decode reads a property list or a .strings file (in any of the formats
// and encodings they come in) as a dictionary, or nil.
func decode(file string) map[string]any {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var d map[string]any
	if _, err := plist.Unmarshal(data, &d); err != nil {
		return nil
	}
	return d
}

func str(v any) string {
	s, _ := v.(string)
	return s
}
