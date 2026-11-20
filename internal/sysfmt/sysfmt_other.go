//go:build !darwin

package sysfmt

import "time"

// Date says when t was, as of now (macOS says it in the system's words).
func Date(t, now time.Time, _ string) string { return fallbackDate(t, now) }

// Bytes is a size in decimal units.
func Bytes(n int64) string { return fallbackBytes(n) }

// Day says the day of t (macOS says it in the system's words).
func Day(t, now time.Time, _ string, _ bool) string { return fallbackDay(t, now) }

// Clock says the time of day of t on a 24-hour clock (macOS says it on
// the system's clock).
func Clock(t time.Time, _ string) string { return fallbackClock(t) }

// Separators are "," and ".".
func Separators() (group, decimal string) { return ",", "." }
