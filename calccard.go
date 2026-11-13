package main

// calccard.go is the calculator's card (design.md 2.3a, copy.md 7): the
// first row of the search panel while the query is a calculation, the
// expression at its left and the answer at its right, and what ↵, ⌘↵
// and ⌘K do with it.

import (
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/ui"

	"gocast/internal/calc"
	"gocast/internal/sysfmt"
	"gocast/locales"
)

// The card's measures (design.md 2.3a), in DIPs.
const (
	cardWidth  = panelWidth - 2*listInset // 624, the list's
	cardHeight = 96
	cardGap    = 4 // under it, to the first result
	cardRadius = 10
	cardHalf   = (cardWidth - 1) / 2 // each side of the line, 311.5
	cardPad    = 16                  // in each side, left and right
	cardTextW  = cardHalf - 2*cardPad
	cardMainH  = 30 // the main line, at both sides
	cardLine   = 14 // the line's inset, top and bottom
	cardBreak  = 32 // the line's break, around the arrow
	cardNoteH  = 20
)

// cardState is what the card's right side shows.
type cardState int

const (
	cardAnswer       cardState = iota // the answer
	cardStale                         // being typed: the last answer, greyed
	cardCant                          // 算不出来
	cardNeedsRates                    // no rates and offline
	cardGettingRates                  // no rates, the first fetch under way
)

// calcCard is what the card shows, made of an evaluation.
type calcCard struct {
	state  cardState
	expr   string // the left side: the input as typed, × and ÷ for * and /
	answer string // the right side's answer, with cardAnswer and cardStale
	// from and to are the notes under each side: full names, the rates'
	// day after the answer's currency; "" for none.
	from, to string
	label    string // what assistive technology reads (copy.md 7.4)
	// equation and number are what the actions copy besides the answer:
	// "100 USD = 15,432 JPY", "15432.12" (copy.md 7.2).
	equation, number string
}

// ok reports whether the card has an answer to act on: none while it
// can't calculate, has no rates, or the expression is being typed (the
// answer greyed is not this expression's).
func (c calcCard) ok() bool { return c.state == cardAnswer }

// makeCalcCard makes the card of res. last is the last evaluation not
// being typed, which shows while res is (Status Incomplete; nil: no card
// then, see launcher.evalCalc). getting: the first fetch of the rates is
// under way, less than firstFetchFor. now dates the rates.
func makeCalcCard(res calc.Result, last *calc.Result, f calc.Formatter, getting bool, now time.Time) calcCard {
	input := res.Input
	card := calcCard{expr: calc.Display(input)}
	shown := res
	if res.Status == calc.Incomplete && last != nil {
		shown = *last
	}
	spoken := spokenExpr(res, f)
	if shown.Kind != calc.KindNumber && shown.From != "" {
		card.from = calcName(shown.From)
	}
	switch shown.Status {
	case calc.OK:
		card.answer = f.Answer(shown)
		if shown.Kind != calc.KindNumber && shown.Unit != "" {
			card.to = calcName(shown.Unit)
		}
		label := locales.CalcLabel(spoken, spokenAnswer(shown, f))
		if shown.RatesDate != "" {
			card.to = locales.CalcRatesFrom(card.to, ratesDay(shown.RatesDate, now, false))
			label = locales.CalcLabelRates(label, ratesDay(shown.RatesDate, now, true))
		}
		card.label = label
		card.equation, card.number = f.Equation(shown), shown.Raw()
		if res.Status == calc.Incomplete {
			card.state = cardStale
			card.label = locales.CalcLabelIncomplete(spoken, spokenAnswer(shown, f))
			card.equation, card.number = "", ""
		}
	case calc.NeedsRates:
		card.to = calcName(shown.Unit)
		card.state, card.label = cardNeedsRates, locales.CalcLabelNeedsRates(spoken)
		if getting {
			card.state, card.label = cardGettingRates, locales.CalcLabelGettingRates(spoken)
		}
	default: // Error, or an Incomplete with nothing before it
		card.state, card.label = cardCant, locales.CalcLabelCantCalculate(spoken)
		card.from = ""
	}
	return card
}

// calcName is the full name of a unit or currency code ("USD" → "US
// dollars"), the code itself for one without.
func calcName(code string) string {
	if n, ok := calcUnitNames[code]; ok {
		return n()
	}
	if n, ok := calcCurrencyNames[code]; ok {
		return n()
	}
	return code
}

