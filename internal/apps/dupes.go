package apps

import (
	"os"
	"path/filepath"
	"strings"
)

// App is an installed app as Unique and Places see it: its bundle
// identifier (its path if it has none), its .app bundle and the name it
// shows.
type App struct {
	ID, Path, Name string
}

// Unique returns the indexes of apps without the other copies of an app,
// which are the same app: the same bundle identifier. It keeps the copy in
// /Applications or /System/Applications (or their folders), then the one
// in ~/Applications, then any other, the first of equals; in apps' order.
// Apps of the same name but different identifiers are different apps,
// kept both (Places tells them apart).
func Unique(apps []App) []int {
	best := map[string]int{} // by ID, the index kept
	for i, a := range apps {
		j, ok := best[a.ID]
		if !ok || rank(a.Path) < rank(apps[j].Path) {
			best[a.ID] = i
		}
	}
	var keep []int
	for i, a := range apps {
		if best[a.ID] == i {
			keep = append(keep, i)
		}
	}
	return keep
}

// rank is how much a copy of an app at path is the one to keep: less is
// better.
func rank(path string) int {
	switch {
	case under(path, "/Applications"), under(path, "/System/Applications"):
		return 0
	case under(path, filepath.Join(home(), "Applications")):
		return 1
	}
	return 2
}

func under(path, dir string) bool {
	return dir != "" && strings.HasPrefix(path, dir+"/")
}

func home() string {
	h, _ := os.UserHomeDir()
	return h
}

// Places returns, for each app whose name another app has too (whatever
// the case), what tells it apart, to show after its name: its bundle's
// file name without .app (Siri AI.app: "Siri AI"), nothing when that is
// its name (Siri.app, named Siri); its folder only when another of those
// apps has its file name too (~/Applications, /System/Applications,
// Chrome Apps without its .localized), its whole path when they also
// share the folder. "" for the others.
func Places(apps []App) []string {
	byName := map[string][]int{}
	for i, a := range apps {
		k := strings.ToLower(a.Name)
		byName[k] = append(byName[k], i)
	}
	out := make([]string, len(apps))
	for _, same := range byName {
		if len(same) < 2 {
			continue
		}
		byFile := map[string][]int{}
		for _, i := range same {
			k := strings.ToLower(fileName(apps[i].Path))
			byFile[k] = append(byFile[k], i)
		}
		for _, files := range byFile {
			if len(files) == 1 {
				i := files[0]
				if f := fileName(apps[i].Path); !strings.EqualFold(f, apps[i].Name) {
					out[i] = f
				}
				continue
			}
			dirs := map[string]int{}
			for _, i := range files {
				dirs[filepath.Dir(apps[i].Path)]++
			}
			for _, i := range files {
				p := apps[i].Path
				if dirs[filepath.Dir(p)] == 1 {
					p = filepath.Dir(p)
				}
				out[i] = short(p)
			}
		}
	}
	return out
}

// fileName is the file name of the bundle at path, without .app.
func fileName(path string) string {
	return strings.TrimSuffix(filepath.Base(path), ".app")
}

// short is a path as Finder names it: ~ for the home folder, folders
// without their .localized.
func short(p string) string {
	if h := home(); h != "" {
		if rest, ok := strings.CutPrefix(p, h); ok && (rest == "" || rest[0] == '/') {
			p = "~" + rest
		}
	}
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = strings.TrimSuffix(s, ".localized")
	}
	return strings.Join(parts, "/")
}
