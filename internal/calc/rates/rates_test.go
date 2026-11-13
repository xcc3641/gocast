package rates

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gocast/internal/calc"
)

// body is a trimmed Frankfurter v2 /rates?base=USD response, with a
// metal and a crypto row calc leaves out.
const body = `[
{"date":"2026-11-13","base":"USD","quote":"JPY","rate":159.52},
{"date":"2026-11-13","base":"USD","quote":"CNY","rate":6.6635},
{"date":"2026-11-13","base":"USD","quote":"EUR","rate":0.90003},
{"date":"2026-11-12","base":"USD","quote":"GBP","rate":0.7544},
{"date":"2026-11-12","base":"USD","quote":"HKD","rate":7.8506},
{"date":"2026-11-12","base":"USD","quote":"TWD","rate":32.937},
{"date":"2026-11-12","base":"USD","quote":"KRW","rate":1334.71},
{"date":"2026-11-12","base":"USD","quote":"AUD","rate":1.4661},
{"date":"2026-11-12","base":"USD","quote":"CAD","rate":1.4249},
{"date":"2026-11-12","base":"USD","quote":"CHF","rate":0.83314},
{"date":"2026-11-12","base":"USD","quote":"SGD","rate":1.2859},
{"date":"2026-11-11","base":"USD","quote":"BTC","rate":1.3504e-05},
{"date":"2026-11-12","base":"USD","quote":"XAU","rate":0.00025}
]`

type server struct {
	*httptest.Server
	hits atomic.Int32
	fail atomic.Bool
}