// calcSymbol writes a unit's symbol in an answer (copy.md 7.1): the units
// of time in the language Gocast shows ("2 小时 25 分", "2 h 25 min"),
// the others as calc writes them ("°F", "m²", "JPY").
func calcSymbol(code string) string {
	switch code {
	case "day":
		return locales.CalcSymbolDay()
	case "h":
		return locales.CalcSymbolHour()
	case "min":
		return locales.CalcSymbolMinute()
	case "s":
		return locales.CalcSymbolSecond()
	case "week":
		return locales.CalcSymbolWeek()
	case "ms":
		return locales.CalcSymbolMillisecond()
	}
	return calc.DefaultSymbol(code)
}

// ratesDay says the day of the rates ("2026-11-13") as of now, in the
// system's relative words: 今天, 昨天, 11月10日; long spells the month,
// for VoiceOver.
func ratesDay(date string, now time.Time, long bool) string {
	t, err := time.ParseInLocation("2006-01-02", date, now.Location())
	if err != nil {
		return date
	}
	return sysfmt.Day(t, now, i18n.Locale(), long)
}

// spokenExpr is the expression as VoiceOver reads it (copy.md 7.4): a
// single quantity by its unit's or currency's full name ("100 US
// dollars"), else the input with its operators in words ("12 times 3").
func spokenExpr(res calc.Result, f calc.Formatter) string {
	if res.Amount != nil && res.From != "" {
		return f.Number(calc.Round(res.Amount, -1)) + " " + calcName(res.From)
	}
	return speakOps(calc.Display(res.Input))
}

// spokenAnswer is the answer as VoiceOver reads it: a quantity by its
// unit's or currency's full name ("15,432 Japanese yen").
func spokenAnswer(r calc.Result, f calc.Formatter) string {
	if (r.Kind == calc.KindUnit || r.Kind == calc.KindCurrency) && r.Unit != "" {
		return f.Number(r.Number()) + " " + calcName(r.Unit)
	}
	return f.Answer(r)
}

var (
	operand    = `\([^()]*\)|[^\s×÷+\-*/^%()]+`
	powerRE    = regexp.MustCompile(`(` + operand + `)\s*(?:\^|\*\*)\s*(-?(?:` + operand + `))`)
	percentRE  = regexp.MustCompile(`([0-9][0-9.,]*)\s*%`)
	spacesRE   = regexp.MustCompile(`\s+`)
	opReplacer = strings.NewReplacer("×", " \x00times ", "÷", " \x00div ")
)

// speakOps writes an expression's operators in words (copy.md 7.4): ×
// 乘, ÷ 除以, ^ 的…次方, % 百分之.
func speakOps(s string) string {
	s = powerRE.ReplaceAllStringFunc(s, func(m string) string {
		p := powerRE.FindStringSubmatch(m)
		return locales.CalcSpokenPower(p[1], p[2])
	})
	s = percentRE.ReplaceAllStringFunc(s, func(m string) string {
		return locales.CalcSpokenPercent(percentRE.FindStringSubmatch(m)[1])
	})
	s = opReplacer.Replace(s)
	s = strings.NewReplacer("\x00times", locales.CalcSpokenTimes(), "\x00div", locales.CalcSpokenDividedBy()).Replace(s)
	return strings.TrimSpace(spacesRE.ReplaceAllString(s, " "))
}

// --- The launcher's card ---

// evalCalc evaluates the query for the card: it shows while the query is
// a calculation, the calculator on, except one being typed with nothing
// calculated before it (design.md 2.3a: 2+ alone shows no card).
func (l *launcher) evalCalc() {
	l.calcOn = false
	s := l.g.calc
	if s == nil || !s.on() || l.empty() {
		l.calcLast = nil
		return
	}
	res, ok := calc.Eval(l.query, s.env())
	if !ok {
		l.calcLast = nil
		return
	}
	if res.Status == calc.Incomplete {
		if l.calcLast == nil {
			return
		}
	} else {
		last := res
		l.calcLast = &last
	}
	l.calcRes, l.calcOn = res, true
}

