package calc

import (
	"errors"
	"math/big"
	"strings"
	"unicode"
	"unicode/utf8"
)

// normalize turns full-width forms into half-width ones and the other
// ways of writing an operator into ASCII: （12+8）×3 → (12+8)*3.
func normalize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 0xFF01 && r <= 0xFF5E: // full-width ASCII
			r -= 0xFEE0
		case r == 0x3000, r == '\t', r == '\n', r == '\r', r == 0xA0:
			r = ' '
		case r == '×', r == '·', r == '⋅', r == '∗', r == '✕', r == '✖':
			r = '*'
		case r == '÷', r == '∕':
			r = '/'
		case r == '−', r == '–', r == '﹣':
			r = '-'
		case r == '￥':
			r = '¥'
		case r == '﹪':
			r = '%'
		}
		b.WriteRune(r)
	}
	return b.String()
}

type tokKind uint8

const (
	tNum  tokKind = iota + 1 // a number; num holds it
	tWord                    // a name, keyword or Chinese word
	tOp                      // + - * / ^ ( ) % ! , = √ °
	tSym                     // a currency symbol: $ € HK$ …
)

type token struct {
	kind     tokKind
	text     string
	num      *big.Rat
	pos, end int  // byte span in the normalised input
	space    bool // whitespace before it
}

var errLex = errors.New("calc: not an expression")

const (
	maxInput  = 256 // longer input isn't a calculation
	maxDigits = 120
	maxExp10  = 1000
)

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isHan(r rune) bool { return unicode.Is(unicode.Han, r) }

// lex cuts s (normalised) into tokens.
func lex(s string) ([]token, error) {
	toks := make([]token, 0, 8)
	space := false
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		start := i
		t := token{pos: start, space: space}
		switch {
		case r == ' ':
			space = true
			i += size
			continue
		case isDigit(s[i]) || (s[i] == '.' && i+1 < len(s) && isDigit(s[i+1])):
			n, end, err := lexNumber(s, i)
			if err != nil {
				return nil, err
			}
			t.kind, t.num, t.text = tNum, n, s[i:end]
			i = end
		case r == '*' && i+1 < len(s) && s[i+1] == '*':
			t.kind, t.text = tOp, "^"
			i += 2
		case strings.ContainsRune("+-*/^()%!,=√", r):
			t.kind, t.text = tOp, string(r)
			i += size
		case r == '°':
			i += size
			if i < len(s) && strings.ContainsRune("cCfF", rune(s[i])) && (i+1 == len(s) || !isLetter(s[i+1:])) {
				t.kind, t.text = tWord, "°"+s[i:i+1]
				i++
			} else {
				t.kind, t.text = tOp, "°"
			}
		case currencySymbols[r]:
			t.kind, t.text = tSym, string(r)
			i += size
		case r == '℃' || r == '℉' || r == 'π' || r == 'τ' || r == 'µ' || r == 'μ':
			if r == 'µ' || r == 'μ' { // µm
				j := i + size
				for j < len(s) && isASCIILetter(s[j]) {
					j++
				}
				t.kind, t.text = tWord, s[i:j]
				i = j
				break
			}
			t.kind, t.text = tWord, string(r)
			i += size
		case isHan(r):
			j := i
			for j < len(s) {
				r2, n := utf8.DecodeRuneInString(s[j:])
				if !isHan(r2) {
					break
				}
				j += n
			}
			words, ok := cutCJK(s[i:j])
			if !ok {
				return nil, errLex
			}
			for k, w := range words {
				toks = append(toks, token{kind: tWord, text: w, pos: i, end: i + len(w), space: space && k == 0})
				i += len(w)
			}
			space = false
			continue
		case isASCIILetter(s[i]):
			j := i
			for j < len(s) && (isASCIILetter(s[j]) || isDigit(s[j])) {
				j++
			}
			for strings.HasPrefix(s[j:], "²") || strings.HasPrefix(s[j:], "³") {
				j += len("²")
			}
			if j < len(s) && s[j] == '$' {
				if _, ok := lookup(s[i:j] + "$"); ok {
					t.kind, t.text = tSym, s[i:j+1]
					i = j + 1
					break
				}
			}
			t.kind, t.text = tWord, s[i:j]
			i = j
		default:
			return nil, errLex
		}
		t.end = i
		toks = append(toks, t)
		space = false
	}
	return toks, nil
}

func isASCIILetter(c byte) bool { return c|0x20 >= 'a' && c|0x20 <= 'z' }

func isLetter(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return unicode.IsLetter(r)
}

// lexNumber reads the number at s[i:]: digits with thousands separators
// (1,234.5), a fraction and an exponent (1e3).
func lexNumber(s string, i int) (*big.Rat, int, error) {
	var b strings.Builder
	j := i
	for j < len(s) && isDigit(s[j]) {
		j++
	}
	b.WriteString(s[i:j])
	if n := j - i; n >= 1 && n <= 3 {
		for j+4 <= len(s) && s[j] == ',' && isDigit(s[j+1]) && isDigit(s[j+2]) && isDigit(s[j+3]) &&
			(j+4 == len(s) || !isDigit(s[j+4])) {
			b.WriteString(s[j+1 : j+4])
			j += 4
		}
	}
	if j+1 < len(s) && s[j] == '.' && isDigit(s[j+1]) || j < len(s) && s[j] == '.' && j > i {
		k := j + 1
		for k < len(s) && isDigit(s[k]) {
			k++
		}
		b.WriteString(s[j:k])
		j = k
	}
	if j < len(s) && (s[j] == 'e' || s[j] == 'E') {
		k := j + 1
		if k < len(s) && (s[k] == '+' || s[k] == '-') {
			k++
		}
		if k < len(s) && isDigit(s[k]) {
			e := 0
			for k < len(s) && isDigit(s[k]) {
				e = e*10 + int(s[k]-'0')
				if e > maxExp10 {
					return nil, 0, errLex
				}
				k++
			}
			b.WriteString(s[j:k])
			j = k
		}
	}
	text := b.String()
	if len(text) > maxDigits {
		return nil, 0, errLex
	}
	if strings.HasSuffix(text, ".") {
		text = strings.TrimSuffix(text, ".")
	}
	r, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, 0, errLex
	}
	return r, j, nil
}

// cutCJK cuts a run of Chinese characters into known words, longest
// first.
func cutCJK(run string) ([]string, bool) {
	var out []string
	for run != "" {
		n := utf8.RuneCountInString(run)
		if n > cjkMaxLen {
			n = cjkMaxLen
		}
		found := false
		for ; n > 0; n-- {
			end := 0
			for k := 0; k < n; k++ {
				_, size := utf8.DecodeRuneInString(run[end:])
				end += size
			}
			if cjkWords[run[:end]] {
				out = append(out, run[:end])
				run = run[end:]
				found = true
				break
			}
		}
		if !found {
			return nil, false
		}
	}
	return out, true
}
