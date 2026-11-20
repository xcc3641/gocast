package keepawake

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Request is a session to start.
type Request struct {
	Kind Kind
	// Duration is For's length, whole minutes.
	Duration time.Duration
	// Hour (0–23) and Minute are Until's time of day.
	Hour, Minute int
}

// The lengths a session can have.
const (
	MinDuration = time.Minute
	MaxDuration = 99 * time.Hour
)

// Presets are the lengths in the search panel and the tray's submenu
// (copy.md 18.1, 18.3).
var Presets = []time.Duration{30 * time.Minute, time.Hour, 2 * time.Hour, 4 * time.Hour}

// ErrInvalid is the error of what can't be parsed or started.
var ErrInvalid = errors.New("keepawake: invalid")

// Validate says what is wrong with r, if anything.
func (r Request) Validate() error {
	switch r.Kind {
	case Forever:
		return nil
	case For:
		if r.Duration < MinDuration || r.Duration > MaxDuration || r.Duration%time.Minute != 0 {
			return fmt.Errorf("%w: length %v", ErrInvalid, r.Duration)
		}
		return nil
	case Until:
		if r.Hour < 0 || r.Hour > 23 || r.Minute < 0 || r.Minute > 59 {
			return fmt.Errorf("%w: time %d:%02d", ErrInvalid, r.Hour, r.Minute)
		}
		return nil
	}
	return fmt.Errorf("%w: kind %d", ErrInvalid, r.Kind)
}

// Deadline is when r, started now, ends: zero for Forever; for Until, the
// time of day today, or tomorrow when that is not after now.
func (r Request) Deadline(now time.Time) time.Time {
	now = now.Round(0)
	switch r.Kind {
	case For:
		return now.Add(r.Duration)
	case Until:
		y, m, d := now.Date()
		t := time.Date(y, m, d, r.Hour, r.Minute, 0, 0, now.Location())
		if !t.After(now) {
			t = time.Date(y, m, d+1, r.Hour, r.Minute, 0, 0, now.Location())
		}
		return t
	}
	return time.Time{}
}

// Tomorrow reports whether r, an Until started now, ends tomorrow.
func (r Request) Tomorrow(now time.Time) bool {
	return r.Kind == Until && isTomorrow(r.Deadline(now), now.Round(0))
}

// Match is what ParseQuery found in a query.
type Match struct {
	// Request is the session typed, unless NeedTime.
	Request Request
	// NeedTime: only "keep awake until" was typed, not yet the time
	// (copy.md 18.1's hint).
	NeedTime bool
}

// Aliases are the other words Keep Awake is found by, besides its names
// (copy.md 18.1).
var Aliases = []string{
	"唤醒", "不睡眠", "不休眠", "防休眠", "防睡眠", "别睡", "屏幕常亮", "咖啡", "咖啡因",
	"keep awake", "awake", "caffeinate", "coffee", "amphetamine", "no sleep",
}

// commands are the words a typed length or time follows, Keep Awake's
// names and Aliases, longest first so that 咖啡因 is taken whole.
var commands = []string{
	"keep awake", "keepawake", "amphetamine", "caffeinate", "保持唤醒", "屏幕常亮",
	"no sleep", "coffee", "awake", "不睡眠", "不休眠", "防休眠", "防睡眠", "咖啡因", "唤醒", "别睡", "咖啡",
}

// untils are the words before a time of day.
var untils = []string{"until", "till", "til", "直到", "到"}

// ParseQuery finds a session to start in what is typed in the search
// panel (copy.md 18.1): Keep Awake's name or an alias and a length
// ("keep awake 1h30m", "唤醒 90m", "keep awake for 2 hours") or a time
// ("keep awake until 5pm", "保持唤醒到 17:30", "咖啡 下午5点"); or a
// time after "until" or "到" alone ("until 5pm", "到 17:30"). With only
// "keep awake until" or "保持唤醒到", the Match is NeedTime. Anything else,
// or a length or time it can't read, is no match: the panel shows Keep
// Awake itself, if its name matches, and nothing starts.
func ParseQuery(q string) (Match, bool) {
	s := normalize(q)
	rest, command := cutWord(s, commands)
	rest = strings.TrimSpace(rest)
	if t, ok := cutWord(rest, untils); ok {
		t = strings.TrimSpace(t)
		if t == "" {
			return Match{NeedTime: command}, command
		}
		h, m, err := ParseTime(t)
		if err != nil {
			return Match{}, false
		}
		return Match{Request: Request{Kind: Until, Hour: h, Minute: m}}, true
	}
	if !command || rest == "" {
		return Match{}, false
	}
	if r, ok := cutWord(rest, []string{"for"}); ok {
		rest = strings.TrimSpace(r)
	}
	if d, err := ParseDuration(rest); err == nil {
		return Match{Request: Request{Kind: For, Duration: d}}, true
	}
	if h, m, err := ParseTime(rest); err == nil {
		return Match{Request: Request{Kind: Until, Hour: h, Minute: m}}, true
	}
	return Match{}, false
}

