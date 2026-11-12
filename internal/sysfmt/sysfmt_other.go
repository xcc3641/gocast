//go:build !darwin

package sysfmt

import "time"

// Date says when t was, as of now (macOS says it in the system's words).
func Date(t, now time.Time, _ string) string { return fallbackDate(t, now) }

// Bytes is a size in decimal units.
func Bytes(n int64) string { return fallbackBytes(n) }
