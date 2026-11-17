// Package notes keeps Gocast's notes (design.md 11): one Markdown file
// a note in a folder of its own, read and written line by line to and
// from richtext's text, which the notes window edits.
package notes

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// ruleMark is how a rule is written. Not ---: under a line of text, as
// a note's rules often are, --- makes that line a heading in any other
// Markdown reader (a setext heading), where *** is a rule wherever it is.
const ruleMark = "***"

// ruleData marks the attachment that stands for a horizontal rule.
type ruleData struct{}

// Ink is the color a rule is drawn in, asked as it draws, for it to
// follow the appearance; nil draws it gray.
type Ink func() ui.Color

// Rule returns a horizontal rule: an attachment far wider than any
// editor, which the editor scales down to its width, a hairline drawn
// across its middle in ink's color (Spans give it its height).
func Rule(ink Ink) *richtext.Attachment {
	return &richtext.Attachment{
		Width: 4000, Height: 120, Name: ruleMark, Data: ruleData{},
		Draw: func(p *ui.Painter, r ui.Rect) {
			color := ui.RGBA(128, 128, 128, 0.4)
			if ink != nil {
				color = ink()
			}
			p.Fill(ui.Rect{X: r.X, Y: r.Y + r.H/2, W: r.W, H: 1}, color, 0)
		},
	}
}

// RuleRun is a paragraph's run of a rule alone.
func RuleRun(ink Ink) richtext.Run {
	return richtext.Run{Text: string(richtext.ObjectReplacement), Attributes: richtext.Attributes{Attachment: Rule(ink)}}
}

// IsRule reports whether the attachment is a rule's.
func IsRule(a *richtext.Attachment) bool {
	if a == nil {
		return false
	}
	_, ok := a.Data.(ruleData)
	return ok
}

// IsRuleParagraph reports whether paragraph p is a horizontal rule alone.
func IsRuleParagraph(p richtext.Paragraph) bool {
	return len(p.Runs) == 1 && IsRule(p.Runs[0].Attributes.Attachment)
}

// Encode writes t as Markdown, one paragraph a line, as Raycast's notes
// are: blank lines are empty paragraphs, a trailing newline ends the file,
// and characters are escaped only where reading the line back would
// otherwise change it.
func Encode(t richtext.Text) string {
	var lines []string
	var numbers []int
	inCode, codeQuote := false, ""
	for _, p := range t.Paragraphs() {
		st := p.Style
		q := ""
		if st.Quote {
			q = strings.Repeat("> ", max(st.QuoteDepth, 1))
		}
		if inCode && (!st.CodeBlock || q != codeQuote) {
			lines = append(lines, codeQuote+"```")
			inCode = false
		}
		if st.CodeBlock {
			if !inCode {
				lines = append(lines, q+"```")
				inCode, codeQuote = true, q
			}
			lines = append(lines, q+plain(p))
			continue
		}
		if st.List == richtext.NoList {
			numbers = numbers[:0]
		}
		if IsRuleParagraph(p) {
			lines = append(lines, q+ruleMark)
			continue
		}
		prefix := ""
		switch {
		case st.Heading > 0:
			prefix = strings.Repeat("#", st.Heading) + " "
		case st.List != richtext.NoList:
			for len(numbers) <= st.Indent {
				numbers = append(numbers, 0)
			}
			numbers = numbers[:st.Indent+1]
			prefix = strings.Repeat("    ", st.Indent)
			switch st.List {
			case richtext.Numbers:
				numbers[st.Indent]++
				prefix += strconv.Itoa(numbers[st.Indent]) + ". "
			case richtext.Tasks:
				numbers[st.Indent] = 0
				prefix += "- [ ] "
				if st.Checked {
					prefix = prefix[:len(prefix)-3] + "x] "
				}
			default:
				numbers[st.Indent] = 0
				prefix += "- "
			}
		}
		lines = append(lines, q+prefix+inline(p, prefix))
	}
	if inCode {
		lines = append(lines, codeQuote+"```")
	}
	return strings.Join(lines, "\n") + "\n"
}

