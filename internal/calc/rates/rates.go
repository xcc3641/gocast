// Package rates keeps Gocast's exchange rates: the latest snapshot from
// Frankfurter (https://frankfurter.dev, central bank reference rates),
// cached in a JSON file and fetched at most once a day while the
// calculator is on. The request asks for the latest rates and nothing
// else: nothing typed goes with it.
package rates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gocast/internal/calc"
)

// Endpoint is Frankfurter's v2 latest rates against USD: a JSON array of
// {"date","base","quote","rate"}, one per currency (fiat, metals and a
// few crypto; calc keeps the fiat ones).
const Endpoint = "https://api.frankfurter.dev/v2/rates?base=USD"

// FileName is the cache's name in the support folder.
const FileName = "rates.json"

const (
	// RetryInterval is how often to call MaybeRefresh while Gocast runs
	// (besides each time the panel shows): a failed fetch is tried again
	// then.
	RetryInterval = 6 * time.Hour
	// MinRetryGap keeps repeated panel shows from fetching again and
	// again while offline.
	MinRetryGap = time.Minute
	// Timeout bounds one fetch.
	Timeout   = 15 * time.Second
	maxBody   = 2 << 20
	minQuotes = 10
)

// Snapshot is the cached file.
type Snapshot struct {
	Source  string           `json:"source"`
	Base    string           `json:"base"`
	Date    string           `json:"date"`    // newest rate's date
	Fetched time.Time        `json:"fetched"` // when it was fetched
	Rates   map[string]Quote `json:"rates"`
}

// Quote is one currency's rate per Base, as decimal text.
type Quote struct {
	Rate string `json:"rate"`
	Date string `json:"date"`
}

// Store holds the rates and fetches them. Its methods are safe from any
// goroutine; MaybeRefresh and Refresh block on the network, so call them
// off the UI thread.
type Store struct {
	dir    string
	url    string
	client *http.Client

	mu          sync.Mutex
	enabled     bool
	snap        *Snapshot
	rates       *calc.Rates
	fetching    bool
	cancel      context.CancelFunc
	lastAttempt time.Time
	lastErr     error
	onChange    func()
}

// New makes a store caching in dir (Gocast's support folder; created on
// first save) and fetching with client (nil: a client with Timeout). It
// starts enabled.
func New(dir string, client *http.Client) *Store {
	if client == nil {
		client = &http.Client{Timeout: Timeout}
	}
	return &Store{dir: dir, url: Endpoint, client: client, enabled: true}
}

// SetURL points the store at another server (tests).
func (s *Store) SetURL(url string) {
	s.mu.Lock()
	s.url = url
	s.mu.Unlock()
}

// OnChange sets f to run (on the fetching goroutine) after new rates
// arrive, so an open card can redraw with them.
func (s *Store) OnChange(f func()) {
	s.mu.Lock()
	s.onChange = f
	s.mu.Unlock()
}

// Path is the cache file.
func (s *Store) Path() string { return filepath.Join(s.dir, FileName) }

// SetEnabled turns fetching on or off (Settings › General › Calculate in
// search). Off cancels a fetch in flight; the cache stays.
func (s *Store) SetEnabled(on bool) {
	s.mu.Lock()
	s.enabled = on
	if !on && s.cancel != nil {
		s.cancel()
	}
	s.mu.Unlock()
}

// Enabled reports whether the store fetches.
func (s *Store) Enabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enabled
}

// Load reads the cache. A missing file isn't an error; a broken one is
// ignored (the next fetch replaces it).
func (s *Store) Load() error {
	b, err := os.ReadFile(s.Path())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var snap Snapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		return fmt.Errorf("rates: reading %s: %w", s.Path(), err)
	}
	r, err := snap.calcRates()
	if err != nil {
		return fmt.Errorf("rates: reading %s: %w", s.Path(), err)
	}
	s.mu.Lock()
	if s.snap == nil || snap.Fetched.After(s.snap.Fetched) {
		s.snap, s.rates = &snap, r
	}
	s.mu.Unlock()
	return nil
}

func (snap *Snapshot) calcRates() (*calc.Rates, error) {
	qs := make([]calc.Quote, 0, len(snap.Rates))
	for code, q := range snap.Rates {
		qs = append(qs, calc.Quote{Code: code, Rate: q.Rate, Date: q.Date})
	}
	return calc.NewRates(snap.Base, qs)
}

// Rates are the rates for calc.Env; nil before the first fetch.
func (s *Store) Rates() *calc.Rates {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rates
}

// Date is the newest rate's date ("2026-11-13"), "" with no rates: the
// card's 汇率更新于.
func (s *Store) Date() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.snap == nil {
		return ""
	}
	return s.snap.Date
}

