package app

// The notes window's editor (design.md 11.4, 11.9): the keys of the
// formats and of the window, what typing makes (a rule, a link), the
// links opened with Command, and find.

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"

	"gocast/internal/notes"
)

// The find bar's highlights (design.md 11.4): every match, and the one
// shown, whose text is dark in both appearances.
var (
	findHighlight   = ui.RGBA(255, 214, 10, 0.35)
	findCurrent     = ui.Hex("#FFD60A")
	findCurrentText = ui.Hex("#1D1D1F")
)

// noteFind is the find bar's state.
type noteFind struct {
	open      bool
	query     string
	match     int      // the match shown
	matches   [][2]int // the runes found, start and end
	input     ui.Handle
	composing bool
	focus     bool   // focus the field in the next frame
	of        uint64 // the version of the text matches are of
	ofDoc     *richtext.Document
	ofQuery   string
}

// formatKey is what a format's key does in the note (design.md 11.9),
// nil to leave the key to the editor.
func (n *notesWindow) formatKey(d *richtext.Document, key ui.Key, mods ui.Modifiers) func() {
	switch mods {
	case ui.Cmd:
		switch key {
		case ui.KeyL:
			return func() { n.link(d) }
		case ui.KeyEnter:
			return func() {
				_, caret := d.Selection()
				d.ToggleChecked(caret)
			}
		case ui.KeyV:
			if url := oneURL(n.services.ReadClipboard()); url != "" {
				if a, c := d.Selection(); a != c {
					return func() { d.SetLink(url) }
				}
			}
		}
	case ui.Cmd | ui.Shift:
		switch key {
		case ui.KeyS:
			return d.ToggleStrikethrough
		case ui.KeyB:
			return d.ToggleQuote
		case ui.Key7:
			return func() { d.ToggleList(richtext.Numbers) }
		case ui.Key8:
			return func() { d.ToggleList(richtext.Bullets) }
		case ui.Key9:
			return func() { d.ToggleList(richtext.Tasks) }
		}
	case ui.Cmd | ui.Alt:
		switch key {
		case ui.Key1, ui.Key2, ui.Key3:
			level := int(key-ui.Key1) + 1
			return func() {
				if d.Formatting().Paragraph.Heading == level {
					level = 0 // back to text (design.md 11.9)
				}
				d.SetHeading(level)
			}
		case ui.KeyC:
			return d.ToggleCodeBlock
		}
	}
	return nil
}

// onKey takes the keys the editor would otherwise: the window's, then
// the formats'.
func (n *notesWindow) onKey(key ui.Key, mods ui.Modifiers) bool {
	if n.command(key, mods) {
		return true
	}
	if n.cur == nil {
		return false
	}
	if fn := n.formatKey(n.cur.doc, key, mods); fn != nil {
		fn()
		return true
	}
	return false
}

// link links the selection (⌘L): to itself when it is a URL, else to
// the URL on the clipboard; a link at the caret, with no URL to link to,
// is unlinked.
func (n *notesWindow) link(d *richtext.Document) {
	a, z := d.Selection()
	a, z = min(a, z), max(a, z)
	if a == z {
		return
	}
	if url := oneURL(d.Text().Slice(a, z).String()); url != "" {
		d.SetLink(url)
		return
	}
	if url := oneURL(n.services.ReadClipboard()); url != "" {
		d.SetLink(url)
		return
	}
	if d.Formatting().Link != "" {
		d.SetLink("")
	}
}

// oneURL is s trimmed when it is one URL (http, https or mailto), else "".
func oneURL(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, " \n\t") {
		return ""
	}
	for _, p := range []string{"http://", "https://", "mailto:"} {
		if strings.HasPrefix(s, p) && len(s) > len(p) {
			return s
		}
	}
	return ""
}

// observe sees the input before the editor: what was typed, for the
// shortcuts made after an edit; whether the input method composes (the
// editor's element does not say); and whether Command was down as the
// pointer went down, for a link to open.
func (n *notesWindow) observe(ev ui.InputEvent) {
	switch ev.Kind {
	case ui.InputCompose:
		n.composing = ev.Text != ""
		if n.composing {
			n.spansVer++ // find's highlights wait
		}
	case ui.InputPointerDown:
		n.cmdDown = ev.Mods&ui.Cmd != 0
		n.typed = ""
	case ui.InputText:
		n.typed = ev.Text
		if n.composing {
			n.composing = false
			n.spansVer++
		}
	case ui.InputKeyDown, ui.InputCommand:
		n.typed = ""
	}
}

// openLink opens a link clicked with Command in the default browser; a
// click alone places the caret (design.md 11.4).
func (n *notesWindow) openLink(url string) {
	if n.cmdDown {
		n.services.OpenURL(url)
	}
}

