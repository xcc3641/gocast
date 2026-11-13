// Package usage remembers what the user opens from the search panel, as
// Raycast does: how often and how lately each item was opened (frecency:
// every opening adds 100, halving every 10 days), and which item each
// query led to (kept 17 days), so that the same query finds it first.
package usage

import (
	"cmp"
	"encoding/json"
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	// HalfLife is how long an opening takes to count half as much.
	HalfLife = 10 * 24 * time.Hour
	// OpenWeight is what an opening adds to an item's score.
	OpenWeight = 100
	// QueryTTL is how long a query keeps leading to the item it opened.
	QueryTTL = 17 * 24 * time.Hour
)

// Item is the score of an item as of At.
type Item struct {
	Score float64   `json:"score"`
	At    time.Time `json:"at"`
}

// Choice is the item a query opened, and when.
type Choice struct {
	ID string    `json:"id"`
	At time.Time `json:"at"`
}

// Store is what usage.json keeps.
type Store struct {
	Items   map[string]Item   `json:"items"`
	Queries map[string]Choice `json:"queries"`
}

// New returns an empty store.
func New() *Store {
	return &Store{Items: map[string]Item{}, Queries: map[string]Choice{}}
}

// Load reads the store at path: an empty one where there is none. The
// counts of earlier versions ({"id": n}) become scores of n openings now.
func Load(path string, now time.Time) (*Store, error) {
	s := New()
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return s, err
	}
	if _, ok := probe["items"]; ok {
		if err := json.Unmarshal(data, s); err != nil {
			return New(), err
		}
		if s.Items == nil {
			s.Items = map[string]Item{}
		}
		if s.Queries == nil {
			s.Queries = map[string]Choice{}
		}
		return s, nil
	}
	var counts map[string]int
	if err := json.Unmarshal(data, &counts); err != nil {
		return s, err
	}
	for id, n := range counts {
		s.Items[id] = Item{Score: float64(n * OpenWeight), At: now}
	}
	return s, nil
}

// Save writes the store to path, dropping what has worn off.
func (s *Store) Save(path string, now time.Time) error {
	s.prune(now)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *Store) prune(now time.Time) {
	for id := range s.Items {
		if s.Score(id, now) < 0.5 {
			delete(s.Items, id)
		}
	}
	for q, c := range s.Queries {
		if now.Sub(c.At) > QueryTTL {
			delete(s.Queries, q)
		}
	}
}

// Score is the item's frecency now: its openings, each counting less as it
// ages.
func (s *Store) Score(id string, now time.Time) float64 {
	it, ok := s.Items[id]
	if !ok {
		return 0
	}
	return decay(it.Score, now.Sub(it.At))
}

func decay(score float64, age time.Duration) float64 {
	if age <= 0 {
		return score
	}
	return score * math.Pow(0.5, float64(age)/float64(HalfLife))
}

// Ranked lists the items with a score now, the highest first, and of
// equal scores the one opened last first (design.md 2.5a).
func (s *Store) Ranked(now time.Time) []string {
	type scored struct {
		id    string
		score float64
		at    time.Time
	}
	var list []scored
	for id, it := range s.Items {
		if sc := s.Score(id, now); sc > 0 {
			list = append(list, scored{id, sc, it.At})
		}
	}
	slices.SortFunc(list, func(a, b scored) int {
		return cmp.Or(cmp.Compare(b.score, a.score), b.at.Compare(a.at), strings.Compare(a.id, b.id))
	})
	ids := make([]string, len(list))
	for i, it := range list {
		ids[i] = it.id
	}
	return ids
}

// Record notes that query (maybe empty) opened the item id now.
func (s *Store) Record(id, query string, now time.Time) {
	s.Items[id] = Item{Score: s.Score(id, now) + OpenWeight, At: now}
	if q := NormalizeQuery(query); q != "" {
		s.Queries[q] = Choice{ID: id, At: now}
	}
}

// Choice returns the item query last opened within QueryTTL, or "".
func (s *Store) Choice(query string, now time.Time) string {
	q := NormalizeQuery(query)
	if q == "" {
		return ""
	}
	c, ok := s.Queries[q]
	if !ok || now.Sub(c.At) > QueryTTL {
		return ""
	}
	return c.ID
}

// NormalizeQuery is query as the store keys it: lower case, spaces
// trimmed and collapsed.
func NormalizeQuery(q string) string {
	return strings.ToLower(strings.Join(strings.Fields(q), " "))
}