// normalize lowercases s, turns full-width letters, digits and signs
// (as a Chinese input method types them: "９０ｍ", "17：30") into ASCII,
// and every run of spaces into one.
func normalize(s string) string {
	var b strings.Builder
	space := false
	for _, r := range strings.TrimSpace(s) {
		switch {
		case r >= 0xFF01 && r <= 0xFF5E:
			r -= 0xFEE0
		case r == '\u3000':
			r = ' '
		}
		if unicode.IsSpace(r) {
			if !space {
				b.WriteByte(' ')
			}
			space = true
			continue
		}
		space = false
		b.WriteRune(unicode.ToLower(r))
	}
	return strings.TrimSpace(b.String())
}

// cutWord cuts the first of words that s starts with off it. A word in
// Latin letters must end where the letters do: "until" is not cut from
// "untilted".
func cutWord(s string, words []string) (string, bool) {
	for _, w := range words {
		rest, ok := strings.CutPrefix(s, w)
		if !ok {
			continue
		}
		last, _ := utf8.DecodeLastRuneInString(w)
		next, _ := utf8.DecodeRuneInString(rest)
		if isLatin(last) && isLatin(next) {
			continue
		}
		return rest, true
	}
	return s, false
}

func isLatin(r rune) bool { return r < utf8.RuneSelf && unicode.IsLetter(r) }

// The units of a length, by what is typed.
var (
	hourUnits   = []string{"个小时", "个钟头", "小时", "钟头", "hours", "hour", "hrs", "hr", "h"}
	minuteUnits = []string{"分钟", "分", "minutes", "minute", "mins", "min", "m"}
)

// ParseDuration reads a length: hours, minutes or both, in figures or
// Chinese numerals, with or without spaces: "90m", "1h30m", "1h30",
// "1.5h", "2 hours", "1 hour 30 minutes", "1 hr and 30 min", "90分钟",
// "1小时30分", "一个半小时", "半小时", "两小时". It is rounded to the
// minute and must be between MinDuration and MaxDuration. A bare number
// is no length: "90" could be minutes or a time.
func ParseDuration(s string) (time.Duration, error) {
	s = normalize(s)
	bad := fmt.Errorf("%w: length %q", ErrInvalid, s)
	if s == "" {
		return 0, bad
	}
	var total float64
	hours, minutes := false, false
	for {
		s = strings.TrimSpace(s)
		if s == "" {
			break
		}
		if r, ok := cutWord(s, []string{"and", "又"}); ok && (hours || minutes) {
			s = strings.TrimSpace(r)
		}
		// 半小时, 半个小时: half an hour.
		if r, ok := cutWord(s, []string{"半个小时", "半个钟头", "半小时", "半钟头"}); ok && !hours && !minutes {
			total, hours, s = 30, true, r
			continue
		}
		n, r, ok := number(s)
		if !ok {
			return 0, bad
		}
		r = strings.TrimSpace(r)
		// 一个半小时: one and a half hours.
		if r2, ok := cutWord(r, []string{"个半小时", "个半钟头", "半小时", "半钟头"}); ok && !hours && !minutes {
			total, hours, s = (n+0.5)*60, true, r2
			continue
		}
		if r2, ok := cutWord(r, hourUnits); ok && !hours && !minutes {
			total += n * 60
			hours, s = true, r2
			continue
		}
		if r2, ok := cutWord(r, minuteUnits); ok && !minutes {
			total += n
			minutes, s = true, r2
			continue
		}
		// "1h30", "1小时30": minutes after hours, at the end.
		if r == "" && hours && !minutes && n == math.Trunc(n) && n < 60 {
			total += n
			minutes, s = true, ""
			continue
		}
		return 0, bad
	}
	d := time.Duration(math.Round(total)) * time.Minute
	if total < 1 || d > MaxDuration {
		return 0, bad
	}
	return d, nil
}

// number reads a number at the start of s: figures ("90", "1.5") or
// Chinese numerals ("九十", "两", "十五"), and what follows it.
func number(s string) (float64, string, bool) {
	i := 0
	for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == '.') {
		i++
	}
	if i > 0 {
		n, err := strconv.ParseFloat(s[:i], 64)
		if err != nil || s[0] == '.' {
			return 0, s, false
		}
		return n, s[i:], true
	}
	return chineseNumber(s)
}

var chineseDigits = map[rune]int{'零': 0, '〇': 0, '一': 1, '二': 2, '两': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9}

