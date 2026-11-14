// Package search ranks launcher items against what the user types: English
// names by prefix, word and letters in order, Chinese names by their
// characters, their full pinyin ("weixin" → 微信) and their initials
// ("wx" → 微信, "jsq" → 计算器), and any mix of those ("weix", "qqyy" →
// QQ音乐).
package search

import (
	"cmp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mozillazg/go-pinyin"
)

// Ranks, best first. A lower rank shows higher.
const (
	RankPrefix      = 0 // the name starts with the query
	RankTokens      = 1 // the query spells the name's first words or syllables, whole or by their starts ("vsc", "wx", "weixin")
	RankWord        = 2 // a later word or syllable starts the query's run ("code" in "Visual Studio Code", "xin" in 微信)
	RankContains    = 3 // the name contains the query
	RankSubsequence = 4 // the name has the query's letters in order
	NoMatch         = -1
)

// Kind is the kind of match of a rank, which scores it (design.md 2.9):
// one score for the name spelled by its words or syllables, whether from
// the first (RankTokens) or a later one (RankWord); a match's place
// doesn't split it further.
func Kind(rank int) int {
	if rank == RankWord {
		return RankTokens
	}
	return rank
}

// Key is a name prepared for matching: its lower-case text and its tokens
// (words, and for each Chinese character the character and its readings).
type Key struct {
	text   string
	tokens [][]string
}

// NewKey prepares name for matching.
func NewKey(name string) Key {
	return Key{text: strings.ToLower(name), tokens: tokenize(name)}
}

var pinyinArgs = func() pinyin.Args {
	a := pinyin.NewArgs()
	a.Style = pinyin.Normal
	a.Heteronym = true // 乐 is both yue (音乐) and le (快乐): accept either
	return a
}()

// tokenize splits a name into words (ASCII runs, also split where camel
// case changes, as "WeChat" into "we" and "chat") and Chinese characters,
// each with its readings in pinyin as alternatives to the character.
func tokenize(name string) [][]string {
	var tokens [][]string
	var word []rune
	flush := func() {
		if len(word) > 0 {
			tokens = append(tokens, []string{strings.ToLower(string(word))})
			word = word[:0]
		}
	}
	runes := []rune(name)
	for i, r := range runes {
		switch {
		case unicode.Is(unicode.Han, r):
			flush()
			alts := []string{string(r)}
			for _, p := range pinyin.SinglePinyin(r, pinyinArgs) {
				if !slices.Contains(alts, p) {
					alts = append(alts, p)
				}
			}
			tokens = append(tokens, alts)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			// A new word where lower case turns upper ("WeChat"), or where
			// an upper case run ends in a word ("HTMLEditor": "html",
			// "editor"), or where letters and digits meet.
			if len(word) > 0 {
				prev := word[len(word)-1]
				switch {
				case unicode.IsUpper(r) && unicode.IsLower(prev):
					flush()
				case unicode.IsUpper(r) && unicode.IsUpper(prev) && i+1 < len(runes) && unicode.IsLower(runes[i+1]):
					flush()
				case unicode.IsDigit(r) != unicode.IsDigit(prev):
					flush()
				}
			}
			word = append(word, r)
		default:
			flush()
		}
	}
	flush()
	return tokens
}

// Rank returns how well query matches the key: one of the Rank constants,
// or NoMatch. An empty query matches everything at RankPrefix.
func (k Key) Rank(query string) int {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" || strings.HasPrefix(k.text, q) {
		return RankPrefix
	}
	compact := strings.Join(strings.Fields(q), "")
	best := NoMatch
	if start := k.tokenRun(compact); start == 0 {
		return RankTokens
	} else if start > 0 {
		best = RankWord
	}
	if best < 0 && strings.Contains(k.text, q) {
		best = RankContains
	}
	if best < 0 && subsequence(k.text, compact) {
		best = RankSubsequence
	}
	return best
}

// tokenRun returns the first token from which q is spelled by consecutive
// tokens, each taken whole or by a start of one of its alternatives (the
// last may be cut anywhere), or -1.
func (k Key) tokenRun(q string) int {
	if q == "" {
		return 0
	}
	for start := range k.tokens {
		if spells(q, k.tokens[start:]) {
			return start
		}
	}
	return -1
}

func spells(q string, tokens [][]string) bool {
	if q == "" {
		return true
	}
	if len(tokens) == 0 {
		return false
	}
	for _, alt := range tokens[0] {
		n := commonPrefix(q, alt)
		for ; n > 0; n-- {
			if n < len(q) && !utf8.RuneStart(q[n]) {
				continue // not at a character's start
			}
			if spells(q[n:], tokens[1:]) {
				return true
			}
		}
	}
	return false
}

