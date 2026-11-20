package keepawake

import (
	"errors"
	"testing"
	"time"
)

// Lengths are read in English and Chinese, in figures or numerals, and
// rounded to the minute.
func TestParseDuration(t *testing.T) {
	for _, c := range []struct {
		in   string
		want time.Duration
	}{
		{"90m", 90 * time.Minute},
		{"1h30m", 90 * time.Minute},
		{"1h 30m", 90 * time.Minute},
		{"1h30", 90 * time.Minute},
		{"2h", 2 * time.Hour},
		{"1.5h", 90 * time.Minute},
		{"0.25h", 15 * time.Minute},
		{"30 min", 30 * time.Minute},
		{"30 mins", 30 * time.Minute},
		{"1 minute", time.Minute},
		{"45 minutes", 45 * time.Minute},
		{"1 hour", time.Hour},
		{"2 hours", 2 * time.Hour},
		{"2 hrs", 2 * time.Hour},
		{"1 hr 30 min", 90 * time.Minute},
		{"1 hour and 30 minutes", 90 * time.Minute},
		{"1H30M", 90 * time.Minute},
		{"９０ｍ", 90 * time.Minute},
		{"1小时30分", 90 * time.Minute},
		{"1小时30分钟", 90 * time.Minute},
		{"1 小时 30 分钟", 90 * time.Minute},
		{"1小时30", 90 * time.Minute},
		{"90分钟", 90 * time.Minute},
		{"90分", 90 * time.Minute},
		{"2个小时", 2 * time.Hour},
		{"3个钟头", 3 * time.Hour},
		{"半小时", 30 * time.Minute},
		{"半个小时", 30 * time.Minute},
		{"一个半小时", 90 * time.Minute},
		{"1个半小时", 90 * time.Minute},
		{"两小时", 2 * time.Hour},
		{"十五分钟", 15 * time.Minute},
		{"一小时二十分", 80 * time.Minute},
		{"一百二十分钟", 2 * time.Hour},
		{"99h", 99 * time.Hour},
	} {
		got, err := ParseDuration(c.in)
		if err != nil || got != c.want {
			t.Errorf("ParseDuration(%q) = %v, %v; want %v", c.in, got, err, c.want)
		}
	}
}

// What is not a length, or not one Keep Awake takes, is an error.
func TestParseDurationInvalid(t *testing.T) {
	for _, in := range []string{
		"", "90", "1:30", "17:30", "5pm", "abc", "h", "m30", "0m", "0.5m", "30s",
		"100h", "1h1h", "30m1h", "30m 10", "1h 60", "1h30x", "-1h",
		".5h", "1..5h", "1 hourly", "1 mint", "半", "一二分钟", "1小时半", "90m please",
	} {
		if d, err := ParseDuration(in); !errors.Is(err, ErrInvalid) {
			t.Errorf("ParseDuration(%q) = %v, %v; want ErrInvalid", in, d, err)
		}
	}
}

// Times of day are read on a 24-hour clock, with am and pm, and in
// Chinese with or without the part of the day; an hour up to 12 with
// none of these is Twelve.
func TestParseTime(t *testing.T) {
	for _, c := range []struct {
		in     string
		h, m   int
		twelve bool
	}{
		{"17:30", 17, 30, false},
		{"9:05", 9, 5, true},
		{"09:05", 9, 5, false},
		{"0:00", 0, 0, false},
		{"23:59", 23, 59, false},
		{"5pm", 17, 0, false},
		{"5 pm", 17, 0, false},
		{"5PM", 17, 0, false},
		{"5:30 pm", 17, 30, false},
		{"5:30pm", 17, 30, false},
		{"5 p.m.", 17, 0, false},
		{"11am", 11, 0, false},
		{"12am", 0, 0, false},
		{"12pm", 12, 0, false},
		{"12:15 am", 0, 15, false},
		{"noon", 12, 0, false},
		{"midnight", 0, 0, false},
		{"17：30", 17, 30, false},
		{"17点", 17, 0, false},
		{"17点30", 17, 30, false},
		{"17点30分", 17, 30, false},
		{"17时30分", 17, 30, false},
		{"5点半", 5, 30, true},
		{"8点整", 8, 0, true},
		{"下午5点", 17, 0, false},
		{"下午5点半", 17, 30, false},
		{"下午 5:30", 17, 30, false},
		{"下午五点半", 17, 30, false},
		{"下午12点", 12, 0, false},
		{"下午17点", 17, 0, false},
		{"上午10点", 10, 0, false},
		{"早上8点", 8, 0, false},
		{"中午12点", 12, 0, false},
		{"中午1点", 13, 0, false},
		{"傍晚6点", 18, 0, false},
		{"晚上8点", 20, 0, false},
		{"晚上8:30", 20, 30, false},
		{"晚上12点", 0, 0, false},
		{"凌晨2点", 2, 0, false},
		{"凌晨12点", 0, 0, false},
		{"十七点", 17, 0, false},
		{"5点", 5, 0, true},
		{"12点", 12, 0, true},
		{"12:15", 12, 15, true},
		{"05:00", 5, 0, false},
		{"05点", 5, 0, false},
		{"五点", 5, 0, true},
	} {
		want := Request{Kind: Until, Hour: c.h, Minute: c.m, Twelve: c.twelve}
		if got, err := ParseTime(c.in); err != nil || got != want {
			t.Errorf("ParseTime(%q) = %+v, %v; want %+v", c.in, got, err, want)
		}
	}
}

