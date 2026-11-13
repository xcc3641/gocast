package sysfmt

import (
	"strings"
	"testing"
	"time"
)

func TestRelativeDay(t *testing.T) {
	now := time.Date(2026, 11, 12, 9, 0, 0, 0, time.Local)
	for _, c := range []struct {
		t    time.Time
		want bool
	}{
		{now.Add(-time.Hour), true},
		{time.Date(2026, 11, 11, 0, 1, 0, 0, time.Local), true},
		{time.Date(2026, 11, 10, 23, 59, 0, 0, time.Local), false},
	} {
		if got := relativeDay(c.t, now); got != c.want {
			t.Errorf("relativeDay(%v) = %v", c.t, got)
		}
	}
}

func TestDate(t *testing.T) {
	// macOS words 今天 and Yesterday against the real date, not now, so
	// the relative cases sit on today's date.
	y, m, d := time.Now().Date()
	now := time.Date(y, m, d, 15, 0, 0, 0, time.Local)
	today := time.Date(y, m, d, 14, 5, 0, 0, time.Local)
	if got := Date(today, now, "zh-Hans"); !strings.HasPrefix(got, "今天") || !strings.Contains(got, "05") {
		t.Errorf("today in Chinese: %q", got)
	}
	if got := Date(today.AddDate(0, 0, -1), now, "en"); !strings.HasPrefix(got, "Yesterday") {
		t.Errorf("yesterday in English: %q", got)
	}
	now = time.Date(2026, 11, 12, 15, 0, 0, 0, time.Local)
	earlier := time.Date(2026, 11, 10, 18, 30, 0, 0, time.Local)
	if got := Date(earlier, now, "zh-Hans"); !strings.Contains(got, "11月10日") || strings.Contains(got, "2026") {
		t.Errorf("earlier this year in Chinese: %q", got)
	}
	if got := Date(earlier.AddDate(-1, 0, 0), now, "en"); !strings.Contains(got, "2025") {
		t.Errorf("last year in English: %q", got)
	}
}

func TestBytes(t *testing.T) {
	if got := Bytes(84_000); !strings.Contains(got, "84") || !strings.Contains(got, "KB") {
		t.Errorf("Bytes(84000) = %q", got)
	}
	if got := fallbackBytes(5_300_000); got != "5.3 MB" {
		t.Errorf("fallbackBytes = %q", got)
	}
}

func TestDay(t *testing.T) {
	y, m, d := time.Now().Date()
	now := time.Date(y, m, d, 15, 0, 0, 0, time.Local)
	if got := Day(now.Add(-time.Hour), now, "zh-Hans", false); got != "今天" {
		t.Errorf("today in Chinese: %q", got)
	}
	if got := Day(now.AddDate(0, 0, -1), now, "en", false); got != "Yesterday" {
		t.Errorf("yesterday in English: %q", got)
	}
	now = time.Date(2026, 11, 13, 15, 0, 0, 0, time.Local)
	earlier := time.Date(2026, 11, 10, 0, 0, 0, 0, time.Local)
	if got := Day(earlier, now, "zh-Hans", false); got != "11月10日" {
		t.Errorf("earlier this year in Chinese: %q", got)
	}
	if got := Day(earlier, now, "en", true); !strings.Contains(got, "November") || strings.Contains(got, "2026") {
		t.Errorf("earlier this year in English, long: %q", got)
	}
	if got := Day(earlier.AddDate(-1, 0, 0), now, "en", false); !strings.Contains(got, "2025") {
		t.Errorf("last year in English: %q", got)
	}
}

func TestSeparators(t *testing.T) {
	group, decimal := Separators()
	if decimal == "" || decimal == group {
		t.Errorf("Separators() = %q, %q", group, decimal)
	}
}
