package notes

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// The marks Plain takes off a line: those that start it (a quote's, a
// heading's, a list item's and a task's), and those around text in it.
var (
	quoteMark   = regexp.MustCompile(`^(\s{0,3}>\s?)+`)
	headingMark = regexp.MustCompile(`^#{1,6}\s+`)
	itemMark    = regexp.MustCompile(`^\s*([-*+]|\d{1,9}[.)])\s+(\[[ xX]\]\s+)?`)
	linkMark    = regexp.MustCompile(`!?\[([^\]]*)\]\([^)\s]*\)`)
	tagMark     = regexp.MustCompile(`</?(u|ins|s|strike|del|b|strong|i|em|br)\s*/?>`)
	emphasis    = strings.NewReplacer("**", "", "__", "", "~~", "", "`", "", "*", "")
	escaped     = regexp.MustCompile(`\\([\\!"#$%&'()*+,\-./:;<=>?@\[\]^_{|}~])`)
)

// Plain is a note's Markdown as the lines of text it shows, without the
// marks: the first is its title, as the window's top bar and Browse
// Notes show it (copy.md 16.2: "# ", "- [ ] ", "> " and the others
// gone), trimmed. The lines of code are as they are, the fences and rules
// empty lines.
func Plain(md string) []string {
	md = strings.TrimSuffix(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	if md == "" {
		return nil
	}
	lines := strings.Split(md, "\n")
	inCode := false
	for i, l := range lines {
		l = quoteMark.ReplaceAllString(l, "")
		if fence.MatchString(l) {
			inCode = !inCode
			lines[i] = ""
			continue
		}
		if inCode {
			lines[i] = strings.TrimSpace(l)
			continue
		}
		if hrLine.MatchString(l) {
			lines[i] = ""
			continue
		}
		l = headingMark.ReplaceAllString(l, "")
		l = itemMark.ReplaceAllString(l, "")
		l = linkMark.ReplaceAllString(l, "$1")
		l = tagMark.ReplaceAllString(l, "")
		l = escaped.ReplaceAllString(emphasisOutsideEscapes(l), "$1")
		lines[i] = strings.TrimSpace(l)
	}
	return lines
}

// emphasisOutsideEscapes takes the marks of emphasis, strikethrough and
// code off s, but those escaped ("\*"), which are text.
func emphasisOutsideEscapes(s string) string {
	if !strings.Contains(s, `\`) {
		return emphasis.Replace(s)
	}
	var b strings.Builder
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			b.WriteString(emphasis.Replace(s[start:i]))
			b.WriteString(s[i : i+2])
			i++
			start = i + 1
		}
	}
	b.WriteString(emphasis.Replace(s[start:]))
	return b.String()
}

// Match is a note a search found: in its title, or else in its text,
// where Snippet is the line it was found in, from about snippetBefore
// characters before it ("…" where cut).
type Match struct {
	Note    *Note
	InTitle bool
	Snippet string
}

// snippetBefore is about how many characters a snippet keeps before what
// was found (design.md 11.6).
const snippetBefore = 12

// Search finds the notes of list whose title or text has query, without
// regard to case, in list's order; an empty query finds them all.
func Search(list []*Note, query string) []Match {
	q := strings.ToLower(strings.TrimSpace(query))
	out := make([]Match, 0, len(list))
	for _, n := range list {
		if q == "" {
			out = append(out, Match{Note: n, InTitle: true})
			continue
		}
		if m, ok := n.find(q); ok {
			out = append(out, m)
		}
	}
	return out
}

// find finds q, in lower case, in the note.
func (n *Note) find(q string) (Match, bool) {
	for i, l := range n.lower {
		at := strings.Index(l, q)
		if at < 0 {
			continue
		}
		if i == 0 {
			return Match{Note: n, InTitle: true}, true
		}
		return Match{Note: n, Snippet: snippet(n.Lines[i], utf8.RuneCountInString(l[:at]))}, true
	}
	return Match{}, false
}

// snippet is line from about snippetBefore characters before the
// character at, "…" before it where cut.
func snippet(line string, at int) string {
	r := []rune(line)
	from := max(0, at-snippetBefore)
	if from == 0 {
		return line
	}
	return "…" + strings.TrimLeft(string(r[min(from, len(r)):]), " ")
}
