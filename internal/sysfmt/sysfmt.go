// Package sysfmt writes dates and sizes as the system does, for the
// clipboard history's info (design.md 2.8.5, copy.md 6.3): when an item
// was last copied in the system's relative words and its 12- or 24-hour
// clock ("今天 14:05", "昨天 09:12", "11月10日 18:30"), and a size in its
// byte units ("84 KB"); and for the calculator's card (design.md 2.3a,
// copy.md 7.1), the day exchange rates are from ("今天", "11月10日") and
// the system's grouping separator and decimal point; and for Keep Awake
// (copy.md 18.0), a time of day on the system's 12- or 24-hour clock.
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

// sysNow is the system's clock, which Foundation's relative words are of.
var sysNow = time.Now

// onSystemDay is t moved by whole days to be as many days from the
// system's today as it is from now's, at the same time of day: Foundation
// says "Today" and "Yesterday" of the system's clock, whatever now is. In
// the app, now is the system's clock, and t stays as it is.
func onSystemDay(t, now time.Time) time.Time {
	loc := now.Location()
	date := func(t time.Time) time.Time {
		y, m, d := t.In(loc).Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) // UTC: no day 23 or 25 hours long
	}
	days := int(date(t).Sub(date(now)).Hours() / 24)
	y, m, d := sysNow().In(loc).Date()
	lt := t.In(loc)
	return time.Date(y, m, d+days, lt.Hour(), lt.Minute(), lt.Second(), lt.Nanosecond(), loc)
}
