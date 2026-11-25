package app

import (
	"log"
	"slices"
	"sync"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"gocast/internal/settings"
	"gocast/internal/termfonts"
)

// termLook is what the floating terminal looks like: its surface, the
// colors of its chrome, and its terminals' font, colors and cursor
// (design.md 3.2, 3.3, 3.3a, 3.3b). The panel takes all of these from its
// look alone, so that switching looks (Settings, copy.md 20) changes them
// together: Ghostty's, the default, or Gocast's own.
type termLook struct {
	// The terminals' font, but its size: its family, a list as
	// ui.Element.Font takes it ("" for the system's monospaced font), its
	// OpenType features and its rows' height, as a multiple of the font's
	// line height (terminal.Font).
	family     string
	features   []string
	lineHeight float32
	// fontSize is the font size of a new tab when Settings has none.
	fontSize float32
	// theme and darkTheme are the terminals' colors in light and dark
	// windows (Options.Theme, Options.DarkTheme).
	theme, darkTheme *terminal.Theme
	// The panel's surface, under its tab bar and terminal: surface, when
	// set, else the tokens' glass tint at tint over the glass (glassFill).
	tint    float32
	surface *ui.Color
	// dark sets the appearance of the panel's ui theme, and so which of
	// the terminals' themes shows, whatever the system's; nil follows the
	// system's. chrome, when set, gives the colors of the panel's chrome
	// (tab bar, capsules, hint card, menu) for the user's contrast
	// setting, in place of the system appearance's tokens.
	dark   *bool
	chrome func(highContrast bool) tokens
	// The cursor's shape, and whether it keeps from blinking, until a
	// program sets them; the least contrast of text with its background
	// (Options.MinimumContrast).
	cursor   terminal.CursorStyle
	noBlink  bool
	contrast float32
	// The terminal's room in the panel: across, under it, and under it
	// while capsules float over its bottom; the terminal's own padding,
	// inside that room (Options.Padding).
	padX, padBottom, padCapsules float32
	gridPadding                  terminal.Padding
}

// lookFor is the look Settings names (settings.TermLook), for the user's
// accent color, which Gocast's follows.
func lookFor(name string, prefs ui.Preferences) termLook {
	if name == settings.LookGocast {
		return gocastLook(prefs)
	}
	return ghosttyLook()
}

// gocastLook is Gocast's own look: the tinted glass, in the system's
// appearance, with the system's monospaced font and the colors of
// design.md 3.3a, its cursor and selection in the user's accent color.
func gocastLook(prefs ui.Preferences) termLook {
	return termLook{
		lineHeight: termLineHeight,
		fontSize:   settings.DefaultFontSize,
		theme:      terminalTheme(false, ui.LightTheme().WithPreferences(prefs).Accent),
		darkTheme:  terminalTheme(true, ui.DarkTheme().WithPreferences(prefs).Accent),
		tint:       terminalTint,
		contrast:   terminalContrast,
		padX:       termPadX, padBottom: termPadBottom, padCapsules: termPadCapsules,
		// None: the room is the view's padding (padX…), which changes
		// as capsules come and go.
		gridPadding: terminal.Padding{X: -1, Y: -1},
	}
}

// --- The Ghostty look (design.md 3.3b) ---

// The Ghostty look's colors, the same in light and dark mode, as
// Ghostty's own (design.md 3.3b): its background, under the tab bar and
// the terminal; the text on it, and the gray text; the selected tab's
// fill, and keycaps'; what floats over the panel.
var (
	termBg            = ui.Hex("#282C34")
	termText          = ui.Hex("#FFFFFF")
	termTextSecondary = ui.Hex("#ABB2BF")
	termSelection     = ui.RGBA(255, 255, 255, 0.10)
	termKeycap        = ui.RGBA(255, 255, 255, 0.12)
	termSurface       = ui.Hex("#363B45")
)

// ghosttyPalette is the 16 colors of Ghostty's built-in defaults (its
// source, not the theme of its themes folder that differs by 1).
var ghosttyPalette = [16]string{
	"#1D1F21", "#CC6666", "#B5BD68", "#F0C674", "#81A2BE", "#B294BB", "#8ABEB7", "#C5C8C6",
	"#666666", "#D54E53", "#B9CA4A", "#E7C547", "#7AA6DA", "#C397D8", "#70C0B1", "#EAEAEA",
}