func newServer(t *testing.T) *server {
	s := &server{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.hits.Add(1)
		if r.URL.Path != "/v2/rates" || r.URL.Query().Get("base") != "USD" || r.Body != nil && r.ContentLength > 0 {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if s.fail.Load() {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(s.Close)
	return s
}

func newStore(t *testing.T, srv *server, dir string) *Store {
	s := New(dir, srv.Client())
	s.SetURL(srv.URL + "/v2/rates?base=USD")
	return s
}

var day1 = time.Date(2026, 11, 13, 9, 0, 0, 0, time.Local)

func TestEndpoint(t *testing.T) {
	if !strings.HasPrefix(Endpoint, "https://api.frankfurter.dev/v2/rates") {
		t.Errorf("Endpoint = %q", Endpoint)
	}
}

func TestRefreshSavesAndLoads(t *testing.T) {
	srv := newServer(t)
	dir := filepath.Join(t.TempDir(), "dev.gocast.app")
	s := newStore(t, srv, dir)
	if s.Rates() != nil || s.Date() != "" {
		t.Fatal("rates before any fetch")
	}
	changed := 0
	s.OnChange(func() { changed++ })
	fetched, err := s.MaybeRefresh(context.Background(), day1)
	if !fetched || err != nil {
		t.Fatalf("MaybeRefresh = %v, %v", fetched, err)
	}
	if changed != 1 {
		t.Errorf("OnChange ran %d times", changed)
	}
	if s.Date() != "2026-11-13" {
		t.Errorf("Date = %q", s.Date())
	}
	r := s.Rates()
	if r == nil || !r.Has("TWD") || r.Has("BTC") || r.Has("XAU") {
		t.Fatalf("rates = %+v", r)
	}
	res, ok := calc.Eval("100 usd in jpy", calc.Env{Region: "CN", Rates: r})
	if !ok || calc.Plain.Answer(res) != "15,952 JPY" || res.RatesDate != "2026-11-13" {
		t.Errorf("100 usd in jpy = %q (%s)", calc.Plain.Answer(res), res.RatesDate)
	}

	// The file: 0600, valid JSON, no temp files left.
	st, err := os.Stat(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v", st.Mode().Perm())
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("dir has %d entries", len(entries))
	}
	var snap Snapshot
	b, _ := os.ReadFile(s.Path())
	if err := json.Unmarshal(b, &snap); err != nil || snap.Rates["JPY"].Rate != "159.52" || !snap.Fetched.Equal(day1) {
		t.Errorf("cache = %s (%v)", b, err)
	}

	// A new store (next launch) reads it, offline.
	srv.fail.Store(true)
	s2 := newStore(t, srv, dir)
	if err := s2.Load(); err != nil {
		t.Fatal(err)
	}
	if s2.Date() != "2026-11-13" || s2.Rates() == nil {
		t.Fatal("cache not loaded")
	}
	res, ok = calc.Eval("100 美元换人民币", calc.Env{Region: "JP", Rates: s2.Rates()})
	if !ok || calc.Plain.Answer(res) != "666.35 CNY" {
		t.Errorf("offline: %q", calc.Plain.Answer(res))
	}
}

func TestOncePerDay(t *testing.T) {
	srv := newServer(t)
	s := newStore(t, srv, t.TempDir())
	ctx := context.Background()
	if f, err := s.MaybeRefresh(ctx, day1); !f || err != nil {
		t.Fatal(f, err)
	}
	for _, at := range []time.Time{day1.Add(time.Minute), day1.Add(7 * time.Hour), time.Date(2026, 11, 13, 23, 59, 0, 0, time.Local)} {
		if f, _ := s.MaybeRefresh(ctx, at); f {
			t.Errorf("fetched again at %v", at)
		}
	}
	if f, err := s.MaybeRefresh(ctx, time.Date(2026, 11, 14, 0, 1, 0, 0, time.Local)); !f || err != nil {
		t.Errorf("next day: %v %v", f, err)
	}
	if n := srv.hits.Load(); n != 2 {
		t.Errorf("hits = %d", n)
	}
	// A loaded cache from today counts too.
	s2 := newStore(t, srv, s.dir)
	if err := s2.Load(); err != nil {
		t.Fatal(err)
	}
	if s2.Due(time.Date(2026, 11, 14, 8, 0, 0, 0, time.Local)) {
		t.Error("due with today's cache")
	}
}

func TestFailureRetries(t *testing.T) {
	srv := newServer(t)
	srv.fail.Store(true)
	s := newStore(t, srv, t.TempDir())
	ctx := context.Background()
	f, err := s.MaybeRefresh(ctx, day1)
	if !f || err == nil {
		t.Fatalf("MaybeRefresh = %v, %v", f, err)
	}
	if s.Rates() != nil || s.LastError() == nil || s.Fetching() {
		t.Error("state after a failure")
	}
	if !s.LastAttempt().Equal(day1) {
		t.Error("LastAttempt")
	}
	// Not again within MinRetryGap (panel shown twice quickly)…
	if f, _ := s.MaybeRefresh(ctx, day1.Add(10*time.Second)); f {
		t.Error("retried within MinRetryGap")
	}
	// …but on the next show after it, or the RetryInterval tick.
	srv.fail.Store(false)
	if f, err := s.MaybeRefresh(ctx, day1.Add(RetryInterval)); !f || err != nil {
		t.Errorf("retry: %v %v", f, err)
	}
	if s.Rates() == nil || s.LastError() != nil {
		t.Error("no rates after the retry")
	}
}

func TestFailureKeepsCache(t *testing.T) {
	srv := newServer(t)
	dir := t.TempDir()
	s := newStore(t, srv, dir)
	if err := s.Refresh(context.Background(), day1); err != nil {
		t.Fatal(err)
	}
	srv.fail.Store(true)
	if err := s.Refresh(context.Background(), day1.Add(48*time.Hour)); err == nil {
		t.Fatal("no error offline")
	}
	if s.Date() != "2026-11-13" || s.Rates() == nil {
		t.Error("failure dropped the rates")
	}
	b, _ := os.ReadFile(s.Path())
	if !strings.Contains(string(b), "159.52") {
		t.Error("failure touched the cache")
	}
}

func TestDisabled(t *testing.T) {
	srv := newServer(t)
	s := newStore(t, srv, t.TempDir())
	s.SetEnabled(false)
	if s.Due(day1) {
		t.Error("due while disabled")
	}
	if f, _ := s.MaybeRefresh(context.Background(), day1); f {
		t.Error("fetched while disabled")
	}
	if err := s.Refresh(context.Background(), day1); err == nil {
		t.Error("Refresh while disabled")
	}
	if n := srv.hits.Load(); n != 0 {
		t.Errorf("hits = %d", n)
	}
	s.SetEnabled(true)
	if !s.Due(day1) {
		t.Error("not due once enabled")
	}
}

func TestDisableCancelsFetch(t *testing.T) {
	started := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer srv.Close()
	s := New(t.TempDir(), srv.Client())
	s.SetURL(srv.URL)
	done := make(chan error)
	go func() {
		_, err := s.MaybeRefresh(context.Background(), day1)
		done <- err
	}()
	<-started
	if !s.Fetching() {
		t.Error("not Fetching during a fetch")
	}
	s.SetEnabled(false)
	select {
	case err := <-done:
		if err == nil {
			t.Error("cancelled fetch succeeded")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("disable didn't cancel the fetch")
	}
	if s.Fetching() || s.Rates() != nil {
		t.Error("state after cancel")
	}
}

func TestBadResponses(t *testing.T) {
	for name, resp := range map[string]string{
		"not json":   "<html>",
		"object":     `{"amount":1,"base":"USD","rates":{"JPY":159}}`,
		"too few":    `[{"date":"2026-11-13","base":"USD","quote":"JPY","rate":159.52}]`,
		"wrong base": strings.ReplaceAll(body, `"base":"USD"`, `"base":"EUR"`),
		"negative":   strings.ReplaceAll(body, `"rate":`, `"rate":-`),
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(resp))
			}))
			defer srv.Close()
			s := New(t.TempDir(), srv.Client())
			s.SetURL(srv.URL)
			if err := s.Refresh(context.Background(), day1); err == nil {
				t.Error("no error")
			}
			if s.Rates() != nil {
				t.Error("kept bad rates")
			}
			if _, err := os.Stat(s.Path()); err == nil {
				t.Error("wrote a cache")
			}
		})
	}
}

func TestLoadBroken(t *testing.T) {
	dir := t.TempDir()
	s := New(dir, nil)
	if err := s.Load(); err != nil {
		t.Errorf("missing file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.Load(); err == nil {
		t.Error("broken file loaded")
	}
	if s.Rates() != nil || !s.Due(day1) {
		t.Error("broken cache counts")
	}
}