// What is not a time of day is an error.
func TestParseTimeInvalid(t *testing.T) {
	for _, in := range []string{
		"", "5", "17", "1730", "24:00", "17:60", "17:5", "17:305", "17.30", "13pm", "0am",
		"5 pm tomorrow", "下午5pm", "上午13点", "凌晨8点", "17点61分", "5.5点", "abc", "pm", "下午",
	} {
		if r, err := ParseTime(in); !errors.Is(err, ErrInvalid) {
			t.Errorf("ParseTime(%q) = %+v, %v; want ErrInvalid", in, r, err)
		}
	}
}

// A time of day typed without saying which half of the day ends at the
// sooner of its two times still to come; one written on a 24-hour clock
// or with its half of the day ends as written; "tomorrow" follows the
// time chosen.
func TestDeadlineTwelve(t *testing.T) {
	day := func(d, h, m int) time.Time { return time.Date(2026, 11, d, h, m, 0, 0, shanghai) }
	for _, c := range []struct {
		in       string
		now      time.Time
		want     time.Time
		tomorrow bool
	}{
		{"5点", day(19, 10, 0), day(19, 17, 0), false},
		{"5点", day(19, 20, 0), day(20, 5, 0), true},
		{"5点", day(19, 4, 0), day(19, 5, 0), false},
		{"5:30", day(19, 17, 30), day(20, 5, 30), true},
		{"5:30", day(19, 17, 29), day(19, 17, 30), false},
		{"12点", day(19, 13, 0), day(20, 0, 0), true},
		{"12点", day(19, 9, 0), day(19, 12, 0), false},
		{"12点", day(19, 11, 59), day(19, 12, 0), false},
		{"11:59", day(19, 11, 59), day(19, 23, 59), false},
		{"11:59", day(19, 11, 58), day(19, 11, 59), false},
		{"11:59", day(19, 23, 59), day(20, 11, 59), true},
		{"12:15", day(19, 23, 0), day(20, 0, 15), true},
		{"05:00", day(19, 10, 0), day(20, 5, 0), true},
		{"17:00", day(19, 10, 0), day(19, 17, 0), false},
		{"上午5点", day(19, 10, 0), day(20, 5, 0), true},
		{"5am", day(19, 10, 0), day(20, 5, 0), true},
		{"下午5点", day(19, 20, 0), day(20, 17, 0), true},
	} {
		r, err := ParseTime(c.in)
		if err != nil {
			t.Fatalf("ParseTime(%q): %v", c.in, err)
		}
		if got := r.Deadline(c.now); !got.Equal(c.want) {
			t.Errorf("%q at %s: %v, want %v", c.in, c.now.Format("15:04"), got, c.want)
		}
		if got := r.Tomorrow(c.now); got != c.tomorrow {
			t.Errorf("%q at %s: Tomorrow %v", c.in, c.now.Format("15:04"), got)
		}
	}
}