// calcChanged evaluates the query again as the rates, the calculator's
// switch or the region changed, and draws the card again: with the rates
// once they come, as the panel shows (design.md 2.3a).
func (l *launcher) calcChanged() {
	if l.shown && !l.clip.open && !l.compact() {
		was := l.calcOn
		l.evalCalc()
		if was != l.calcOn {
			l.rebuild()
		}
	}
	if l.win != nil {
		l.win.Invalidate()
	}
}

// card is the card of the query, as of now.
func (l *launcher) card(now time.Time) calcCard {
	s := l.g.calc
	return makeCalcCard(l.calcRes, l.calcLast, s.formatter(), s.gettingFirst(now), now)
}

// calcRow draws the card: a row 96 high and 4 under it, in the selection
// gray when chosen (the list draws no selection of its own, see
// resultList).
func (l *launcher) calcRow(c *ui.Context, k tokens, chosen bool) {
	card := l.card(time.Now())
	bg := k.cardFill
	if chosen {
		bg = k.selection
	}
	ui.Column(c).Height(cardHeight+cardGap).Shrink(0).Padding(0, 0, cardGap, 0).Children(func() {
		ui.Row(c).Height(cardHeight).Shrink(0).Radius(cardRadius).Background(bg).AccessHidden(true).Children(func() {
			noted := card.from != "" || card.to != ""
			cardSide(c, k, noted, card.from, func() { cardExpr(c, k, card.expr) })
			// The line, broken around the arrow.
			const piece = (cardHeight - 2*cardLine - cardBreak) / 2
			ui.Column(c).Width(1).Height(cardHeight).Shrink(0).Padding(cardLine, 0).Children(func() {
				ui.Box(c).Width(1).Height(piece).Shrink(0).Background(k.hairline)
				ui.Box(c).Height(cardBreak).Shrink(0)
				ui.Box(c).Width(1).Height(piece).Shrink(0).Background(k.hairline)
			})
			cardSide(c, k, noted, card.to, func() { cardAnswerLine(c, k, card) })
			ui.Box(c).Absolute().Left(cardWidth/2-cardBreak/2).Top(cardHeight/2-cardBreak/2).Size(cardBreak, cardBreak).
				Center().Children(func() {
				sfsymbols.Symbol(c, "arrow.right").Size(14).Weight(sfsymbols.Semibold).Color(k.textOnGlass)
			})
		})
	})
}

// cardSide is a side of the card: its main line, 30 high, and its note
// under it, 8 apart, centred; with no notes on either side, the main
// line alone, centred (design.md 2.3a).
func cardSide(c *ui.Context, k tokens, noted bool, note string, main func()) {
	ui.Column(c).Width(cardHalf).Shrink(0).Padding(0, cardPad).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Height(cardMainH).MaxWidth(cardTextW).Shrink(0).AlignItems(ui.Center).Justify(ui.Center).Children(main)
		if !noted {
			return
		}
		ui.Box(c).Height(cardNoteH).Shrink(0).Margin(8, 0, 0, 0).Children(func() {
			if note == "" {
				return
			}
			ui.Row(c).Height(cardNoteH).MaxWidth(cardTextW).Radius(6).Padding(0, 8).AlignItems(ui.Center).
				Background(k.keycap).Children(func() {
				ui.Text(c, note).FontSize(12).TextColor(k.textOnGlass).SingleLine().Shrink(1)
			})
		})
	})
}

// cardSizes are the main lines' sizes, the next taken when the line
// doesn't fit (design.md 2.3a).
var cardSizes = []float32{0, 17, 15} // 0: the side's own, 20 or 24

// cardFit returns the size of s, which starts at first, for it to fit
// the side, or 15 when nothing does.
func cardFit(c *ui.Context, s string, first float32, weight int) (size float32, fits bool) {
	for _, size := range cardSizes {
		if size == 0 {
			size = first
		}
		w, _ := c.MeasureText(0, ui.Span{Text: s, Size: size, Weight: weight})
		if w+2 <= cardTextW { // tabular digits are a little wider
			return size, true
		}
	}
	return cardSizes[len(cardSizes)-1], false
}

// cardExpr is the left side's main line: the expression, 20 Regular,
// cut at its start when even 15 does not fit, keeping what was typed
// last.
func cardExpr(c *ui.Context, k tokens, expr string) {
	size, fits := cardFit(c, expr, 20, 400)
	if !fits {
		expr = fitHead(expr, cardTextW-2, func(s string) float32 {
			w, _ := c.MeasureText(0, ui.Span{Text: s, Size: size})
			return w
		})
	}
	ui.Text(c, expr).FontSize(size).TextColor(k.text).FontFeatures("tnum").SingleLine().Shrink(1)
}

