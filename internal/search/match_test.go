package search

import (
	"slices"
	"testing"
)

func titles(items []Item, idx []int) []string {
	var out []string
	for _, i := range idx {
		out = append(out, items[i].Title)
	}
	return out
}

func TestEnglish(t *testing.T) {
	var items []Item
	for _, name := range []string{"Calendar", "Visual Studio Code", "Activity Monitor", "Code Editor", "Xcode", "Calculator", "Safari", "WeChat"} {
		items = append(items, NewItem(name, name))
	}
	for query, want := range map[string][]string{
		"cal":  {"Calendar", "Calculator"},                     // prefix, shorter first
		"saf":  {"Safari"},                                     // prefix
		"SAF":  {"Safari"},                                     // any case
		"code": {"Code Editor", "Visual Studio Code", "Xcode"}, // prefix, word, contains
		"vsc":  {"Visual Studio Code"},                         // initials of words
		"stu":  {"Visual Studio Code"},                         // a later word
		"wc":   {"WeChat"},                                     // camel case words
		"ACT":  {"Activity Monitor", "Calculator"},             // prefix, then letters in order
		"zzz":  nil,
	} {
		if got := titles(items, Filter(items, query)); !slices.Equal(got, want) {
			t.Errorf("%q: %v, want %v", query, got, want)
		}
	}
}

func TestChinese(t *testing.T) {
	items := []Item{
		NewItem("wechat", "微信", "WeChat"),
		NewItem("calc", "计算器", "Calculator"),
		NewItem("settings", "系统设置", "System Settings"),
		NewItem("music", "QQ音乐", "QQMusic"),
		NewItem("safari", "Safari浏览器", "Safari"),
		NewItem("notes", "备忘录", "Notes"),
	}
	for query, want := range map[string][]string{
		"微":        {"微信"},   // characters
		"计算":       {"计算器"},  // characters, prefix
		"设置":       {"系统设置"}, // characters, inside
		"weixin":   {"微信"},   // full pinyin
		"wx":       {"微信"},   // initials
		"weix":     {"微信"},   // pinyin then initial
		"wei xin":  {"微信"},   // with a space
		"jsq":      {"计算器"},  // initials
		"jisuanqi": {"计算器"},  // full pinyin
		"jisuan":   {"计算器"},  // full pinyin, start
		"xtsz":     {"系统设置"}, // initials
		"sz":       {"系统设置"}, // initials of later syllables
		"qqyy":     {"QQ音乐"}, // words and initials mixed
		"qqyinyue": {"QQ音乐"}, // 乐 read yue
		"wechat":   {"微信"},   // the English name
		"calc":     {"计算器"},  // the English name
		"saf":      {"Safari浏览器"},
		"bwl":      {"备忘录"},
		"notes":    {"备忘录"},
	} {
		if got := titles(items, Filter(items, query)); !slices.Equal(got, want) {
			t.Errorf("%q: %v, want %v", query, got, want)
		}
	}
}

func TestRanks(t *testing.T) {
	for _, c := range []struct {
		name, query string
		want        int
	}{
		{"Safari", "saf", RankPrefix},
		{"Visual Studio Code", "vsc", RankTokens},
		{"Visual Studio Code", "code", RankWord},
		{"Xcode", "code", RankContains},
		{"Activity Monitor", "atm", RankSubsequence},
		{"微信", "wx", RankTokens},
		{"微信", "weixin", RankTokens},
		{"微信", "xin", RankWord},
		{"计算器", "jsq", RankTokens},
		{"计算器", "jq", NoMatch},
		{"计算器", "xyz", NoMatch},
	} {
		if got := NewKey(c.name).Rank(c.query); got != c.want {
			t.Errorf("%q in %q: rank %d, want %d", c.query, c.name, got, c.want)
		}
	}
}

func TestBoost(t *testing.T) {
	items := []Item{NewItem("a", "Calendar"), NewItem("b", "Calculator")}
	items[1].Boost = 3
	if got := titles(items, Filter(items, "cal")); !slices.Equal(got, []string{"Calculator", "Calendar"}) {
		t.Errorf("boosted: %v", got)
	}
	if got := titles(items, Filter(items, "")); !slices.Equal(got, []string{"Calculator", "Calendar"}) {
		t.Errorf("empty query: %v", got)
	}
}
