// Package termfonts carries the fonts of the floating terminal's Ghostty
// look (design.md 3.3b), as Ghostty carries them: JetBrains Mono 2.304,
// its regular, bold, italic and bold italic faces (SIL Open Font License
// 1.1, JetBrainsMono-OFL.txt), and Symbols Nerd Font Mono of Nerd Fonts
// 3.4.0, the icons of prompts, which Ghostty falls back to (MIT,
// SymbolsNerdFont-LICENSE). THIRD_PARTY_NOTICES.md reproduces both
// licenses.
//
// Register adds them to the app's text, once, as Gocast builds the
// floating terminal in the Ghostty look, at start by default: in Gocast's main process only, never in the terminal host
// (`Gocast --terminal-host`), which draws no text.
package termfonts

import (
	_ "embed"
	"fmt"
	"sync"

	"github.com/egoist/mygo/ui"
)

// The families the fonts are registered under, as terminal.Font.Family
// names them: Family first, Symbols for what it lacks.
const (
	Family  = "JetBrains Mono"
	Symbols = "Symbols Nerd Font Mono"
)

var (
	//go:embed JetBrainsMono-Regular.ttf
	regular []byte
	//go:embed JetBrainsMono-Bold.ttf
	bold []byte
	//go:embed JetBrainsMono-Italic.ttf
	italic []byte
	//go:embed JetBrainsMono-BoldItalic.ttf
	boldItalic []byte
	//go:embed SymbolsNerdFontMono-Regular.ttf
	symbols []byte
)

// font is a font file and the family it goes in.
type font struct {
	name, family string
	data         []byte
}

// fonts are the files, each JetBrains Mono face under the one family,
// which the text picks bold and italic faces of.
var fonts = []font{
	{"JetBrainsMono-Regular.ttf", Family, regular},
	{"JetBrainsMono-Bold.ttf", Family, bold},
	{"JetBrainsMono-Italic.ttf", Family, italic},
	{"JetBrainsMono-BoldItalic.ttf", Family, boldItalic},
	{"SymbolsNerdFontMono-Regular.ttf", Symbols, symbols},
}

// registerFont adds a font to the app's text: ui.RegisterFont, swapped in
// tests.
var registerFont = ui.RegisterFont

var (
	once sync.Once
	err  error
)

// Register adds the fonts to the app's text, the first time it is called;
// it reports the first font that could not be added, every time. Text
// naming a family that failed falls back to the system's monospaced font.
func Register() error {
	once.Do(func() {
		for _, f := range fonts {
			if e := registerFont(f.data, f.family); e != nil && err == nil {
				err = fmt.Errorf("termfonts: %s: %w", f.name, e)
			}
		}
	})
	return err
}
