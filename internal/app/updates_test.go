package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// TestUpdateStrings checks the update window's texts (copy.md 4.5): each
// one set in both languages, no exclamation marks, and the formats fit
// the arguments the plugin gives them.
func TestUpdateStrings(t *testing.T) {
	args := map[string]int{
		"UpToDateMessage":    2,
		"UnavailableMessage": 1,
		"Available":          1,
		"AvailableMessage":   3,
		"Progress":           2,
		"Megabytes":          1,
		"ReadyMessage":       2,
	}
	all := updateStrings()
	for _, lang := range []string{"en", "zh-Hans"} {
		s, ok := all[lang]
		if !ok {
			t.Fatalf("no strings for %s", lang)
		}
		v := reflect.ValueOf(s)
		for i := range v.NumField() {
			name, text := v.Type().Field(i).Name, v.Field(i).String()
			if text == "" {
				t.Errorf("%s %s is empty", lang, name)
			}
			if strings.ContainsAny(text, "!！") {
				t.Errorf("%s %s = %q has an exclamation mark", lang, name, text)
			}
			n := args[name]
			a := make([]any, n)
			for j := range a {
				a[j] = "x"
			}
			if out := fmt.Sprintf(text, a...); strings.Contains(out, "%!") {
				t.Errorf("%s %s = %q does not take %d arguments: %q", lang, name, text, n, out)
			}
		}
	}
	if got := all["zh-Hans"].Install; got != "安装更新" {
		t.Errorf("zh-Hans Install = %q", got)
	}
	if got := all["en"].RemindLater; got != "Remind Later" {
		t.Errorf("en RemindLater = %q", got)
	}
	if got := all["zh-Hans"].MenuItem; got != "检查更新…" {
		t.Errorf("zh-Hans MenuItem = %q", got)
	}
}
