//go:build !darwin

package region

// Code is the region of the POSIX locale ("ja_JP.UTF-8" → "JP"); "" when
// unknown.
func Code() string { return fromEnv() }
