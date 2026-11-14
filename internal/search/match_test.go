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
		if got := titles(items, Filter(items, query, "")); !slices.Equal(got, want) {
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
		if got := titles(items, Filter(items, query, "")); !slices.Equal(got, want) {
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
		{"微信", "weix", RankTokens},
		{"QQ音乐", "qqyy", RankTokens},
		// A syllable cut before the query's end: only to its initial.
		{"左三分之一", "saf", NoMatch},
		{"左三分之一", "sanf", RankWord},
		{"左三分之一", "sf", RankWord},
		{"左三分之一", "zuosanf", RankTokens},
		{"左三分之一", "zuos", RankTokens},
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
	items[1].Score = 300
	if got := titles(items, Filter(items, "cal", "")); !slices.Equal(got, []string{"Calculator", "Calendar"}) {
		t.Errorf("boosted: %v", got)
	}
	if got := titles(items, Filter(items, "", "")); !slices.Equal(got, []string{"Calculator", "Calendar"}) {
		t.Errorf("empty query: %v", got)
	}
	// The item a query opened last comes first for it, whatever the rank.
	items = append(items, NewItem("c", "iCal Helper"))
	if got := titles(items, Filter(items, "cal", "c")); got[0] != "iCal Helper" {
		t.Errorf("chosen: %v", got)
	}
}

func TestSuggestions(t *testing.T) {
	items := []Item{NewItem("a", "A"), NewItem("b", "B"), NewItem("c", "C"), NewItem("d", "D")}
	items[0].Score, items[1].Score, items[3].Score = 1, 0, 5
	items[2].Score = 3
	got := Suggestions(items, 2)
	if len(got) != 2 || got[0] != 3 || got[1] != 2 {
		t.Fatalf("Suggestions = %v, want [3 2]", got)
	}
	if got := Suggestions(items, 10); len(got) != 3 {
		t.Fatalf("Suggestions(10) = %v, want the 3 used items", got)
	}
}

func TestFilterTier(t *testing.T) {
	// Of one rank and score, a lower tier first, before the shorter title:
	// an app before an action (design.md 2.9).
	items := []Item{
		{ID: "action:sleep", Title: "Sleep", Keys: []Key{NewKey("Sleep")}, Tier: 1},
		{ID: "app:safari", Title: "Safari", Keys: []Key{NewKey("Safari")}},
		{ID: "action:left", Title: "Left Half", Keys: []Key{NewKey("Left Half"), NewKey("S")}, Tier: 2},
	}
	got := Filter(items, "s", "")
	if len(got) != 3 || items[got[0]].ID != "app:safari" || items[got[1]].ID != "action:sleep" || items[got[2]].ID != "action:left" {
		t.Fatalf("order: %v", got)
	}
	// Opened more, an action comes first all the same.
	items[0].Score = 1
	if got := Filter(items, "s", ""); items[got[0]].ID != "action:sleep" {
		t.Fatalf("an action opened more: %v", got)
	}
}

func TestFilterKind(t *testing.T) {
	// One kind of match is one score (design.md 2.9): the actions keep
	// their order whatever the match's place, title or length; the apps
	// still go by the first words before a later one, then shorter.
	names := []string{"左三分之一", "中间三分之一", "右三分之一", "左三分之二", "右三分之二"}
	var items []Item
	for _, n := range names {
		it := NewItem("action:"+n, n)
		it.Tier = 2
		items = append(items, it)
	}
	if got := titles(items, Filter(items, "三分之", "")); !slices.Equal(got, names) {
		t.Errorf("三分之: %v", got)
	}
	if got := titles(items, Filter(items, "sanfenzhi", "")); !slices.Equal(got, names) {
		t.Errorf("sanfenzhi: %v", got)
	}
	// Spelled from the first syllable or a later one: one kind.
	both := []Item{NewItem("action:center", "居中"), NewItem("action:c3", "中间三分之一")}
	both[0].Tier, both[1].Tier = 2, 2
	if got := titles(both, Filter(both, "zhong", "")); !slices.Equal(got, []string{"居中", "中间三分之一"}) {
		t.Errorf("zhong: %v", got)
	}
	// The apps: the first words before a later one, then shorter.
	apps := []Item{NewItem("app:a", "Studio Code Helper"), NewItem("app:b", "Visual Studio Code"), NewItem("app:c", "Code")}
	if got := titles(apps, Filter(apps, "code", "")); !slices.Equal(got, []string{"Code", "Studio Code Helper", "Visual Studio Code"}) {
		t.Errorf("apps: %v", got)
	}
	// An app of the same kind before an action, though from a later word.
	mixed := []Item{{ID: "action:left", Title: "左三分之一", Keys: []Key{NewKey("左三分之一")}, Tier: 2}, NewItem("app:x", "Xin Sanfen")}
	if got := titles(mixed, Filter(mixed, "san", "")); !slices.Equal(got, []string{"Xin Sanfen", "左三分之一"}) {
		t.Errorf("mixed: %v", got)
	}
}
