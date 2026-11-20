package keepawake

import (
	"time"

	"gocast/internal/sysfmt"
	"gocast/locales"
)

// Words say Keep Awake's state in one language (copy.md 18).
type Words struct {
	Msgs locales.Messages
	// Clock says a time of day: on the system's 12- or 24-hour clock in
	// the app (copy.md 18.0), a fixed one in the tests.
	Clock func(time.Time) string
}

// WordsIn returns the words in the language of locale, as "en" or
// "zh-Hans", with the system's clock.
func WordsIn(locale string) Words {
	return Words{Msgs: locales.In(locale), Clock: func(t time.Time) string { return sysfmt.Clock(t, locale) }}
}

// Length says a length as a title does: "1 Hour 30 Minutes", "30 分钟"
// (the presets' rows and the tray's submenu, copy.md 18.1, 18.3).
func (w Words) Length(d time.Duration) string {
	m := int(d / time.Minute)
	switch {
	case m < 60:
		return w.Msgs.AwakeMinutes(m)
	case m%60 == 0:
		return w.Msgs.AwakeHours(m / 60)
	}
	return w.Msgs.AwakeHoursMinutes(m/60, m%60)
}

// Title is the search row of r, typed now: "Keep Awake for 1 Hour 30
// Minutes", "保持唤醒到明天 08:00" (copy.md 18.1).
func (w Words) Title(r Request, now time.Time) string {
	switch r.Kind {
	case For:
		return w.Msgs.AwakeFor(w.Length(r.Duration))
	case Until:
		at := w.Clock(r.Deadline(now))
		if r.Tomorrow(now) {
			return w.Msgs.AwakeUntilTomorrow(at)
		}
		return w.Msgs.AwakeUntil(at)
	}
	return w.Msgs.AwakeTitle()
}

// left says the time left of l, a For: "1 hr 23 min", "25 分钟".
func (w Words) left(l Left) string {
	switch {
	case l.Hours == 0:
		return w.Msgs.AwakeLeftMinutes(l.Minutes)
	case l.Minutes == 0:
		return w.Msgs.AwakeLeftHours(l.Hours)
	}
	return w.Msgs.AwakeLeftHoursMinutes(l.Hours, l.Minutes)
}

// Detail is the small text of Stop Keeping Awake's search row: "还剩 1
// 小时 23 分", "还剩不到 1 分钟", "到 17:30", "到明天 08:00", "直到你停止"
// (copy.md 18.1, design.md 2.10); "" when off.
func (w Words) Detail(l Left) string {
	switch l.Kind {
	case Forever:
		return w.Msgs.AwakeLeftForever()
	case For:
		if l.LastMinute {
			return w.Msgs.AwakeLastMinute()
		}
		return w.Msgs.AwakeLeft(w.left(l))
	case Until:
		if l.Tomorrow {
			return w.Msgs.AwakeLeftUntilTomorrow(w.Clock(l.At))
		}
		return w.Msgs.AwakeLeftUntil(w.Clock(l.At))
	}
	return ""
}

// Status is the tray menu's first line while keeping awake: "Keeping
// Awake · 1 hr 23 min left" (copy.md 18.3); "" when off.
func (w Words) Status(l Left) string {
	switch l.Kind {
	case Forever:
		return w.Msgs.AwakeStatusForever()
	case For:
		if l.LastMinute {
			return w.Msgs.AwakeStatusLastMinute()
		}
		return w.Msgs.AwakeStatusLeft(w.left(l))
	case Until:
		if l.Tomorrow {
			return w.Msgs.AwakeStatusUntilTomorrow(w.Clock(l.At))
		}
		return w.Msgs.AwakeStatusUntil(w.Clock(l.At))
	}
	return ""
}

// Notice is the notice of a session just started (copy.md 18.2):
// "正在保持唤醒，还剩 1 小时 30 分"; when off, that of one stopped.
func (w Words) Notice(l Left) string {
	switch l.Kind {
	case Forever:
		return w.Msgs.AwakeNoticeForever()
	case For:
		if l.LastMinute {
			l.Minutes = 1
		}
		return w.Msgs.AwakeNoticeLeft(w.left(l))
	case Until:
		if l.Tomorrow {
			return w.Msgs.AwakeNoticeUntilTomorrow(w.Clock(l.At))
		}
		return w.Msgs.AwakeNoticeUntil(w.Clock(l.At))
	}
	return w.Msgs.AwakeNoticeStopped()
}
