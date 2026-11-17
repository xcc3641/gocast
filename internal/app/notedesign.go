package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"

	"gocast/internal/notes"
)

// noteDesign is the editor's typography (design.md 11.4) at a zoom (1 for
// 100%), in the colors of an appearance: what richtext's Theme and Font
// say, and the rest through Spans and Space.
type noteDesign struct {
	zoom   float32
	k      tokens
	dark   bool
	accent ui.Color
}

// The 11.4 sizes at 100%, in DIPs.
const (
	noteBodySize, noteBodyLine = 15, 22
	noteCodeSize               = 13
	noteParaGap                = 8  // between paragraphs of text
	noteListGap                = 4  // between the items of a list
	noteCodeGap                = 12 // around a block of code, outside its background
	noteRuleGap                = 16 // from a rule's hairline to the text around it
)

func (d noteDesign) z() float32 {
	if d.zoom == 0 {
		return 1
	}
	return d.zoom
}

// em turns DIPs at 100% into ems of the text.
func em(dips float32) float32 { return dips / noteBodySize }

// link is the system's link color (design.md 4.5).
func (d noteDesign) link() ui.Color {
	if d.dark {
		return ui.Hex("#419CFF")
	}
	return ui.Hex("#0068DA")
}

// quoteBar is the bar of a quotation.
func (d noteDesign) quoteBar() ui.Color {
	if d.dark {
		return ui.RGBA(255, 255, 255, 0.22)
	}
	return ui.RGBA(0, 0, 0, 0.15)
}

// font is the text's: 15 on lines of 22 at 100%.
func (d noteDesign) font() ui.Font {
	return ui.Font{Size: noteBodySize * d.z(), LineHeight: float32(noteBodyLine) / noteBodySize}
}

// theme is all of 11.4 richtext's Theme can say. Its lengths are in ems,
// which the zoom scales; QuoteBarWidth is in DIPs, scaled here.
func (d noteDesign) theme() *richtext.Theme {
	k := d.k
	th := &richtext.Theme{
		Text: k.text, Muted: k.textOnGlass, Link: d.link(),
		Caret: d.accent, Selection: d.accent.Alpha(0.28),
		CodeSize:            float32(noteCodeSize) / noteBodySize,
		Code:                k.text,
		CodeBackground:      k.keycap,
		CodeBlockBackground: k.keycap,
		CodeBlockRadius:     em(8),
		CodeBlockPadding:    [2]float32{em(12), em(10)},
		ParagraphSpacing:    em(noteParaGap),
		Indent:              em(20),
		Marker:              k.textOnGlass,
		Checkbox:            d.accent,
		Checkmark:           ui.RGB(255, 255, 255),
		QuoteBar:            d.quoteBar(),
		QuoteText:           k.textOnGlass,
		QuoteBarWidth:       3 * d.z(),
	}
	// Top in ems of the heading, Bottom of the text, LineHeight of the
	// heading: 24 on 30, 20 / 8; 19 on 26, 16 / 6; 16 on 22, 12 / 4.
	th.Headings[0] = richtext.HeadingStyle{Size: 24.0 / noteBodySize, Weight: 700, Top: 20.0 / 24, Bottom: em(8), LineHeight: 30.0 / 24}
	th.Headings[1] = richtext.HeadingStyle{Size: 19.0 / noteBodySize, Weight: 600, Top: 16.0 / 19, Bottom: em(6), LineHeight: 26.0 / 19}
	th.Headings[2] = richtext.HeadingStyle{Size: 16.0 / noteBodySize, Weight: 600, Top: 12.0 / 16, Bottom: em(4), LineHeight: 22.0 / 16}
	return th
}

// blockSize is the size of the text of a paragraph of style st at 100%.
func blockSize(st richtext.ParagraphStyle) float32 {
	switch {
	case st.Heading == 1:
		return 24
	case st.Heading == 2:
		return 19
	case st.Heading == 3:
		return 16
	case st.CodeBlock:
		return noteCodeSize
	}
	return noteBodySize
}

// spans restyles what the theme can't (design.md 11.4): inline code 1.5
// smaller than its block, 4 round and 3 wider at each side;
// strikethrough 1 DIP in textSecondary, its text too; links underlined
// never; a task ticked in textSecondary without the strikethrough
// richtext gives it; bold in a heading as heavy as the heading; a rule
// as high as the x-height. runAt gives the attributes of the text at a
// rune of the paragraph, which starts at start.
func (d noteDesign) spans(st richtext.ParagraphStyle, spans []ui.Span, start int, runAt func(i int) richtext.Attributes) []ui.Span {
	k, z := d.k, d.z()
	at := start
	for i := range spans {
		s := &spans[i]
		a := runAt(at)
		at += len([]rune(s.Text))
		if a.Code && !st.CodeBlock {
			s.Size = (blockSize(st) - 1.5) * z
			s.Radius = 4 * z
			s.PaddingX = 3 * z
		}
		if a.Link != "" {
			s.Underline = false
		}
		if st.List == richtext.Tasks && st.Checked {
			s.Strikethrough = a.Strikethrough
			if a.Link == "" {
				s.Color = k.textSecondary
			}
		}
		if a.Strikethrough {
			s.DecorationThickness = 1
			s.DecorationColor = k.textSecondary
			s.Color = k.textSecondary
		}
		if notes.IsRule(a.Attachment) {
			// Full width (richtext scaled it down to the line), and no
			// taller than the hairline's room in the middle of its line.
			s.ObjectHeight, s.ObjectDescent = ruleObject*z, 0
		}
		if st.Heading > 0 && a.Bold() {
			s.Weight = 0
		}
	}
	return spans
}

// ruleObject is the height of a rule's attachment at 100%: its hairline
// is drawn across its middle.
const ruleObject = 10

// ruleAbove and ruleBelow are what Space adds to the room the theme
// leaves (ParagraphSpacing) above a rule and below it, at 100%, for
// noteRuleGap from its hairline to the lines of text around it: the
// rule's line is a line of text, noteBodyLine high, its attachment
// standing on the baseline, its hairline 12.4 below the line's top, 9.6
// above its bottom (TestNotesTypography measures them).
const (
	ruleAbove = noteRuleGap - 12.4 - noteParaGap
	ruleBelow = noteRuleGap - 9.6 - noteParaGap
)

// space is the room in DIPs Space adds to the theme's above paragraph p,
// given the paragraphs' styles and which are rules: 4 between the items
// of a list, 8 around a list, 12 around a block of code, outside its
// background, and 16 from a rule's hairline to the text around it.
func (d noteDesign) space(styles []richtext.ParagraphStyle, rules []bool, p int) float32 {
	if p <= 0 || p >= len(styles) {
		return 0
	}
	st, prev := styles[p], styles[p-1]
	list, prevList := st.List != richtext.NoList, prev.List != richtext.NoList
	var room float32
	switch {
	case rules[p]:
		room = ruleAbove
	case rules[p-1]:
		room = ruleBelow
	case st.CodeBlock && !prev.CodeBlock, prev.CodeBlock && !st.CodeBlock:
		room = noteCodeGap
	case list && prevList:
		room = noteListGap
	case list != prevList && st.Heading == 0 && prev.Heading == 0:
		room = noteParaGap
	}
	return room * d.z()
}
