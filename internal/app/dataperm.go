package app

import (
	"log"
	"os"
	"path/filepath"
)

// restrictDataDir makes the data folder and the files Gocast keeps at its
// top level readable by the user only (0700 and 0600), as the clipboard
// history already is: earlier builds wrote settings, presets and usage
// with 0644 and the folder with 0755.
func restrictDataDir(dir string) {
	if err := os.Chmod(dir, 0o700); err != nil {
		log.Printf("data folder: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("data folder: %v", err)
		return
	}
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		info, err := os.Lstat(p)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		mode := os.FileMode(0o600)
		if info.IsDir() {
			mode = 0o700
		}
		if info.Mode().Perm()&^mode != 0 {
			if err := os.Chmod(p, info.Mode().Perm()&mode); err != nil {
				log.Printf("data folder: %v", err)
			}
		}
	}
}