func plain(p richtext.Paragraph) string {
	var b strings.Builder
	for _, r := range p.Runs {
		b.WriteString(r.Text)
	}
	return b.String()
}

var selfLink = regexp.MustCompile(`\[([^\]]+)\]\((https?://[^)\s]+)\)`)

// inline writes the runs of p as richtext does, then takes back each
// escape, entity and self-link that reading the line back does not need.
func inline(p richtext.Paragraph, prefix string) string {
	if len(p.Runs) == 0 {
		return ""
	}
	md := richtext.TextOf(richtext.Paragraph{Runs: p.Runs}).Markdown()
	want := runKey(p.Runs)
	same := func(s string) bool {
		got := DecodeLine(prefix+s, nil)
		return runKey(got.Runs) == want
	}
	// [url](url) → url, which the reader links again (GFM autolinks).
	md = selfLink.ReplaceAllStringFunc(md, func(m string) string {
		sub := selfLink.FindStringSubmatch(m)
		if strings.ReplaceAll(sub[1], `\`, "") == sub[2] {
			return sub[2]
		}
		return m
	})
	md = strings.NewReplacer("&#32;", " ", "&#9;", "\t").Replace(md)
	if !same(md) {
		md = richtext.TextOf(richtext.Paragraph{Runs: p.Runs}).Markdown()
	}
	if s := strings.ReplaceAll(md, `\`, ""); !strings.Contains(plain(p), `\`) && same(s) {
		return s
	}
	// One escape at a time, keeping those the line needs.
	for i := 0; i < len(md); i++ {
		if md[i] != '\\' || i+1 >= len(md) {
			continue
		}
		if s := md[:i] + md[i+1:]; same(s) {
			md = s
		} else if md[i+1] == '\\' {
			i++ // an escaped backslash the line needs
		}
	}
	return md
}

// runKey describes runs by what Markdown keeps of them, for comparing.
func runKey(runs []richtext.Run) string {
	var b strings.Builder
	for _, r := range runs {
		a := r.Attributes
		at := ""
		if a.Attachment != nil {
			at = a.Attachment.Name + a.Attachment.URL
		}
		for _, c := range r.Text {
			b.WriteRune(c)
			if !unicode.IsSpace(c) {
				b.WriteString(strconv.FormatBool(a.Bold()) + strconv.FormatBool(a.Italic) + strconv.FormatBool(a.Strikethrough) +
					strconv.FormatBool(a.Underline) + strconv.FormatBool(a.Code) + a.Link + at)
			}
			b.WriteByte(0)
		}
	}
	return strings.TrimRight(b.String(), " \x00")
}

var (
	hrLine   = regexp.MustCompile(`^ {0,3}((- *){3,}|(\* *){3,}|(_ *){3,})$`)
	heading  = regexp.MustCompile(`^(#{1,6}) (.*)$`)
	listItem = regexp.MustCompile(`^([ \t]*)([-*+]|\d{1,9}[.)]) (.*)$`)
	task     = regexp.MustCompile(`^\[([ xX])\] (.*)$`)
	fence    = regexp.MustCompile("^ {0,3}(```+|~~~+)")
	// tags richtext keeps; any other < is text.
	keptTag = regexp.MustCompile(`^</?(u|ins|s|strike|del|b|strong|i|em|br)\s*/?>|^<https?://[^>\s]+>`)
)

// Decode reads what Encode writes, and Markdown as people write it, line
// by line: a line is a paragraph, a blank line an empty one. Its rules
// are drawn in ink's color.
func Decode(s string, ink Ink) richtext.Text {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	var paras []richtext.Paragraph
	lines := strings.Split(s, "\n")
	var stack []int // the columns of the list items open
	for i := 0; i < len(lines); i++ {
		line, depth := unquote(lines[i])
		if m := fence.FindString(line); m != "" {
			close := strings.TrimLeft(m, " ")
			for i++; i < len(lines); i++ {
				l, _ := unquote(lines[i])
				if strings.HasPrefix(strings.TrimLeft(l, " "), close) && strings.TrimSpace(strings.Trim(l, close[:1])) == "" {
					break
				}
				p := richtext.Paragraph{Style: richtext.ParagraphStyle{CodeBlock: true, Quote: depth > 0, QuoteDepth: qd(depth)}}
				if l != "" {
					p.Runs = []richtext.Run{{Text: l}}
				}
				paras = append(paras, p)
			}
			continue
		}
		p := decodeLine(line, &stack, ink)
		p.Style.Quote, p.Style.QuoteDepth = depth > 0, qd(depth)
		paras = append(paras, p)
	}
	return richtext.TextOf(paras...)
}

func qd(depth int) int {
	if depth > 1 {
		return depth
	}
	return 0
}

// unquote takes the "> " marks off a line.
func unquote(l string) (string, int) {
	depth := 0
	for {
		t := strings.TrimLeft(l, " ")
		if !strings.HasPrefix(t, ">") || len(l)-len(t) > 3 {
			return l, depth
		}
		l = strings.TrimPrefix(t[1:], " ")
		depth++
	}
}

// DecodeLine reads one line, outside a block of code.
func DecodeLine(line string, ink Ink) richtext.Paragraph {
	var stack []int
	return decodeLine(line, &stack, ink)
}

func decodeLine(line string, stack *[]int, ink Ink) richtext.Paragraph {
	var st richtext.ParagraphStyle
	content := line
	switch {
	case hrLine.MatchString(line):
		*stack = (*stack)[:0]
		return richtext.Paragraph{Runs: []richtext.Run{RuleRun(ink)}}
	case heading.MatchString(line):
		m := heading.FindStringSubmatch(line)
		st.Heading = len(m[1])
		content = strings.TrimRight(strings.TrimRight(m[2], "#"), " ")
		if content == "" && strings.HasSuffix(m[2], "#") {
			content = m[2]
		}
		*stack = (*stack)[:0]
	case listItem.MatchString(line):
		m := listItem.FindStringSubmatch(line)
		col := len(strings.ReplaceAll(m[1], "\t", "    "))
		for len(*stack) > 0 && (*stack)[len(*stack)-1] > col {
			*stack = (*stack)[:len(*stack)-1]
		}
		if len(*stack) == 0 || (*stack)[len(*stack)-1] < col {
			*stack = append(*stack, col)
		}
		st.Indent = len(*stack) - 1
		st.List = richtext.Bullets
		if c := m[2][len(m[2])-1]; c == '.' || c == ')' {
			st.List = richtext.Numbers
		}
		content = m[3]
		if t := task.FindStringSubmatch(content); t != nil && st.List == richtext.Bullets {
			st.List, st.Checked, content = richtext.Tasks, t[1] != " ", t[2]
		}
	default:
		if strings.TrimSpace(line) != "" {
			*stack = (*stack)[:0]
		}
	}
	return richtext.Paragraph{Style: st, Runs: inlineRuns(content)}
}

// inlineRuns reads the inlines of a line: behind a no-break space, which
// starts no block and lets any mark open, then dropped; a < that is not a
// tag richtext keeps is escaped, as text.
func inlineRuns(s string) []richtext.Run {
	if s == "" {
		return nil
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '<' && !keptTag.MatchString(s[i:]) && (i == 0 || s[i-1] != '\\') {
			b.WriteByte('\\')
		}
		b.WriteByte(s[i])
	}
	t := richtext.ParseMarkdown("\u00a0" + b.String())
	var runs []richtext.Run
	for _, p := range t.Paragraphs() {
		if runs != nil {
			runs = append(runs, richtext.Run{Text: " "})
		}
		runs = append(runs, p.Runs...)
	}
	if len(runs) > 0 {
		_, n := utf8.DecodeRuneInString(runs[0].Text)
		runs[0].Text = runs[0].Text[n:]
		if runs[0].Text == "" {
			runs = runs[1:]
		}
	}
	return runs
}
