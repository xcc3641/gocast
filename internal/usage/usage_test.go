package usage

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScoreDecays(t *testing.T) {
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	s := New()
	s.Record("app:a", "", now)
	if got := s.Score("app:a", now); got != OpenWeight {
		t.Fatalf("score now = %v, want %v", got, OpenWeight)
	}
	if got := s.Score("app:a", now.Add(HalfLife)); math.Abs(got-OpenWeight/2) > 1e-9 {
		t.Fatalf("score after a half-life = %v, want %v", got, OpenWeight/2)
	}
	s.Record("app:a", "", now.Add(HalfLife))
	if got := s.Score("app:a", now.Add(HalfLife)); math.Abs(got-150) > 1e-9 {
		t.Fatalf("score after a second opening = %v, want 150", got)
	}
}

func TestQueryChoice(t *testing.T) {
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	s := New()
	s.Record("app:safari", "  Saf ", now)
	if got := s.Choice("saf", now.Add(time.Hour)); got != "app:safari" {
		t.Fatalf("choice = %q", got)
	}
	if got := s.Choice("saf", now.Add(QueryTTL+time.Hour)); got != "" {
		t.Fatalf("choice after the TTL = %q, want none", got)
	}
	if got := s.Choice("", now); got != "" {
		t.Fatalf("empty query chose %q", got)
	}
}

func TestLoadOldCounts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "usage.json")
	if err := os.WriteFile(path, []byte(`{"app:x": 2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	s, err := Load(path, now)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Score("app:x", now); got != 2*OpenWeight {
		t.Fatalf("migrated score = %v", got)
	}
	s.Record("app:y", "y", now)
	if err := s.Save(path, now); err != nil {
		t.Fatal(err)
	}
	s2, err := Load(path, now)
	if err != nil {
		t.Fatal(err)
	}
	if s2.Choice("y", now) != "app:y" || s2.Score("app:x", now) != 2*OpenWeight {
		t.Fatalf("round trip lost data: %+v", s2)
	}
}
