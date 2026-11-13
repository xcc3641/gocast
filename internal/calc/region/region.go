// Package region tells the system's region (Language & Region › Region),
// which the calculator uses to read ¥ and to pick the currency to convert
// into.
package region

import (
	"os"
	"strings"
)

// fromEnv is the region in a POSIX locale like "ja_JP.UTF-8" from
// LC_ALL, LC_MONETARY or LANG; "" when there is none.
func fromEnv() string {
	for _, k := range []string{"LC_ALL", "LC_MONETARY", "LANG"} {
		if r := parseLocale(os.Getenv(k)); r != "" {
			return r
		}
	}
	return ""
}

// parseLocale is the region of a locale identifier: "ja_JP.UTF-8" → "JP",
// "zh-Hans-CN" → "CN", "en" → "".
func parseLocale(s string) string {
	s, _, _ = strings.Cut(s, ".")
	s, _, _ = strings.Cut(s, "@")
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '_' || r == '-' })
	for _, p := range parts[min(1, len(parts)):] {
		if len(p) == 2 && isUpperASCII(p) {
			return p
		}
	}
	return ""
}

func isUpperASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 'A' || s[i] > 'Z' {
			return false
		}
	}
	return true
}