// cardAnswerLine is the right side's main line: the answer, 24 Semibold, in
// the secondary gray while being typed; or why there is none.
func cardAnswerLine(c *ui.Context, k tokens, card calcCard) {
	switch card.state {
	case cardAnswer, cardStale:
		color := k.text
		if card.state == cardStale {
			color = k.textOnGlass
		}
		size, _ := cardFit(c, card.answer, 24, 600)
		ui.Text(c, card.answer).FontSize(size).FontWeight(600).FontFeatures("tnum").TextColor(color).SingleLine().Shrink(0)
	case cardCant:
		ui.Text(c, locales.CalcCantCalculate()).FontSize(15).TextColor(k.textOnGlass).SingleLine().Shrink(1)
	case cardNeedsRates, cardGettingRates:
		text := locales.CalcNeedsRates()
		ui.Row(c).Gap(6).AlignItems(ui.Center).Shrink(1).Children(func() {
			if card.state == cardGettingRates {
				text = locales.CalcGettingRates()
				ui.Spinner(c).Size(14, 14).Shrink(0)
			} else {
				sfsymbols.Symbol(c, "wifi.slash").Size(14).Color(k.textOnGlass)
			}
			ui.Text(c, text).FontSize(13).TextColor(k.textOnGlass).SingleLine().Shrink(1)
		})
	}
}

// fitHead cuts s at its start, with an ellipsis, to at most max wide as
// measure measures it: "…+ 125 × 3".
func fitHead(s string, max float32, measure func(string) float32) string {
	if measure(s) <= max {
		return s
	}
	runes := []rune(s)
	lo, hi := 0, len(runes) // the most runes at the end that fit, with "…"
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if measure("…"+string(runes[len(runes)-mid:])) <= max {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return "…" + strings.TrimLeft(string(runes[len(runes)-lo:]), " ")
}

// --- Acting on the card ---

// calcActions are the card's ⌘K rows (design.md 2.3a, copy.md 7.2), all
// dimmed while it has no answer (launcher.menuDim).
func (l *launcher) calcActions() []action {
	return []action{
		{"doc.on.doc", locales.CalcCopyAnswer(), []string{"↩"}, l.primary},
		{"doc.on.clipboard", l.target.label(l.target.app.Name), []string{"⌘", "↩"}, l.secondary},
		{"equal.square", locales.CalcCopyEquation(), nil, func() { l.calcCopy(func(c calcCard) string { return c.equation }) }},
		{"number", locales.CalcCopyNumber(), nil, func() { l.calcCopy(func(c calcCard) string { return c.number }) }},
	}
}

// calcAct is the card's ↵ (copy the answer) and ⌘↵ (paste it).
func (l *launcher) calcAct(second bool) {
	if second {
		l.calcPaste()
		return
	}
	l.calcCopy(func(c calcCard) string { return c.answer })
}

// calcCopy puts what of the card on the pasteboard, and hides the panel:
// no notice, hiding says it (design.md 2.3a).
func (l *launcher) calcCopy(what func(calcCard) string) {
	card := l.card(time.Now())
	if !card.ok() {
		return
	}
	if err := l.g.clip.copyText(what(card)); err != nil {
		log.Printf("calculator: copying: %v", err)
		return
	}
	l.hide()
}

// calcPaste pastes the answer into the app the panel showed over, as the
// clipboard history pastes (design.md 2.8.6): without the Accessibility
// permission, it copies it, and the panel stays and says to press ⌘V.
func (l *launcher) calcPaste() {
	card := l.card(time.Now())
	if !card.ok() {
		return
	}
	s := l.g.clip
	if !l.target.allowed(s.trusted()) {
		if err := s.copyText(card.answer); err != nil {
			log.Printf("calculator: copying: %v", err)
			return
		}
		l.say(locales.ClipCopiedPressCmdV(), false)
		return
	}
	if err := s.pasteText(card.answer, l.target); err != nil {
		log.Printf("calculator: pasting: %v", err)
		return
	}
	l.hide()
}
