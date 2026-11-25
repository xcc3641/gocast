package app

import (
	"log"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"gocast/internal/ghosttyconf"
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
	// The panel's surface, under its tab bar and terminal: grounds, when
	// set, in light and dark mode, else the tokens' glass tint at tint
	// over the glass (glassFill).
	tint    float32
	grounds *[2]termGround
	// dark sets the appearance of the panel's ui theme, and so which of
	// the terminals' themes shows, whatever the system's; nil follows the
	// system's (but see setTheme).
	dark *bool
	// ground is the index in grounds of the ground of the frame, as
	// setTheme picked it.
	ground int
	// The cursor's shape, and whether it keeps from blinking, until a
	// program sets them; the least contrast of text with its background
	// (Options.MinimumContrast).
	cursor   terminal.CursorStyle
	noBlink  bool
	contrast float32
	// The terminal's room in the panel: left and right of it, over it,
	// under it, and under it while capsules float over its bottom; the
	// terminal's own padding, inside that room (Options.Padding).
	padLeft, padRight, padTop float32
	padBottom, padCapsules    float32
	gridPadding               terminal.Padding
}

// termGround is a look's own surface, and the chrome drawn on it: the
// terminals' background, at the config's background-opacity, and their
// text (design.md 3.3b).
type termGround struct{ bg, fg ui.Color }

