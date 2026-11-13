package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// legacyIdentifier is the bundle ID Gocast had before identifier, the name
// of the folder of its data then.
const legacyIdentifier = "dev.gocast.app"

// migrateDataDir copies Gocast's folder in base (Application Support) from
// the legacy bundle ID's name to the current one, once: when base/to does
// not exist and base/from does. The old folder is left as it is.
//
// The copy is made in a hidden sibling, .<to>.migrating, renamed to to
// when complete, so that a copy cut short (a crash, the Mac shutting down)
// never leaves a half folder behind that would stop the next launch from
// copying again; that next launch removes the sibling first. Files and
// folders keep their permissions and modification times, symbolic links
// stay links (to the same target); anything else (sockets, pipes) is
// skipped.
//
// It reports whether it copied. On an error nothing is at base/to, and the
// old folder is still intact.
func migrateDataDir(base, from, to string) (copied bool, err error) {
	dst := filepath.Join(base, to)
	tmp := filepath.Join(base, "."+to+".migrating")
	if err := removeTemp(tmp); err != nil {
		return false, fmt.Errorf("remove the copy cut short: %w", err)
	}
	if _, err := os.Lstat(dst); err == nil {
		return false, nil // migrated already, or a new install
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	src := filepath.Join(base, from)
	info, err := os.Stat(src)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil // nothing to migrate
	} else if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, fmt.Errorf("%s is not a folder", src)
	}
	if err := copyTree(src, tmp, info); err != nil {
		_ = removeTemp(tmp)
		return false, err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = removeTemp(tmp)
		return false, err
	}
	// Renaming a folder may touch its time: set it again.
	_ = os.Chtimes(dst, time.Time{}, info.ModTime())
	syncDir(base)
	return true, nil
}

// copyTree copies the folder src, whose info is info, to dst, which must
// not exist. The folder gets its permissions and time once its contents
// are in, as writing them changes its time (and its permissions might not
// let them be written).
func copyTree(src, dst string, info fs.FileInfo) error {
	if err := os.Mkdir(dst, 0o700); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		s, d := filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())
		fi, err := os.Lstat(s)
		if err != nil {
			return err
		}
		switch mode := fi.Mode(); {
		case mode.IsDir():
			err = copyTree(s, d, fi)
		case mode.IsRegular():
			err = copyFile(s, d, fi)
		case mode&fs.ModeSymlink != 0:
			var target string
			if target, err = os.Readlink(s); err == nil {
				err = os.Symlink(target, d)
			}
		}
		if err != nil {
			return err
		}
	}
	syncDir(dst)
	if err := os.Chmod(dst, info.Mode().Perm()); err != nil {
		return err
	}
	return os.Chtimes(dst, time.Time{}, info.ModTime())
}

// copyFile copies the regular file src, whose info is info, to dst, on
// disk before it returns.
func copyFile(src, dst string, info fs.FileInfo) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if err := os.Chmod(dst, info.Mode().Perm()); err != nil {
		return err
	}
	return os.Chtimes(dst, time.Time{}, info.ModTime())
}

// removeTemp removes the copy at tmp, if any, even with folders in it
// that the copy made read-only.
func removeTemp(tmp string) error {
	if _, err := os.Lstat(tmp); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	_ = filepath.WalkDir(tmp, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			_ = os.Chmod(p, 0o700)
		}
		return nil
	})
	return os.RemoveAll(tmp)
}

// syncDir puts the entries of the folder on disk, as far as it can.
func syncDir(dir string) {
	if f, err := os.Open(dir); err == nil {
		_ = f.Sync()
		f.Close()
	}
}
