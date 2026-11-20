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
	// The relative cases on today's date, as the app has them.
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

func TestRelativeWordsOfNow(t *testing.T) {
	// The words are of now, not of the system's clock: today and
	// yesterday as of a day long past, at their time of day.
	now := time.Date(2020, 3, 9, 15, 0, 0, 0, time.Local)
	if got := Day(now.Add(-time.Hour), now, "en", false); got != "Today" {
		t.Errorf("today as of now: %q", got)
	}
	if got := Date(time.Date(2020, 3, 8, 14, 5, 0, 0, time.Local), now, "en"); !strings.HasPrefix(got, "Yesterday") || !strings.Contains(got, "05") {
		t.Errorf("yesterday as of now: %q", got)
	}
	// onSystemDay keeps the days between and the time of day.
	sysNow = func() time.Time { return time.Date(2026, 11, 17, 8, 0, 0, 0, time.Local) }
	t.Cleanup(func() { sysNow = time.Now })
	got := onSystemDay(time.Date(2020, 3, 8, 23, 30, 0, 0, time.Local), now)
	if want := time.Date(2026, 11, 16, 23, 30, 0, 0, time.Local); !got.Equal(want) {
		t.Errorf("onSystemDay = %v, want %v", got, want)
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

// Clock says the hour and minute on either clock: "17:30" or "5:30 PM".
func TestClock(t *testing.T) {
	at := time.Date(2026, 11, 12, 17, 30, 0, 0, time.Local)
	for _, locale := range []string{"en", "zh-Hans"} {
		s := Clock(at, locale)
		if !strings.Contains(s, "30") || !strings.Contains(s, "17") && !strings.Contains(s, "5") {
			t.Errorf("Clock(17:30, %s) = %q", locale, s)
		}
	}
}