// lookFor is the look Settings names (settings.TermLook), for the user's
// accent color, which Gocast's follows; Ghostty's with what the user's
// Ghostty config sets (nil for none).
func lookFor(name string, prefs ui.Preferences, conf *ghosttyconf.Result) termLook {
	if name == settings.LookGocast {
		return gocastLook(prefs)
	}
	return ghosttyLook(conf)
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
		padLeft:    termPadX, padRight: termPadX, padBottom: termPadBottom, padCapsules: termPadCapsules,
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

// sysGhosttyEnv is where the user's Ghostty config is: their home folder,
// $XDG_CONFIG_HOME, and the themes of the Ghostty app installed, where
// Launch Services finds it, else in /Applications. Tests read a folder of
// their own.
var sysGhosttyEnv = func() ghosttyconf.Env {
	home, _ := os.UserHomeDir()
	env := ghosttyconf.Env{Home: home, XDGConfigHome: os.Getenv("XDG_CONFIG_HOME")}
	if app := sysAppPath(ghosttyBundleID); app != "" {
		env.Resources = append(env.Resources, filepath.Join(app, "Contents", "Resources", "ghostty"))
	}
	env.Resources = append(env.Resources, "/Applications/Ghostty.app/Contents/Resources/ghostty")
	return env
}

const ghosttyBundleID = "com.mitchellh.ghostty"

// fontsOnce adds the fonts once, and says once that it could not.
var fontsOnce sync.Once

// ghosttyLook is the floating terminal as Ghostty looks with the user's
// Ghostty config conf, or out of the box where it sets nothing or is nil
// (design.md 3.3b): an opaque dark gray panel, JetBrains Mono with rows
// its own line high, Ghostty's colors and a white block cursor, blinking,
// which no accent color changes; selected text inverted, and colors as
// programs give them (no least contrast). Its room is Gocast's, or the
// config's padding where larger: the 2 of Ghostty's would put the last
// line's first letter under the panel's round corner. The config's font
// families come first, JetBrains Mono and the symbols after them for
// what they lack, or where they aren't installed. A theme of light:…,
// dark:… gives each appearance its colors, the panel then following the
// system's; else the panel's appearance is its background's, in light
// and dark mode alike. The first call adds the fonts to the app's text.
func ghosttyLook(conf *ghosttyconf.Result) termLook {
	fontsOnce.Do(func() {
		if err := termfonts.Register(); err != nil {
			log.Printf("terminal fonts: %v", err)
		}
	})
	if conf == nil {
		conf = &ghosttyconf.Result{Light: &ghosttyconf.Config{}, Dark: &ghosttyconf.Config{}}
	}
	light, lightGround := ghosttyTheme(conf.Light)
	dark, darkGround := light, lightGround
	if conf.Conditional {
		dark, darkGround = ghosttyTheme(conf.Dark)
	}
	l := termLook{
		family:     ghosttyFamily,
		lineHeight: 1,
		fontSize:   settings.DefaultFontSize,
		theme:      light, darkTheme: dark,
		grounds:  &[2]termGround{lightGround, darkGround},
		cursor:   terminal.CursorBlock,
		contrast: 1,
		padLeft:  termPadX, padRight: termPadX, padBottom: termPadBottom, padCapsules: termPadCapsules,
		gridPadding: terminal.Padding{X: -1, Y: -1},
	}
	if !conf.Conditional {
		d := !isLight(lightGround.bg)
		l.dark = &d
	}
	// The rest is the same in light and dark mode: the light one's, where
	// a theme of each sets it (themes set colors).
	c := conf.Light
	var families []string
	for _, f := range c.FontFamily {
		// The families' list is comma-separated, unquoted (ui.Element.Font).
		if f = strings.TrimSpace(f); f != "" && !strings.Contains(f, ",") {
			families = append(families, f)
		}
	}
	if len(families) > 0 {
		l.family = strings.Join(append(families, ghosttyFamily), ", ")
	}
	if c.FontSize > 0 {
		l.fontSize = float32(min(max(c.FontSize, settings.MinFontSize), settings.MaxFontSize))
	}
	switch c.CursorStyle {
	case ghosttyconf.CursorBar:
		l.cursor = terminal.CursorBar
	case ghosttyconf.CursorUnderline:
		l.cursor = terminal.CursorUnderline
	}
	l.noBlink = c.CursorBlink != nil && !*c.CursorBlink
	if p := c.PaddingX; p != nil {
		start, end := float32(min(p.Start, maxPadX)), float32(min(p.End, maxPadX))
		l.padLeft, l.padRight = max(l.padLeft, start), max(l.padRight, end)
	}
	if p := c.PaddingY; p != nil {
		start, end := float32(min(p.Start, maxPadY)), float32(min(p.End, maxPadY))
		l.padTop = start
		l.padBottom, l.padCapsules = max(l.padBottom, end), max(l.padCapsules, end)
	}
	return l
}

// The most room the config's window-padding-x and -y give the terminal at
// each side: a larger one is taken as this, and counts as not recognized
// (lookUnrecognized). The smallest panel (termMinWidth×termMinHeight)
// keeps a terminal 240 wide (some 30 columns at 13) and 128 high (some 7
// rows), its tab bar and the room of the capsules (termPadCapsules)
// taken.
const maxPadX, maxPadY = 200, 60

// lookUnrecognized reports whether the Ghostty look can't use some of
// the appearance the config c sets: the keys it couldn't read, or a
// padding larger than the most it takes (maxPadX, maxPadY).
func lookUnrecognized(c *ghosttyconf.Config) bool {
	if len(c.Unrecognized) > 0 {
		return true
	}
	if p := c.PaddingX; p != nil && max(p.Start, p.End) > maxPadX {
		return true
	}
	p := c.PaddingY
	return p != nil && max(p.Start, p.End) > maxPadY
}

// ghosttyTheme is the terminals' colors as the config c sets them, over
// Ghostty's own: the selection inverted, and the cursor the text's color
// over the background, where it sets none; and the panel's ground, its
// background at the config's background-opacity.
func ghosttyTheme(c *ghosttyconf.Config) (*terminal.Theme, termGround) {
	color := func(v *ghosttyconf.Color, or ui.Color) ui.Color {
		if v == nil {
			return or
		}
		return ui.RGB(v.R, v.G, v.B)
	}
	fg, bg := color(c.Foreground, termText), color(c.Background, termBg)
	th := &terminal.Theme{
		Foreground: fg, Background: bg,
		Cursor: color(c.CursorColor, fg), CursorText: color(c.CursorText, bg),
		// Set both: the zero values select in a third of the
		// foreground, under the text's own colors.
		Selection: color(c.SelectionBackground, fg), SelectionText: color(c.SelectionForeground, bg),
	}
	for i, h := range ghosttyPalette {
		th.Palette[i] = color(c.Palette[i], ui.Hex(h))
	}
	g := termGround{bg: bg, fg: fg}
	if c.BackgroundOpacity != nil {
		g.bg = bg.Alpha(float32(*c.BackgroundOpacity))
	}
	return th, g
}

// isLight reports whether c is a light color: its luminance over half.
func isLight(c ui.Color) bool {
	return 0.2126*float32(c.R)+0.7152*float32(c.G)+0.0722*float32(c.B) > 127.5
}

// luminance is c's relative luminance, as WCAG 2 has it, its alpha
// ignored.
func luminance(c ui.Color) float64 {
	lin := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.04045 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

// contrast is the contrast ratio of a and b, as WCAG 2 has it: 1 to 21.
func contrast(a, b ui.Color) float64 {
	la, lb := luminance(a), luminance(b)
	return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)
}

// The gray text on a ground of the config's colors (groundTokens): the
// text moved toward the background by up to secondaryMix, as far as it
// keeps secondaryContrast with it.
const (
	secondaryMix      = 0.35
	secondaryContrast = 4.5
)

// secondaryText is the gray text on the background bg of text fg
// (design.md 3.3b): fg moved toward bg by secondaryMix, or less, where
// that would take it under secondaryContrast with bg; fg itself where fg
// is under it already.
func secondaryText(fg, bg ui.Color) ui.Color {
	if contrast(fg, bg) < secondaryContrast {
		return fg
	}
	// The contrast falls as fg nears bg: the farthest step that keeps it.
	const steps = 70
	for i := steps; i > 0; i-- {
		c := fg.Mix(bg, secondaryMix*float32(i)/steps)
		if contrast(c, bg) >= secondaryContrast {
			return c
		}
	}
	return fg
}

// minSurfaceContrast is the least contrast of a light ground's surface
// with the ground, for capsules and cards to stand out of it.
const minSurfaceContrast = 1.08

// groundTokens are the colors of the chrome on the Ghostty look's ground g
// (design.md 3.3b): on Ghostty's dark gray, the table's; on another
// background, the like from its own text and background, dark or light
// mode's grays, danger and warning as the background is; more contrast as
// the system has it, as tokensOf.
func groundTokens(g termGround, highContrast bool) tokens {
	dark := !isLight(g.bg)
	k := tokensOf(dark, highContrast) // danger, warning, panelBorder as the look has them
	bg := g.bg
	bg.A = 255
	secondary, surface := termTextSecondary, termSurface
	selection := g.fg.Alpha(0.10)
	if bg != termBg || g.fg != termText {
		// About as far from the text toward the background as Ghostty's
		// gray is, keeping its contrast; the surface a step lighter than
		// the background, or, on a light one, toward white, else, where
		// that is not apart from it (white, or near), a step darker.
		secondary = secondaryText(g.fg, bg)
		surface = bg.Mix(ui.RGB(255, 255, 255), 0.065)
		if !dark {
			surface = bg.Mix(ui.RGB(255, 255, 255), 0.6)
			if contrast(surface, bg) < minSurfaceContrast {
				surface = bg.Mix(ui.RGB(0, 0, 0), 0.05)
			}
			// The selected tab a shade darker than the background, not
			// lighter.
			selection = ui.RGBA(0, 0, 0, 0.06)
		}
	}
	k.text, k.keycapText = g.fg, g.fg
	k.textSecondary, k.textOnGlass = secondary, secondary
	k.selection, k.keycap = selection, g.fg.Alpha(0.12)
	k.hairline, k.capsuleBorder = g.fg.Alpha(0.10), g.fg.Alpha(0.10)
	k.surface, k.glassTint, k.overlay = surface, bg, surface
	if highContrast {
		k.selection = k.selection.Alpha(1.5)
		k.hairline = g.fg.Alpha(0.25)
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
	if l.grounds == nil {
		return glassFill(c, k, l.tint)
	}
	bg := l.grounds[l.ground].bg
	if !c.Vibrancy() {
		bg.A = 255 // no glass to show: opaque, as Gocast's
	}
	return bg
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// setTheme sets the panel's theme for the frame, themed's in the look's
// appearance where it has one of its own, else the system's, and returns
// the colors its chrome is drawn in. A look of a ground for each of the
// system's appearances (a theme of light:…,dark:…) takes the one of the
// system's, and the appearance of its background, so that fields and
// focus rings are drawn as the chrome: the terminals keep the theme of
// the system's appearance (termThemes).
func (l *termLook) setTheme(c *ui.Context) tokens {
	sys := c.Theme().Dark
	dark := l.dark
	if l.grounds != nil {
		l.ground = b2i(sys)
		if dark == nil {
			d := !isLight(l.grounds[l.ground].bg)
			dark = &d
		}
	}
	if dark != nil && *dark != sys {
		t := ui.LightTheme()
		if *dark {
			t = ui.DarkTheme()
		}
		c.SetTheme(t)
	}
	k := tokensFor(c)
	if l.grounds != nil {
		k = groundTokens(l.grounds[l.ground], c.Preferences().HighContrast)
	}
	c.SetTheme(themedWith(c, k))
	return k
}

// ownThemes reports whether the terminals' theme follows the system's
// appearance, not the panel's (setTheme): in the Ghostty look of a theme
// of light:…,dark:….
func (l *termLook) ownThemes() bool { return l.grounds != nil && l.dark == nil }

// termThemes are the terminals' colors, as Options.Theme and DarkTheme
// take them, in a panel whose system's appearance is sysDark: the look's
// light and dark ones, the panel's appearance picking; or, where the
// panel's appearance is its ground's (ownThemes), the one the system's
// picks alone.
func (l *termLook) termThemes(sysDark bool) (theme, dark *terminal.Theme) {
	if !l.ownThemes() {
		return l.theme, l.darkTheme
	}
	if sysDark {
		return l.darkTheme, nil
	}
	return l.theme, nil
}
