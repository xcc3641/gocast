package termfonts

import (
	"bytes"
	"testing"
)

// The fonts are embedded, each a TrueType file, and Register adds them
// once, under their families, however often the look asks.
func TestRegister(t *testing.T) {
	type call struct {
		size   int
		family string
	}
	var calls []call
	was := registerFont
	registerFont = func(data []byte, family string) error {
		calls = append(calls, call{len(data), family})
		return nil
	}
	t.Cleanup(func() { registerFont = was })

	for _, f := range fonts {
		// A TrueType file starts with version 1.0 of its table directory.
		if len(f.data) < 1024 || !bytes.HasPrefix(f.data, []byte{0, 1, 0, 0}) {
			t.Errorf("%s: %d bytes, not a TrueType font", f.name, len(f.data))
		}
	}
	for range 3 {
		if err := Register(); err != nil {
			t.Fatal(err)
		}
	}
	if len(calls) != len(fonts) {
		t.Fatalf("registered %d fonts, want %d once each", len(calls), len(fonts))
	}
	families := map[string]int{}
	for i, c := range calls {
		if c.size != len(fonts[i].data) {
			t.Errorf("font %d: %d bytes registered, want %d", i, c.size, len(fonts[i].data))
		}
		families[c.family]++
	}
	if families[Family] != 4 || families[Symbols] != 1 {
		t.Errorf("families %v: want JetBrains Mono's four faces and the symbols", families)
	}
}