// Fetched is when the rates were fetched; zero with none.
func (s *Store) Fetched() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.snap == nil {
		return time.Time{}
	}
	return s.snap.Fetched
}

// Fetching reports a fetch in flight: with no rates yet, the card says
// it's getting them (正在获取汇率…).
func (s *Store) Fetching() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fetching
}

// LastAttempt is when the last fetch started (the card treats a first
// fetch running over 5 s as offline); zero before any.
func (s *Store) LastAttempt() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastAttempt
}

// LastError is the last fetch's error, nil after a success.
func (s *Store) LastError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastErr
}

// Due reports whether MaybeRefresh would fetch at now: enabled, nothing
// in flight, no rates fetched today (local date), and no attempt in the
// last MinRetryGap.
func (s *Store) Due(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dueLocked(now)
}

func (s *Store) dueLocked(now time.Time) bool {
	if !s.enabled || s.fetching {
		return false
	}
	if s.snap != nil && sameDay(s.snap.Fetched, now) {
		return false
	}
	if !s.lastAttempt.IsZero() && now.Sub(s.lastAttempt) < MinRetryGap && now.Sub(s.lastAttempt) >= 0 {
		return false
	}
	return true
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.In(b.Location()).Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// MaybeRefresh fetches when Due: call it when the panel shows and every
// RetryInterval. It reports whether it fetched; errors are for logs, the
// UI stays quiet (the cache keeps serving).
func (s *Store) MaybeRefresh(ctx context.Context, now time.Time) (bool, error) {
	s.mu.Lock()
	if !s.dueLocked(now) {
		s.mu.Unlock()
		return false, nil
	}
	ctx, done := s.startLocked(ctx, now)
	s.mu.Unlock()
	return true, s.fetch(ctx, now, done)
}

// Refresh fetches now, unless disabled or a fetch is in flight.
func (s *Store) Refresh(ctx context.Context, now time.Time) error {
	s.mu.Lock()
	if !s.enabled {
		s.mu.Unlock()
		return errDisabled
	}
	if s.fetching {
		s.mu.Unlock()
		return nil
	}
	ctx, done := s.startLocked(ctx, now)
	s.mu.Unlock()
	return s.fetch(ctx, now, done)
}

var errDisabled = errors.New("rates: calculator is off")

func (s *Store) startLocked(ctx context.Context, now time.Time) (context.Context, context.CancelFunc) {
	s.fetching = true
	s.lastAttempt = now
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	s.cancel = cancel
	return ctx, cancel
}

func (s *Store) fetch(ctx context.Context, now time.Time, cancel context.CancelFunc) (err error) {
	defer cancel()
	var snap *Snapshot
	var r *calc.Rates
	defer func() {
		s.mu.Lock()
		s.fetching, s.cancel, s.lastErr = false, nil, err
		var notify func()
		if err == nil && s.enabled {
			s.snap, s.rates = snap, r
			notify = s.onChange
		}
		s.mu.Unlock()
		if notify != nil {
			notify()
		}
	}()
	s.mu.Lock()
	url := s.url
	s.mu.Unlock()
	snap, err = download(ctx, s.client, url, now)
	if err != nil {
		return err
	}
	if r, err = snap.calcRates(); err != nil {
		return err
	}
	if !s.Enabled() {
		return errDisabled
	}
	return s.save(snap)
}

// download gets the latest rates from url.
func download(ctx context.Context, client *http.Client, url string, now time.Time) (*Snapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rates: %s", resp.Status)
	}
	var rows []struct {
		Date  string      `json:"date"`
		Base  string      `json:"base"`
		Quote string      `json:"quote"`
		Rate  json.Number `json:"rate"`
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, maxBody))
	dec.UseNumber()
	if err := dec.Decode(&rows); err != nil {
		return nil, fmt.Errorf("rates: %w", err)
	}
	snap := &Snapshot{Source: url, Base: "USD", Fetched: now, Rates: map[string]Quote{}}
	for _, row := range rows {
		if row.Base != "USD" || !calc.IsCurrency(row.Quote) || row.Quote == "USD" {
			continue
		}
		snap.Rates[row.Quote] = Quote{Rate: row.Rate.String(), Date: row.Date}
		if row.Date > snap.Date {
			snap.Date = row.Date
		}
	}
	if len(snap.Rates) < minQuotes {
		return nil, fmt.Errorf("rates: only %d rates", len(snap.Rates))
	}
	return snap, nil
}

// save writes snap to the cache atomically, readable by the user only.
func (s *Store) save(snap *Snapshot) error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(snap, "", "\t")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.dir, ".rates-*.json")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, s.Path())
}