// The search panel's query gives a session after Keep Awake's name or an
// alias, or a time after "until" or 到; "until" alone after the name
// asks for the time; anything else is no session (copy.md 18.1).
func TestParseQuery(t *testing.T) {
	type m = Match
	until := func(h, min int) Match { return Match{Request: Request{Kind: Until, Hour: h, Minute: min}} }
	either := func(h, min int) Match {
		return Match{Request: Request{Kind: Until, Hour: h, Minute: min, Twelve: true}}
	}
	length := func(d time.Duration) Match { return Match{Request: Request{Kind: For, Duration: d}} }
	for _, c := range []struct {
		in   string
		want Match
		ok   bool
	}{
		{"keep awake 1h30m", length(90 * time.Minute), true},
		{"Keep Awake 1h30m", length(90 * time.Minute), true},
		{"keep awake for 2 hours", length(2 * time.Hour), true},
		{"keep awake for 90m", length(90 * time.Minute), true},
		{"keepawake 30m", length(30 * time.Minute), true},
		{"awake 45 min", length(45 * time.Minute), true},
		{"caffeinate 1h", length(time.Hour), true},
		{"coffee 2h", length(2 * time.Hour), true},
		{"amphetamine 1h", length(time.Hour), true},
		{"no sleep 1h", length(time.Hour), true},
		{"唤醒 90m", length(90 * time.Minute), true},
		{"保持唤醒 1小时30分", length(90 * time.Minute), true},
		{"保持唤醒1小时", length(time.Hour), true},
		{"保持唤醒 半小时", length(30 * time.Minute), true},
		{"不休眠 2小时", length(2 * time.Hour), true},
		{"咖啡因 90分钟", length(90 * time.Minute), true},
		{"咖啡 1h", length(time.Hour), true},
		{"屏幕常亮 1h", length(time.Hour), true},

		{"until 5pm", until(17, 0), true},
		{"until 17:30", until(17, 30), true},
		{"till 5:30 pm", until(17, 30), true},
		{"到 17:30", until(17, 30), true},
		{"到 5pm", until(17, 0), true},
		{"到下午5点", until(17, 0), true},
		{"直到 17:30", until(17, 30), true},
		{"keep awake until 5pm", until(17, 0), true},
		{"keep awake until17:30", until(17, 30), true},
		{"保持唤醒到 17:30", until(17, 30), true},
		{"保持唤醒到下午5点", until(17, 0), true},
		{"保持唤醒 到 17:30", until(17, 30), true},
		{"唤醒到 5pm", until(17, 0), true},
		{"keep awake 17:30", until(17, 30), true},
		{"唤醒 下午5点", until(17, 0), true},

		{"保持唤醒到", m{NeedTime: true}, true},
		{"keep awake until", m{NeedTime: true}, true},
		{"keep awake until ", m{NeedTime: true}, true},
		{"唤醒到", m{NeedTime: true}, true},

		{"keep awake", m{}, false},
		{"保持唤醒", m{}, false},
		{"keep awake for", m{}, false},
		{"keep awake soon", m{}, false},
		{"保持唤醒 一会儿", m{}, false},
		{"唤醒 90", length(90 * time.Minute), true},
		{"keep awake 90", length(90 * time.Minute), true},
		{"keep awake for 45", length(45 * time.Minute), true},
		{"保持唤醒 5940", length(MaxDuration), true},
		{"保持唤醒 5941", m{}, false},
		{"唤醒 0", m{}, false},
		{"until 90", m{}, false},
		{"到 5", m{}, false},
		{"保持唤醒到 5", m{}, false},
		{"到 5点", either(5, 0), true},
		{"到 5点半", either(5, 30), true},
		{"until 5:30", either(5, 30), true},
		{"唤醒 12点", either(12, 0), true},
		{"到 05:00", until(5, 0), true},
		{"keep awake 100h", m{}, false},
		{"keep awake until later", m{}, false},
		{"保持唤醒到 25:00", m{}, false},
		{"until", m{}, false},
		{"到", m{}, false},
		{"90m", m{}, false},
		{"17:30", m{}, false},
		{"awakening 1h", m{}, false},
		{"coffeescript 1h", m{}, false},
		{"untilted 5pm", m{}, false},
		{"到达", m{}, false},
		{"safari", m{}, false},
	} {
		got, ok := ParseQuery(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("ParseQuery(%q) = %+v, %v; want %+v, %v", c.in, got, ok, c.want, c.ok)
		}
		if ok && !got.NeedTime {
			if err := got.Request.Validate(); err != nil {
				t.Errorf("ParseQuery(%q): %v", c.in, err)
			}
		}
	}
}
