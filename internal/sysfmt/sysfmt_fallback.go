package sysfmt

import (
	"fmt"
	"time"
)

// fallbackDate is Date where the system can't say it.
func fallbackDate(t, now time.Time) string {
	if t.In(now.Location()).Year() != now.Year() {
		return t.Format("2006-01-02 15:04")
	}
	return t.Format("01-02 15:04")
}

// fallbackBytes is Bytes where the system can't say it, in decimal units
// as Finder's.
func fallbackBytes(n int64) string {
	switch {
	case n < 1000:
		return fmt.Sprintf("%d bytes", n)
	case n < 1000*1000:
		return fmt.Sprintf("%d KB", (n+500)/1000)
	case n < 1000*1000*1000:
		return fmt.Sprintf("%.1f MB", float64(n)/1e6)
	}
	return fmt.Sprintf("%.1f GB", float64(n)/1e9)
}