// afterEdit follows an edit the user made: the full-width marks of an
// input method at the start of a line read as Markdown's, ---
// becomes a rule, [text](url) a link; then the note is written soon.
func (n *notesWindow) afterEdit(d *richtext.Document) {
	n.halfWidthMarks(d)
	n.ruleShortcut(d)
	n.linkShortcut(d)
	n.plainAfterBlock(d)
	n.edited()
}

// halfWidth are the full-width marks a Chinese input method types, by
// the Markdown mark they stand for at the start of a line.
var halfWidth = map[string]string{"＃": "#", "＞": ">", "－": "-", "＊": "*", "·": "`", "｀": "`"}

// halfWidthMarks makes a full-width mark just typed at the start of a
// line, the line marks alone, Markdown's: then the space after it makes
// the heading, the quote or the list it makes typed in ASCII.
func (n *notesWindow) halfWidthMarks(d *richtext.Document) {
	half, ok := halfWidth[n.typed]
	if !ok {
		return
	}
	t := d.Text()
	a, caret := d.Selection()
	if a != caret || caret == 0 {
		return
	}
	ps, _ := t.ParagraphRange(caret)
	before := t.Slice(ps, caret).String()
	for _, r := range before {
		if _, mark := halfWidth[string(r)]; !mark && !strings.ContainsRune("#>-*`", r) {
			return
		}
	}
	attrs := t.AttributesAt(caret - 1)
	d.Edit(func(e *richtext.Edit) {
		e.ReplaceString(caret-1, caret, half, attrs)
		e.Select(caret, caret)
	})
	n.typed = half
}

// ruleShortcut makes ---, ___ or *** typed alone on a line of text a
// rule (design.md 11.4), which undo takes back to the marks.
func (n *notesWindow) ruleShortcut(d *richtext.Document) {
	if n.typed != "-" && n.typed != "_" && n.typed != "*" {
		return
	}
	t := d.Text()
	_, caret := d.Selection()
	ps, pe := t.ParagraphRange(caret)
	line := t.Slice(ps, pe).String()
	st := t.ParagraphStyleAt(ps)
	if (line != "---" && line != "___" && line != "***") || caret != pe || st != (richtext.ParagraphStyle{ID: st.ID}) {
		return
	}
	rule := notes.RuleRun(n.ruleInk)
	d.Edit(func(e *richtext.Edit) {
		e.ReplaceString(ps, pe, rule.Text, rule.Attributes)
		e.ReplaceString(ps+1, ps+1, "\n", richtext.Attributes{})
		e.Select(ps+2, ps+2)
	})
}

// mdLink is a link typed in Markdown, ending at the caret.
var mdLink = regexp.MustCompile(`\[([^\]\n]+)\]\(([^)\s]+)\)$`)

// linkShortcut makes [text](url) just typed a link, as Raycast does and
// richtext's shortcuts do not.
func (n *notesWindow) linkShortcut(d *richtext.Document) {
	t := d.Text()
	a, caret := d.Selection()
	if a != caret || caret == 0 || n.typed != ")" {
		return
	}
	ps, _ := t.ParagraphRange(caret)
	before := t.Slice(ps, caret).String()
	m := mdLink.FindStringSubmatchIndex(before)
	if m == nil {
		return
	}
	start := ps + utf8.RuneCountInString(before[:m[0]])
	label, url := before[m[2]:m[3]], before[m[4]:m[5]]
	attrs := t.AttributesAt(start)
	attrs.Link = url
	d.Edit(func(e *richtext.Edit) {
		e.ReplaceString(start, caret, label, attrs)
		end := start + utf8.RuneCountInString(label)
		e.Select(end, end)
	})
}

// plainAfterBlock works around richtext making the text typed bold after
// a block's shortcut ("> ", "- ", "# ") on a line after one an inline
// shortcut made bold (MyGo at 87b82398): the line the shortcut just made,
// empty, types plain.
func (n *notesWindow) plainAfterBlock(d *richtext.Document) {
	if n.typed != " " {
		return
	}
	t := d.Text()
	a, caret := d.Selection()
	ps, pe := t.ParagraphRange(caret)
	if a != caret || ps != pe {
		return
	}
	st := t.ParagraphStyleAt(ps)
	if st.Heading == 0 && st.List == richtext.NoList && !st.Quote {
		return
	}
	if ta := d.TypingAttributes(); ta.Bold() {
		ta.Weight = 0
		d.SetTypingAttributes(ta)
	}
}

// --- What the editor lays out ---

// snapshot keeps the text's paragraphs for spans and space, which run as
// the editor lays it out, when it changed.
func (n *notesWindow) snapshot(d *richtext.Document) {
	if v := d.Version(); v == n.textVer && d == n.textDoc && n.styles != nil {
		return
	}
	n.textVer, n.textDoc = d.Version(), d
	n.text = d.Text()
	n.styles, n.rules, n.paraAt = n.styles[:0], n.rules[:0], n.paraAt[:0]
	for start, para := range n.text.Paragraphs() {
		n.styles = append(n.styles, para.Style)
		n.rules = append(n.rules, notes.IsRuleParagraph(para))
		n.paraAt = append(n.paraAt, start)
	}
}

