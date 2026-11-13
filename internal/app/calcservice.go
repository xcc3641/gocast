package app

// calcservice.go runs the calculator for the app (design.md 2.3a): the
// exchange rates (internal/calc/rates), cached in rates.json next to
// settings.json and fetched at most once a day while the calculator is
// on, as the panel shows and every 6 hours, on goroutines; the system's
// region, which reads ¥ and picks the currency to convert into; and the
// system's number format, which answers are written in.

import (
	"context"
	"log"
	"time"

	"gocast/internal/calc"
	"gocast/internal/calc/rates"
	"gocast/internal/calc/region"
	"gocast/internal/sysfmt"
)

// firstFetchFor is how long the card says it is getting the rates, with
// none yet, before it takes Gocast as offline (design.md 2.3a).
const firstFetchFor = 5 * time.Second

// calcService is the calculator of the app. Its fields are read and
// changed on the main thread; the store is safe from any goroutine.
type calcService struct {
	g     *gocast
	store *rates.Store

	// region is the system's region ("JP"), and group and decimal its
	// number format's separators, as the panel last showed.
	region, group, decimal string

	// fetching: a refresh runs, since fetchStart.
	fetching   bool
	fetchStart time.Time
	ticker     *time.Ticker
}

func newCalcService(g *gocast) *calcService {
	s := &calcService{g: g, store: rates.New(g.dir, nil), group: ",", decimal: "."}
	s.store.SetEnabled(g.settings.Calculator())
	return s
}

// start reads the cached rates on a goroutine, then fetches them if they
// are due, and every rates.RetryInterval from then on.
func (s *calcService) start() {
	s.store.OnChange(func() { s.g.launcher.win.Update(s.g.launcher.calcChanged) })
	go func() {
		if err := s.store.Load(); err != nil {
			log.Print(err)
		}
		s.g.launcher.win.Update(func() {
			s.g.launcher.calcChanged()
			s.refresh()
		})
	}()
	s.ticker = time.NewTicker(rates.RetryInterval)
	go func() {
		for range s.ticker.C {
			s.g.launcher.win.Update(s.refresh)
		}
	}()
}

// on reports whether the search panel calculates.
func (s *calcService) on() bool { return s.g.settings.Calculator() }

// setOn turns the calculator on or off, and saves it: off, no card and
// no fetching (a fetch under way stops); the rates kept stay.
func (s *calcService) setOn(on bool) {
	s.g.settings.CalculatorOff = !on
	s.g.saveSettings()
	s.store.SetEnabled(on)
	if on {
		s.refresh()
	}
	s.g.launcher.calcChanged()
}

// shown takes the system's region and number format as the panel shows,
// and fetches the rates if they are due.
func (s *calcService) shown() {
	s.region = region.Code()
	s.group, s.decimal = sysfmt.Separators()
	s.refresh()
}

// refresh fetches the rates on a goroutine if they are due; the card
// draws again as it ends, with the rates or, with none, offline.
func (s *calcService) refresh() {
	now := time.Now()
	if s.fetching || !s.on() || !s.store.Due(now) {
		return
	}
	s.fetching, s.fetchStart = true, now
	l := s.g.launcher
	go func() {
		if _, err := s.store.MaybeRefresh(context.Background(), time.Now()); err != nil {
			log.Printf("rates: %v", err)
		}
		l.win.Update(func() {
			s.fetching = false
			l.calcChanged()
		})
	}()
	// Past firstFetchFor, the card says it needs to go online.
	time.AfterFunc(firstFetchFor+50*time.Millisecond, func() { l.win.Update(l.calcChanged) })
}

// gettingFirst reports whether the card says it is getting the rates at
// now: none yet, fetching for less than firstFetchFor.
func (s *calcService) gettingFirst(now time.Time) bool {
	if s.store.Rates() != nil {
		return false
	}
	switch {
	case s.fetching:
		return now.Sub(s.fetchStart) < firstFetchFor
	case s.store.Fetching():
		return now.Sub(s.store.LastAttempt()) < firstFetchFor
	}
	return false
}

// env is what the calculator evaluates with.
func (s *calcService) env() calc.Env {
	return calc.Env{Region: s.region, Rates: s.store.Rates()}
}

// formatter writes answers in the system's number format, durations in
// the language Gocast shows (copy.md 7.1).
func (s *calcService) formatter() calc.Formatter {
	return calc.Formatter{Group: s.group, Decimal: s.decimal, Symbol: calcSymbol}
}
