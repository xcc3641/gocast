package apps

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestNamesOfStrings(t *testing.T) {
	app := filepath.Join(t.TempDir(), "WeChat.app")
	res := filepath.Join(app, "Contents", "Resources")
	for _, d := range []string{"en.lproj", "zh-Hans.lproj"} {
		if err := os.MkdirAll(filepath.Join(res, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(p string, b []byte) {
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(app, "Contents", "Info.plist"), []byte(`<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict><key>CFBundleName</key><string>WeChat</string><key>CFBundleDisplayName</key><string>$(PRODUCT_NAME)</string></dict></plist>`))
	write(filepath.Join(res, "en.lproj", "InfoPlist.strings"), []byte("/* English */\n\"CFBundleDisplayName\" = \"WeChat\";\n"))
	// UTF-16 with a byte order mark, as Xcode writes them.
	s := "\ufeff\"CFBundleDisplayName\" = \"微信\";\n\"CFBundleName\" = \"微信\";\n"
	var u16 []byte
	for _, r := range s {
		u16 = append(u16, byte(r), byte(r>>8))
	}
	write(filepath.Join(res, "zh-Hans.lproj", "InfoPlist.strings"), u16)

	info := Names(app)
	if got, want := info.Names, []string{"WeChat", "微信"}; !slices.Equal(got, want) {
		t.Errorf("Names: %q, want %q", got, want)
	}
	if got := info.Localized("zh-Hans"); got != "微信" {
		t.Errorf("zh-Hans: %q", got)
	}
	if got := info.Localized("en"); got != "WeChat" {
		t.Errorf("en: %q", got)
	}
	if got := info.Localized("zh-Hant"); got != "" {
		t.Errorf("zh-Hant: %q, want none", got)
	}
}

// TestSystemApps reads the bundles of apps every Mac has, where they are.
func TestSystemApps(t *testing.T) {
	const calc = "/System/Applications/Calculator.app"
	if _, err := os.Stat(calc); err != nil {
		t.Skip("no Calculator here")
	}
	info := Names(calc)
	if !slices.Contains(info.Names, "Calculator") || !slices.Contains(info.Names, "计算器") {
		t.Errorf("Calculator: %q", info.Names)
	}
	if got := info.Localized("zh-Hans"); got != "计算器" {
		t.Errorf("Calculator in zh-Hans: %q", got)
	}
}