// spans styles the paragraph para (design.md 11.4) and colors the matches
// of find in it, but while the input method composes, as they would move.
func (n *notesWindow) spans(para int, st richtext.ParagraphStyle, spans []ui.Span) []ui.Span {
	if para >= len(n.paraAt) {
		return spans
	}
	start := n.paraAt[para]
	spans = n.design.spans(st, spans, start, n.text.AttributesAt)
	if len(n.find.matches) == 0 || !n.find.open || n.composing {
		return spans
	}
	var out []ui.Span
	at := start
	for _, s := range spans {
		runes := []rune(s.Text)
		if len(runes) == 0 {
			out = append(out, s)
			continue
		}
		from := 0
		for k := range runes {
			hl := n.highlightAt(at + k)
			if k == len(runes)-1 || n.highlightAt(at+k+1) != hl {
				part := s
				part.Text = string(runes[from : k+1])
				switch hl {
				case 1:
					part.Background = findHighlight
				case 2:
					part.Background, part.Color = findCurrent, findCurrentText
				}
				out = append(out, part)
				from = k + 1
			}
		}
		at += len(runes)
	}
	return out
}

// highlightAt is how the rune i is found: 0 not, 1 a match, 2 the match
// shown.
func (n *notesWindow) highlightAt(i int) int {
	for k, m := range n.find.matches {
		if i >= m[0] && i < m[1] {
			if k == n.find.match {
				return 2
			}
			return 1
		}
		if m[0] > i {
			break
		}
	}
	return 0
}

// space is the room Space adds above paragraph p.
func (n *notesWindow) space(p int) float32 { return n.design.space(n.styles, n.rules, p) }

// --- Find ---

// openFind shows the find bar, its field focused, the selection to find
// when there is one on a line.
func (n *notesWindow) openFind() {
	if n.cur == nil {
		return
	}
	f := &n.find
	if a, z := n.cur.doc.Selection(); a != z {
		if s := n.cur.doc.Text().Slice(min(a, z), max(a, z)).String(); !strings.Contains(s, "\n") {
			f.query = s
		}
	}
	f.open, f.focus = true, true
	f.ofDoc = nil // find again
	n.redraw()
}

// closeFind closes the find bar, the match shown selected, the caret in
// the text (design.md 11.4).
func (n *notesWindow) closeFind() {
	f := &n.find
	if !f.open {
		return
	}
	if n.cur != nil && f.match < len(f.matches) {
		m := f.matches[f.match]
		n.cur.doc.SetSelection(m[0], m[1])
	}
	f.open, f.matches = false, nil
	n.spansVer++
	n.focusText = true
	n.redraw()
}

// findAll finds the query in the note showing, whatever the case, as
// the text or the query change.
func (n *notesWindow) findAll() {
	f := &n.find
	if !f.open || n.cur == nil {
		return
	}
	d := n.cur.doc
	if f.ofDoc == d && f.of == d.Version() && f.ofQuery == f.query {
		return
	}
	f.ofDoc, f.of, f.ofQuery = d, d.Version(), f.query
	found := findRunes(d.Text().String(), f.query)
	same := len(found) == len(f.matches)
	for i := 0; same && i < len(found); i++ {
		same = found[i] == f.matches[i]
	}
	if !same {
		f.matches = found
		f.match = min(f.match, max(len(found)-1, 0))
		n.spansVer++
	}
}

// findRunes finds q in text, whatever the case, by runes: the ranges, not
// overlapping.
func findRunes(text, q string) [][2]int {
	if q == "" {
		return nil
	}
	t := []rune(text)
	qr := []rune(strings.ToLower(q))
	var found [][2]int
	for i := 0; i+len(qr) <= len(t); i++ {
		ok := true
		for k, r := range qr {
			if unicode.ToLower(t[i+k]) != r {
				ok = false
				break
			}
		}
		if ok {
			found = append(found, [2]int{i, i + len(qr)})
			i += len(qr) - 1
		}
	}
	return found
}

// stepFind shows the next match (step 1) or the one before (-1),
// wrapping, scrolled to.
func (n *notesWindow) stepFind(step int) {
	f := &n.find
	n.findAll()
	if len(f.matches) == 0 {
		return
	}
	f.match = (f.match + step + len(f.matches)) % len(f.matches)
	n.showMatch()
}

// showMatch scrolls to the match shown.
func (n *notesWindow) showMatch() {
	f := &n.find
	if n.cur == nil || f.match >= len(f.matches) {
		return
	}
	m := f.matches[f.match]
	t := n.cur.doc.Text()
	para := strings.Count(t.Slice(0, m[0]).String(), "\n")
	n.cur.doc.ScrollToParagraph(para, ui.ScrollCenter)
	n.spansVer++
	n.redraw()
}