func commonPrefix(a, b string) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}

// subsequence reports whether s has q's characters in order.
func subsequence(s, q string) bool {
	for _, r := range q {
		i := strings.IndexRune(s, r)
		if i < 0 {
			return false
		}
		s = s[i+utf8.RuneLen(r):]
	}
	return true
}

// Item is something the launcher can find: an app or a terminal preset.
type Item struct {
	ID    string // stable: usage counts and list keys use it
	Title string // what the list shows
	Keys  []Key  // the names it is found by: the title first, then others (English, Chinese…)
	// Score is how often and lately it was opened (frecency): higher
	// shows first among matches of one rank.
	Score float64
	// Tier orders items of one kind of match and Score: lower first. The
	// apps, presets and Gocast's own items are 0, its actions after them
	// (system 1, window 2), which keep their order on a tie.
	Tier int
}

// NewItem makes an item found by title and the other names.
func NewItem(id, title string, names ...string) Item {
	it := Item{ID: id, Title: title, Keys: []Key{NewKey(title)}}
	seen := map[string]bool{strings.ToLower(title): true}
	for _, n := range names {
		if n == "" || seen[strings.ToLower(n)] {
			continue
		}
		seen[strings.ToLower(n)] = true
		it.Keys = append(it.Keys, NewKey(n))
	}
	return it
}

// Rank returns the best rank of query over the item's names, and whether
// that came from its title.
func (it Item) Rank(query string) (rank int, byTitle bool) {
	rank = NoMatch
	for i, k := range it.Keys {
		r := k.Rank(query)
		if r >= 0 && (rank < 0 || r < rank) {
			rank, byTitle = r, i == 0
		}
	}
	return rank, byTitle
}

// Filter returns the indexes of the items matching query, best first:
// the item the same query opened last (chosen, "" for none) when it
// matches, then by the kind of match (Kind), then the highest Score,
// then the lowest Tier. Items of one tier above 0 (the actions) then
// keep their order, copy.md's table (design.md 2.9); the others go by
// rank (the query from the name's first words before a later one),
// then a match of the title over one of another name, then shorter
// titles, then in their order.
func Filter(items []Item, query, chosen string) []int {
	type found struct {
		i, rank int
		byTitle bool
	}
	var fs []found
	for i, it := range items {
		if r, t := it.Rank(query); r >= 0 {
			fs = append(fs, found{i, r, t})
		}
	}
	slices.SortStableFunc(fs, func(a, b found) int {
		if ac, bc := items[a.i].ID == chosen, items[b.i].ID == chosen; chosen != "" && ac != bc {
			if ac {
				return -1
			}
			return 1
		}
		if c := cmp.Compare(Kind(a.rank), Kind(b.rank)); c != 0 {
			return c
		}
		if c := cmp.Compare(items[b.i].Score, items[a.i].Score); c != 0 {
			return c
		}
		if c := cmp.Compare(items[a.i].Tier, items[b.i].Tier); c != 0 {
			return c
		}
		if items[a.i].Tier > 0 {
			return 0 // the actions' order (design.md 2.9)
		}
		if c := cmp.Compare(a.rank, b.rank); c != 0 {
			return c
		}
		if a.byTitle != b.byTitle {
			if a.byTitle {
				return -1
			}
			return 1
		}
		if strings.TrimSpace(query) == "" {
			return 0 // keep the given order (by name)
		}
		return cmp.Compare(utf8.RuneCountInString(items[a.i].Title), utf8.RuneCountInString(items[b.i].Title))
	})
	out := make([]int, len(fs))
	for i, f := range fs {
		out[i] = f.i
	}
	return out
}

// SortKey is a name to sort by: lower case, Chinese characters spelled in
// pinyin, so that 计算器 sorts among the names starting with J.
func SortKey(name string) string {
	var b strings.Builder
	for _, r := range name {
		if unicode.Is(unicode.Han, r) {
			if p := pinyin.SinglePinyin(r, pinyin.NewArgs()); len(p) > 0 {
				b.WriteString(p[0])
				b.WriteByte(' ')
				continue
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// Suggestions returns up to n items for an empty query, most used first
// (by Score, frecency), leaving out those never used.
func Suggestions(items []Item, n int) []int {
	var out []int
	for i, it := range items {
		if it.Score > 0 {
			out = append(out, i)
		}
	}
	slices.SortStableFunc(out, func(a, b int) int { return cmp.Compare(items[b].Score, items[a].Score) })
	return out[:min(n, len(out))]
}
