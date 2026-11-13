package region

import "testing"

func TestParseLocale(t *testing.T) {
	for in, want := range map[string]string{
		"ja_JP.UTF-8": "JP", "zh-Hans-CN": "CN", "en_US": "US", "de_DE@euro": "DE",
		"en": "", "C": "", "": "", "POSIX": "", "zh_Hant_TW.UTF-8": "TW", "jp": "",
	} {
		if got := parseLocale(in); got != want {
			t.Errorf("parseLocale(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MONETARY", "")
	t.Setenv("LANG", "ko_KR.UTF-8")
	if got := fromEnv(); got != "KR" {
		t.Errorf("fromEnv() = %q, want KR", got)
	}
}

func TestCode(t *testing.T) {
	// The system's region, whatever it is: two capitals or unknown.
	c := Code()
	if c != "" && (len(c) != 2 || !isUpperASCII(c)) {
		t.Errorf("Code() = %q", c)
	}
	if Code() != c {
		t.Error("Code() changes between calls")
	}
}