// ghosttyFamily is the Ghostty look's font: JetBrains Mono, which Gocast
// carries, Symbols Nerd Font Mono for the icons of prompts it lacks, then
// the system's monospaced font should neither be there.
const ghosttyFamily = termfonts.Family + ", " + termfonts.Symbols + ", monospace"

// fontsOnce adds the fonts once, and says once that it could not.
var fontsOnce sync.Once

// ghosttyLook is the floating terminal as Ghostty looks out of the box,
// in light and dark mode alike (design.md 3.3b): an opaque dark gray
// panel, JetBrains Mono with rows its own line high, Ghostty's colors and
// a white block cursor, blinking, which no accent color changes; selected
// text inverted, and colors as programs give them (no least contrast).
// Its room is Gocast's: the 2 of Ghostty's would put the last line's
// first letter under the panel's round corner. The first call adds the
// fonts to the app's text.
func ghosttyLook() termLook {
	fontsOnce.Do(func() {
		if err := termfonts.Register(); err != nil {
			log.Printf("terminal fonts: %v", err)
		}
	})
	th := &terminal.Theme{
		Foreground: termText, Background: termBg,
		Cursor: termText, CursorText: termBg,
		// Set both: the zero values select in a third of the
		// foreground, under the text's own colors.
		Selection: termText, SelectionText: termBg,
	}
	for i, h := range ghosttyPalette {
		th.Palette[i] = ui.Hex(h)
	}
	dark := true
	surface := termBg
	return termLook{
		family:     ghosttyFamily,
		lineHeight: 1,
		fontSize:   settings.DefaultFontSize,
		theme:      th, darkTheme: th,
		surface:  &surface,
		dark:     &dark,
		chrome:   ghosttyTokens,
		cursor:   terminal.CursorBlock,
		contrast: 1,
		padX:     termPadX, padBottom: termPadBottom, padCapsules: termPadCapsules,
		gridPadding: terminal.Padding{X: -1, Y: -1},
	}
}

// ghosttyTokens are the colors of the Ghostty look's chrome on its dark
// gray (design.md 3.3b): dark mode's, with the look's text, grays, fills
// and overlays; more contrast as the system has it, as tokensOf.
func ghosttyTokens(highContrast bool) tokens {
	k := tokensOf(true, highContrast) // danger, warning, panelBorder as the look has them
	k.text, k.keycapText = termText, termText
	k.textSecondary, k.textOnGlass = termTextSecondary, termTextSecondary
	k.selection, k.keycap = termSelection, termKeycap
	k.hairline, k.capsuleBorder = ui.RGBA(255, 255, 255, 0.10), ui.RGBA(255, 255, 255, 0.10)
	k.surface, k.glassTint, k.overlay = termSurface, termBg, termSurface
	if highContrast {
		k.selection = k.selection.Alpha(1.5)
		k.hairline = ui.RGBA(255, 255, 255, 0.25)
	}
	return k
}

// font is the terminals' font at size: in Gocast's look, the system's
// monospaced font (SF Mono), its rows 1.25 its line height (design.md
// 3.3a); in Ghostty's, JetBrains Mono, its rows its own line (3.3b).
func (l *termLook) font(size float32) terminal.Font {
	return terminal.Font{Family: l.family, Size: size, Features: l.features, LineHeight: l.lineHeight}
}

// sameFont reports whether the looks' fonts are the same at each size.
func (l *termLook) sameFont(o *termLook) bool {
	return l.family == o.family && l.lineHeight == o.lineHeight && slices.Equal(l.features, o.features)
}

// fill is the panel's surface: the look's own color, or the tinted glass
// (design.md 3.2).
func (l *termLook) fill(c *ui.Context, k tokens) ui.Color {
	if l.surface != nil {
		return *l.surface
	}
	return glassFill(c, k, l.tint)
}

// setTheme sets the panel's theme for the frame, themed's in the look's
// appearance where it has one of its own, else the system's, and returns
// the colors its chrome is drawn in.
func (l *termLook) setTheme(c *ui.Context) tokens {
	if l.dark != nil && *l.dark != c.Theme().Dark {
		t := ui.LightTheme()
		if *l.dark {
			t = ui.DarkTheme()
		}
		c.SetTheme(t)
	}
	k := tokensFor(c)
	if l.chrome != nil {
		k = l.chrome(c.Preferences().HighContrast)
	}
	c.SetTheme(themedWith(c, k))
	return k
}
