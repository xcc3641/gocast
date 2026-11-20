package keepawake

import (
	"testing"
	"time"

	"gocast/locales"
)

// words are the words in locale on a fixed 24-hour clock, not the
// system's.
func words(locale string) Words {
	return Words{Msgs: locales.In(locale), Clock: func(t time.Time) string { return t.Format("15:04") }}
}

// What is left reads in hours and minutes, rounded up to the minute, then
// "less than a minute"; a time of day reads "until", "until tomorrow"
// past midnight (copy.md 18.0, 18.1, 18.3).
func TestDetail(t *testing.T) {
	for90 := State{Kind: For, Started: t0, Deadline: t0.Add(90 * time.Minute)}
	tomorrow := State{Kind: Until, Deadline: time.Date(2026, 11, 20, 8, 0, 0, 0, shanghai)}
	for _, c := range []struct {
		s      State
		at     time.Duration
		en, zh string
	}{
		{for90, 0, "1 hr 30 min left", "还剩 1 小时 30 分"},
		{for90, 7*time.Minute + time.Second, "1 hr 23 min left", "还剩 1 小时 23 分"},
		{for90, 30 * time.Minute, "1 hr left", "还剩 1 小时"},
		{for90, 65 * time.Minute, "25 min left", "还剩 25 分钟"},
		{for90, 89 * time.Minute, "1 min left", "还剩 1 分钟"},
		{for90, 89*time.Minute + time.Second, "Less than a minute left", "还剩不到 1 分钟"},
		{State{Kind: Until, Deadline: time.Date(2026, 11, 19, 17, 30, 0, 0, shanghai)}, 0, "Until 17:30", "到 17:30"},
		{tomorrow, 0, "Until tomorrow 08:00", "到明天 08:00"},
		{tomorrow, 10 * time.Hour, "Until 08:00", "到 08:00"},
		{State{Kind: Forever}, 0, "Until you stop", "直到你停止"},
		{State{}, 0, "", ""},
	} {
		l := c.s.Left(t0.Add(c.at))
		if got := words("en").Detail(l); got != c.en {
			t.Errorf("%+v en: %q, want %q", l, got, c.en)
		}
		if got := words("zh-Hans").Detail(l); got != c.zh {
			t.Errorf("%+v zh: %q, want %q", l, got, c.zh)
		}
	}
}

// A typed session's row says its length as a title does, and a time of
// day already past as tomorrow's (copy.md 18.1).
func TestTitle(t *testing.T) {
	for _, c := range []struct {
		r      Request
		en, zh string
	}{
		{Request{Kind: For, Duration: 30 * time.Minute}, "Keep Awake for 30 Minutes", "保持唤醒 30 分钟"},
		{Request{Kind: For, Duration: time.Hour}, "Keep Awake for 1 Hour", "保持唤醒 1 小时"},
		{Request{Kind: For, Duration: 4 * time.Hour}, "Keep Awake for 4 Hours", "保持唤醒 4 小时"},
		{Request{Kind: For, Duration: 90 * time.Minute}, "Keep Awake for 1 Hour 30 Minutes", "保持唤醒 1 小时 30 分钟"},
		{Request{Kind: For, Duration: 61 * time.Minute}, "Keep Awake for 1 Hour 1 Minute", "保持唤醒 1 小时 1 分钟"},
		{Request{Kind: Until, Hour: 17, Minute: 30}, "Keep Awake Until 17:30", "保持唤醒到 17:30"},
		{Request{Kind: Until, Hour: 8}, "Keep Awake Until Tomorrow 08:00", "保持唤醒到明天 08:00"},
		{Request{Kind: Forever}, "Keep Awake", "保持唤醒"},
	} {
		if got := words("en").Title(c.r, t0); got != c.en {
			t.Errorf("%+v en: %q, want %q", c.r, got, c.en)
		}
		if got := words("zh-Hans").Title(c.r, t0); got != c.zh {
			t.Errorf("%+v zh: %q, want %q", c.r, got, c.zh)
		}
	}
}

// The tray's line and the notice say the same as the row, in their own
// sentences (copy.md 18.2, 18.3).
func TestStatusAndNotice(t *testing.T) {
	l := State{Kind: For, Deadline: t0.Add(90 * time.Minute)}.Left(t0)
	en, zh := words("en"), words("zh-Hans")
	for _, c := range []struct{ got, want string }{
		{en.Status(l), "Keeping Awake · 1 hr 30 min left"},
		{zh.Status(l), "正在保持唤醒 · 还剩 1 小时 30 分"},
		{en.Notice(l), "Keeping awake, 1 hr 30 min left"},
		{zh.Notice(l), "正在保持唤醒，还剩 1 小时 30 分"},
		{en.Status(Left{Kind: For, LastMinute: true}), "Keeping Awake · less than a minute left"},
		{en.Status(Left{Kind: Forever}), "Keeping Awake · until you stop"},
		{zh.Status(Left{Kind: Until, At: t0, Tomorrow: true}), "正在保持唤醒 · 到明天 14:00"},
		{en.Notice(Left{Kind: Until, At: t0}), "Keeping awake until 14:00"},
		{zh.Notice(Left{Kind: Forever}), "正在保持唤醒，直到你停止"},
		{zh.Notice(Left{}), "已停止保持唤醒"},
		{en.Status(Left{}), ""},
	} {
		if c.got != c.want {
			t.Errorf("%q, want %q", c.got, c.want)
		}
	}
}