// chineseNumber reads a number under 1000 in Chinese numerals at the start
// of s: "十五", "二十", "一百二十".
func chineseNumber(s string) (float64, string, bool) {
	total, digit, read := 0, -1, 0
	for i, r := range s {
		switch d, isDigit := chineseDigits[r]; {
		case isDigit:
			if digit >= 0 {
				return 0, s, false // 一二: not a number
			}
			digit = d
		case r == '十':
			if digit < 0 {
				digit = 1
			}
			total += digit * 10
			digit = -1
		case r == '百':
			if digit < 0 {
				return 0, s, false
			}
			total += digit * 100
			digit = -1
		default:
			if read == 0 {
				return 0, s, false
			}
			if digit >= 0 {
				total += digit
			}
			return float64(total), s[i:], true
		}
		read = i + utf8.RuneLen(r)
	}
	if read == 0 {
		return 0, s, false
	}
	if digit >= 0 {
		total += digit
	}
	return float64(total), "", true
}

// The parts of the day a Chinese time can start with, and how each turns
// an hour on a 12-hour clock into one on a 24-hour clock.
var dayParts = []struct {
	word string
	to24 func(h int) (int, bool)
}{
	{"凌晨", func(h int) (int, bool) { return h % 12, h <= 6 || h == 12 }},
	{"半夜", func(h int) (int, bool) { return h % 12, h <= 4 || h == 12 }},
	{"早上", morning}, {"早晨", morning}, {"上午", morning},
	{"中午", func(h int) (int, bool) {
		if h >= 11 {
			return h, h <= 14
		}
		return h + 12, h <= 2
	}},
	{"下午", afternoon}, {"傍晚", afternoon},
	{"晚上", evening}, {"夜里", evening}, {"夜晚", evening},
}

func morning(h int) (int, bool) { return h, h <= 12 }

func afternoon(h int) (int, bool) {
	if h >= 13 {
		return h, h <= 23
	}
	return h%12 + 12, true
}

func evening(h int) (int, bool) {
	switch {
	case h == 12:
		return 0, true
	case h >= 5 && h <= 11:
		return h + 12, true
	}
	return h, h <= 4 || h >= 17 && h <= 23
}

// ParseTime reads a time of day: on a 24-hour clock ("17:30", "9:05"),
// with am or pm ("5pm", "5:30 pm", "5 p.m.", "12am"), or in Chinese
// ("17点", "17点30分", "5点半", "下午5点", "晚上8:30", "下午五点半"). A
// bare number is no time.
func ParseTime(s string) (hour, minute int, err error) {
	s = normalize(s)
	bad := fmt.Errorf("%w: time %q", ErrInvalid, s)
	switch s {
	case "noon", "中午":
		return 12, 0, nil
	case "midnight", "午夜":
		return 0, 0, nil
	}
	var to24 func(int) (int, bool)
	for _, p := range dayParts {
		if r, ok := strings.CutPrefix(s, p.word); ok {
			s, to24 = strings.TrimSpace(r), p.to24
			break
		}
	}
	n, rest, ok := number(s)
	if !ok || n != math.Trunc(n) || strings.Contains(s[:len(s)-len(rest)], ".") {
		return 0, 0, bad
	}
	h, m := int(n), 0
	marked := false // a minute, 点, am or pm says it is a time
	switch {
	case strings.HasPrefix(rest, ":"):
		mm := rest[1:]
		i := 0
		for i < len(mm) && mm[i] >= '0' && mm[i] <= '9' {
			i++
		}
		if i != 2 {
			return 0, 0, bad
		}
		m, _ = strconv.Atoi(mm[:2])
		rest, marked = mm[2:], true
	default:
		if r, ok := cutWord(rest, []string{"点", "时"}); ok {
			rest, marked = r, true
			switch {
			case strings.HasPrefix(rest, "半"):
				m, rest = 30, strings.TrimPrefix(rest, "半")
			case strings.HasPrefix(rest, "整"):
				rest = strings.TrimPrefix(rest, "整")
			default:
				if mn, r, ok := number(rest); ok {
					if mn != math.Trunc(mn) || strings.Contains(rest[:len(rest)-len(r)], ".") {
						return 0, 0, bad
					}
					m = int(mn)
					rest = strings.TrimPrefix(strings.TrimSpace(r), "分钟")
					rest = strings.TrimPrefix(rest, "分")
				}
			}
		}
	}
	rest = strings.TrimSpace(rest)
	if rest != "" {
		pm, ok := map[string]bool{"am": false, "a.m.": false, "a.m": false, "pm": true, "p.m.": true, "p.m": true}[rest]
		if !ok || to24 != nil || h < 1 || h > 12 {
			return 0, 0, bad
		}
		h %= 12
		if pm {
			h += 12
		}
		marked = true
	} else if to24 != nil {
		if h, ok = to24(h); !ok {
			return 0, 0, bad
		}
		marked = true
	}
	if !marked || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, bad
	}
	return h, m, nil
}
