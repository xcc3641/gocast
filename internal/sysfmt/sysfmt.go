// Package sysfmt writes dates and sizes as the system does, for the
// clipboard history's info (design.md 2.8.5, copy.md 6.3): when an item
// was last copied in the system's relative words and its 12- or 24-hour
// clock ("今天 14:05", "昨天 09:12", "11月10日 18:30"), and a size in its
// byte units ("84 KB").
package sysfmt

import "time"

// relativeDay reports whether t is today or yesterday as of now, by the
// local dates in now's location: those the system says in words.
func relativeDay(t, now time.Time) bool {
	day := func(t time.Time) time.Time {
		y, m, d := t.In(now.Location()).Date()
		return time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	}
	today, then := day(now), day(t)
	return !then.Before(today.AddDate(0, 0, -1)) && !then.After(today)
}
