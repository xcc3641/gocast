//go:build darwin

package clipboard

import (
	"errors"
	"unsafe"

	"github.com/ebitengine/purego/objc"
)

var (
	selFileURL      = objc.RegisterName("fileURLWithPath:")
	selNumberBool   = objc.RegisterName("numberWithBool:")
	selBoolValue    = objc.RegisterName("boolValue")
	selSetResource  = objc.RegisterName("setResourceValue:forKey:error:")
	selGetResource  = objc.RegisterName("getResourceValue:forKey:error:")
	selDescription  = objc.RegisterName("localizedDescription")
	errNoFoundation = errors.New("clipboard: Foundation unavailable")
)

// excludedKey is NSURLIsExcludedFromBackupKey.
func excludedKey() objc.ID {
	load()
	return objc.ID(symbol(libs.foundation, "NSURLIsExcludedFromBackupKey"))
}

// excludeFromBackup leaves the file or directory at path out of Time
// Machine backups: the item's own resource value
// (NSURLIsExcludedFromBackupKey), which goes with it; no system setting.
func excludeFromBackup(path string) error {
	key := excludedKey()
	if key == 0 {
		return errNoFoundation
	}
	var err error
	withPool(func() {
		url := class("NSURL").Send(selFileURL, class("NSString").Send(selString, path))
		yes := class("NSNumber").Send(selNumberBool, true)
		if url == 0 || yes == 0 {
			err = errNoFoundation
			return
		}
		var nsErr objc.ID
		if !objc.Send[bool](url, selSetResource, yes, key, unsafe.Pointer(&nsErr)) {
			msg := "clipboard: could not leave " + path + " out of backups"
			if nsErr != 0 {
				msg += ": " + goString(nsErr.Send(selDescription))
			}
			err = errors.New(msg)
		}
	})
	return err
}

// excludedFromBackup reports whether the item at path is left out of
// backups, as excludeFromBackup leaves it.
func excludedFromBackup(path string) (bool, error) {
	key := excludedKey()
	if key == 0 {
		return false, errNoFoundation
	}
	var (
		out bool
		err error
	)
	withPool(func() {
		url := class("NSURL").Send(selFileURL, class("NSString").Send(selString, path))
		if url == 0 {
			err = errNoFoundation
			return
		}
		var value, nsErr objc.ID
		if !objc.Send[bool](url, selGetResource, unsafe.Pointer(&value), key, unsafe.Pointer(&nsErr)) {
			err = errors.New("clipboard: could not read the backup attribute of " + path)
			return
		}
		out = value != 0 && objc.Send[bool](value, selBoolValue)
	})
	return out, err
}
