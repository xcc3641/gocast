package app

import (
	"reflect"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"gocast/internal/ghosttyconf"
	"gocast/internal/settings"
	"gocast/locales"
)

// Gocast's look is the panel as it looked before looks: the system's
// monospaced font with its rows 1.25 high, terminalTheme's colors for the
// accent of each appearance, the tinted glass in the system's appearance.
func TestGocastLook(t *testing.T) {
	for _, prefs := range []ui.Preferences{{}, {Accent: ui.Hex("#FF2D55")}} {
		l := gocastLook(prefs)
		for _, size := range []float32{settings.MinFontSize, settings.DefaultFontSize, 16, settings.MaxFontSize} {
			if got, want := l.font(size), (terminal.Font{Size: size, LineHeight: termLineHeight}); !reflect.DeepEqual(got, want) {
				t.Errorf("font(%v) = %+v, want %+v", size, got, want)
			}
		}
		if l.fontSize != settings.DefaultFontSize {
			t.Errorf("fontSize = %v, want %v", l.fontSize, settings.DefaultFontSize)
		}
		light := terminalTheme(false, ui.LightTheme().WithPreferences(prefs).Accent)
		dark := terminalTheme(true, ui.DarkTheme().WithPreferences(prefs).Accent)
		if !reflect.DeepEqual(l.theme, light) || !reflect.DeepEqual(l.darkTheme, dark) {
			t.Errorf("accent %v: themes = %+v, %+v, want %+v, %+v", prefs.Accent, l.theme, l.darkTheme, light, dark)
		}
		if l.dark != nil || l.grounds != nil || l.tint != terminalTint {
			t.Errorf("surface: dark %v, grounds %v, tint %v; want the system's, the glass at %v", l.dark, l.grounds, l.tint, terminalTint)
		}
		if l.cursor != terminal.CursorBlock || l.noBlink || l.contrast != terminalContrast ||
			l.gridPadding != (terminal.Padding{X: -1, Y: -1}) {
			t.Errorf("cursor %v, noBlink %v, contrast %v, padding %+v", l.cursor, l.noBlink, l.contrast, l.gridPadding)
		}
		if l.padLeft != termPadX || l.padRight != termPadX || l.padTop != 0 || l.padBottom != termPadBottom || l.padCapsules != termPadCapsules {
			t.Errorf("room = %v, %v, %v, %v, %v", l.padLeft, l.padRight, l.padTop, l.padBottom, l.padCapsules)
		}
	}
	// The accent colors the cursor and selection.
	if a, b := gocastLook(ui.Preferences{}), gocastLook(ui.Preferences{Accent: ui.Hex("#FF2D55")}); a.theme.Cursor == b.theme.Cursor {
		t.Errorf("cursor %v whatever the accent", a.theme.Cursor)
	}
}

// The panel's tabs take the look's font size unless Settings has one.
func TestTermPanelFontSize(t *testing.T) {
	p := &termPanel{g: &gocast{}, look: gocastLook(ui.Preferences{})}
	if got := p.fontSize(); got != settings.DefaultFontSize {
		t.Errorf("fontSize() = %v, want %v", got, settings.DefaultFontSize)
	}
	p.g.settings.FontSize = 99
	if got := p.fontSize(); got != settings.MaxFontSize {
		t.Errorf("fontSize() = %v, want %v", got, settings.MaxFontSize)
	}
}

// The Ghostty look is Ghostty's out of the box (design.md 3.3b): its
// colors, its white block cursor blinking and inverted selection, no
// least contrast, JetBrains Mono with the symbols after it and rows its
// own line high, on an opaque dark gray, in Gocast's room.
func TestGhosttyLook(t *testing.T) {
	l := ghosttyLook(nil)
	if got, want := l.font(15), (terminal.Font{Family: "JetBrains Mono, Symbols Nerd Font Mono, monospace", Size: 15, LineHeight: 1}); !reflect.DeepEqual(got, want) {
		t.Errorf("font = %+v, want %+v", got, want)
	}
	want := &terminal.Theme{
		Foreground: ui.Hex("#FFFFFF"), Background: ui.Hex("#282C34"),
		Cursor: ui.Hex("#FFFFFF"), CursorText: ui.Hex("#282C34"),
		Selection: ui.Hex("#FFFFFF"), SelectionText: ui.Hex("#282C34"),
	}
	for i, h := range []string{
		"#1D1F21", "#CC6666", "#B5BD68", "#F0C674", "#81A2BE", "#B294BB", "#8ABEB7", "#C5C8C6",
		"#666666", "#D54E53", "#B9CA4A", "#E7C547", "#7AA6DA", "#C397D8", "#70C0B1", "#EAEAEA",
	} {
		want.Palette[i] = ui.Hex(h)
	}
	if !reflect.DeepEqual(l.theme, want) || !reflect.DeepEqual(l.darkTheme, want) {
		t.Errorf("themes = %+v, %+v, want %+v in both", l.theme, l.darkTheme, want)
	}
	if l.cursor != terminal.CursorBlock || l.noBlink || l.contrast != 1 {
		t.Errorf("cursor %v, noBlink %v, contrast %v", l.cursor, l.noBlink, l.contrast)
	}
	if l.grounds == nil || l.grounds[0] != l.grounds[1] || l.grounds[0].bg != ui.Hex("#282C34") || l.grounds[0].bg.A != 255 || l.dark == nil || !*l.dark {
		t.Errorf("grounds %v, dark %v, want #282C34, opaque, dark", l.grounds, l.dark)
	}
	if l.padLeft != 20 || l.padRight != 20 || l.padTop != 0 || l.padBottom != 12 || l.padCapsules != 60 || l.gridPadding != (terminal.Padding{X: -1, Y: -1}) {
		t.Errorf("room = %v, %v, %v, %v, %v, %+v", l.padLeft, l.padRight, l.padTop, l.padBottom, l.padCapsules, l.gridPadding)
	}
	if l.fontSize != settings.DefaultFontSize {
		t.Errorf("fontSize = %v", l.fontSize)
	}
}

// The Ghostty look's chrome is its dark gray's in light and dark mode
// alike, with its overlays opaque (design.md 3.3b); Gocast's follows the
// system, as the search panel does.
func TestTermLookChrome(t *testing.T) {
	// Each look in a window of its own, as SetTheme is the window's.
	chrome := func(dark bool, look func(c *ui.Context) termLook) (k tokens, chromeDark bool) {
		tt := ui.NewTester(func(c *ui.Context) {
			l := look(c)
			k, chromeDark = l.setTheme(c), c.Theme().Dark
		}, 200, 100)
		tt.SetDark(dark)
		tt.Frame()
		return k, chromeDark
	}
	for _, dark := range []bool{false, true} {
		ghostty, ghosttyDark := chrome(dark, func(*ui.Context) termLook { return ghosttyLook(nil) })
		gocast, gocastDark := chrome(dark, func(c *ui.Context) termLook { return gocastLook(c.Preferences()) })
		if !ghosttyDark || ghostty.text != ui.Hex("#FFFFFF") || ghostty.textOnGlass != ui.Hex("#ABB2BF") ||
			ghostty.keycapText != ui.Hex("#FFFFFF") || ghostty.overlay != ui.Hex("#363B45") ||
			ghostty.panelBorder != ui.RGBA(255, 255, 255, 0.18) || ghostty.warning != ui.Hex("#FFB340") {
			t.Errorf("system dark %v: the Ghostty look's chrome dark %v, %+v", dark, ghosttyDark, ghostty)
		}
		if gocastDark != dark || gocast != tokensOf(dark, false) {
			t.Errorf("system dark %v: Gocast's chrome dark %v, %+v", dark, gocastDark, gocast)
		}
	}
}

// Choosing a look in Settings changes the open panel's at its next frame:
// its surface, chrome and terminals (design.md 3.3b, 切换外观时).
func TestTermLookSwitch(t *testing.T) {
	p, tt := hintsPanel(t, "en", 1000, 600, "~ % ", "pi")
	if p.look.family != ghosttyFamily || p.look.contrast != 1 {
		t.Fatalf("by default: %q, contrast %v", p.look.family, p.look.contrast)
	}
	p.g.settings.SetTermLook(settings.LookGocast)
	p.lookChanged()
	tt.Frame()
	if want := gocastLook(ui.Preferences{}); p.look.family != "" || p.look.contrast != terminalContrast || p.look.grounds != nil ||
		!reflect.DeepEqual(p.look.theme, want.theme) {
		t.Errorf("Gocast chosen: %+v", p.look)
	}
	p.g.settings.SetTermLook(settings.LookGhostty)
	tt.Frame()
	if p.look.family != ghosttyFamily {
		t.Errorf("Ghostty again: %q", p.look.family)
	}
}

// TestTermLookSwitchHidden: a look chosen while the panel is hidden, which
// draws no frame, places the panel at the pointer with that look's rows.
func TestTermLookSwitchHidden(t *testing.T) {
	p, _ := hintsPanel(t, "en", 1000, 600, "~ % ", "pi")
	p.g.settings.SetTermLook(settings.LookGocast)
	p.lookChanged()
	if p.look.family != "" {
		t.Fatalf("Gocast chosen, no frame: %q", p.look.family)
	}
	p.g.settings.SetTermLook(settings.LookGhostty)
	p.lookChanged()
	if p.look.family != ghosttyFamily || p.look.contrast != 1 {
		t.Errorf("Ghostty chosen, no frame: %q, contrast %v", p.look.family, p.look.contrast)
	}
}

func TestCloseColor(t *testing.T) {
	white := ui.Hex("#FFFFFF")
	gh := tokens{textOnGlass: termTextSecondary, overlay: termSurface}
	if got := closeColor(gh, white); got != termTextSecondary {
		t.Errorf("Ghostty: %v", got)
	}
	if got := closeColor(tokens{textOnGlass: termTextSecondary}, white); got != white {
		t.Errorf("Gocast: %v", got)
	}
}

// The paths of the Ghostty config files of useGhosttyConfig.
const (
	ghosttyConfig       = ".config/ghostty/config"
	ghosttyAppSupport   = "Library/Application Support/com.mitchellh.ghostty/config.ghostty"
	ghosttyThemeShipped = "res/themes/Mocha"
)

// mochaTheme is a theme as Ghostty ships them: colors alone.
const mochaTheme = "palette = 0=#45475a\npalette = 1=#f38ba8\nbackground = #1e1e2e\nforeground = #cdd6f4\n" +
	"cursor-color = #f5e0dc\ncursor-text = #11111b\nselection-background = #353749\nselection-foreground = #cdd6f4\n"

// The Ghostty look takes what the user's Ghostty config sets of the
// appearance, over Ghostty's own (design.md 3.3b): the theme's colors,
// the config's over them, its fonts before JetBrains Mono, its cursor,
// its padding where larger than Gocast's room, its background's opacity
// over the glass; the panel's appearance its background's.
func TestGhosttyLookConfig(t *testing.T) {
	useGhosttyConfig(t, map[string]string{
		ghosttyThemeShipped: mochaTheme,
		ghosttyConfig: "theme = Mocha\npalette = 1=#ff0000\nfont-family = Fira Code\nfont-family = Not, A Font\nfont-size = 15.5\n" +
			"cursor-style = bar\ncursor-style-blink = false\nwindow-padding-x = 24,8\nwindow-padding-y = 6,30\nbackground-opacity = 0.8\n" +
			"keybind = cmd+t=new_tab\n",
	})
	l := ghosttyLook(ghosttyconf.Load(sysGhosttyEnv()))
	if want := (terminal.Font{Family: "Fira Code, " + ghosttyFamily, Size: 15, LineHeight: 1}); !reflect.DeepEqual(l.font(15), want) || l.fontSize != 15.5 {
		t.Errorf("font %+v, size %v; want %+v, 15.5", l.font(15), l.fontSize, want)
	}
	th := l.theme
	if th != l.darkTheme || th.Background != ui.Hex("#1e1e2e") || th.Foreground != ui.Hex("#cdd6f4") ||
		th.Palette[0] != ui.Hex("#45475a") || th.Palette[1] != ui.Hex("#ff0000") || th.Palette[2] != ui.Hex("#B5BD68") ||
		th.Cursor != ui.Hex("#f5e0dc") || th.CursorText != ui.Hex("#11111b") ||
		th.Selection != ui.Hex("#353749") || th.SelectionText != ui.Hex("#cdd6f4") {
		t.Errorf("theme %+v", th)
	}
	if l.cursor != terminal.CursorBar || !l.noBlink || l.contrast != 1 {
		t.Errorf("cursor %v, noBlink %v, contrast %v", l.cursor, l.noBlink, l.contrast)
	}
	if l.padLeft != 24 || l.padRight != 20 || l.padTop != 6 || l.padBottom != 30 || l.padCapsules != 60 {
		t.Errorf("room %v %v %v %v %v", l.padLeft, l.padRight, l.padTop, l.padBottom, l.padCapsules)
	}
	if g := l.grounds[0]; l.dark == nil || !*l.dark || g != l.grounds[1] || g.bg != ui.Hex("#1e1e2e").Alpha(0.8) {
		t.Errorf("dark %v, ground %+v", l.dark, g)
	}
	// The chrome on it: its text, and its background opaque.
	var k tokens
	var fill ui.Color
	var panelDark bool
	tt := ui.NewTester(func(c *ui.Context) {
		k = l.setTheme(c)
		fill, panelDark = l.fill(c, k), c.Theme().Dark
	}, 200, 100)
	tt.SetDark(false)
	tt.SetVibrancy(true)
	tt.Frame()
	if k.text != ui.Hex("#cdd6f4") || k.glassTint != ui.Hex("#1e1e2e") || !panelDark || fill != ui.Hex("#1e1e2e").Alpha(0.8) {
		t.Errorf("chrome %+v, dark %v, fill %v", k, panelDark, fill)
	}
	tt.SetVibrancy(false) // Reduce Transparency: no glass to show through
	tt.Frame()
	if fill != ui.Hex("#1e1e2e") {
		t.Errorf("fill without the glass %v", fill)
	}
}

// A light background makes the panel light in either mode; a theme of
// light:…,dark:… colors each mode its own, the panel following the
// system's.
func TestGhosttyLookAppearance(t *testing.T) {
	home := useGhosttyConfig(t, map[string]string{
		ghosttyThemeShipped: mochaTheme,
		"res/themes/Latte":  "background = #eff1f5\nforeground = #4c4f69\n",
		ghosttyConfig:       "theme = Latte\n",
	})
	l := ghosttyLook(ghosttyconf.Load(sysGhosttyEnv()))
	if l.dark == nil || *l.dark || l.theme.Background != ui.Hex("#eff1f5") {
		t.Errorf("light theme: dark %v, %v", l.dark, l.theme.Background)
	}
	writeGhosttyFile(t, home, ghosttyConfig, "theme = light:Latte,dark:Mocha\n")
	l = ghosttyLook(ghosttyconf.Load(sysGhosttyEnv()))
	if l.dark != nil || l.theme.Background != ui.Hex("#eff1f5") || l.darkTheme.Background != ui.Hex("#1e1e2e") {
		t.Fatalf("light and dark: dark %v, %v, %v", l.dark, l.theme.Background, l.darkTheme.Background)
	}
	for dark, want := range map[bool]ui.Color{false: ui.Hex("#eff1f5"), true: ui.Hex("#1e1e2e")} {
		var fill ui.Color
		var panelDark bool
		tt := ui.NewTester(func(c *ui.Context) {
			k := l.setTheme(c)
			fill, panelDark = l.fill(c, k), c.Theme().Dark
		}, 200, 100)
		tt.SetDark(dark)
		tt.Frame()
		if panelDark != dark || fill != want {
			t.Errorf("system dark %v: dark %v, fill %v", dark, panelDark, fill)
		}
	}
}

// New tabs take the Ghostty config's font-size while the user never chose
// one in Settings, within Settings' bounds; any the user chose, 13 too,
// from then on; Gocast's look takes none of Ghostty's.
func TestGhosttyFontSize(t *testing.T) {
	home := useGhosttyConfig(t, map[string]string{ghosttyConfig: "font-size = 15\n"})
	p := &termPanel{g: &gocast{}}
	p.readConf()
	p.applyLook(p.prefs)
	if got := p.fontSize(); got != 15 {
		t.Errorf("never chosen: %v, want Ghostty's 15", got)
	}
	p.g.settings.FontSize = settings.DefaultFontSize
	if got := p.fontSize(); got != settings.DefaultFontSize {
		t.Errorf("13 chosen: %v", got)
	}
	p.g.settings.FontSize = 0
	p.g.settings.SetTermLook(settings.LookGocast)
	p.lookChanged()
	if got := p.fontSize(); got != settings.DefaultFontSize {
		t.Errorf("Gocast's look: %v", got)
	}
	writeGhosttyFile(t, home, ghosttyConfig, "font-size = 40\n")
	p.g.settings.SetTermLook(settings.LookGhostty)
	p.lookChanged()
	if got := p.fontSize(); got != settings.MaxFontSize {
		t.Errorf("Ghostty's 40: %v", got)
	}
}

// The config is read again as the panel shows where a file it was read
// from, its theme or a file it includes, changed, or one it looked for is
// there now (design.md 3.3b): the tabs take the new colors, and those at
// the font size new tabs had the new one; a tab zoomed keeps its size.
// Not read while Gocast's look is chosen; as the Ghostty look is chosen.
func TestGhosttyConfigReload(t *testing.T) {
	home := useGhosttyConfig(t, map[string]string{
		ghosttyThemeShipped:    mochaTheme,
		ghosttyConfig:          "theme = Mocha\nconfig-file = more\n",
		".config/ghostty/more": "font-size = 14\n",
	})
	p := &termPanel{g: &gocast{}}
	p.tabs = []*session{{fontSize: 14}, {fontSize: 18}}
	show := func() { p.readConf(); p.applyLook(p.prefs) } // as show does
	show()
	if p.look.theme.Background != ui.Hex("#1e1e2e") || p.fontSize() != 14 {
		t.Fatalf("read: %v, %v", p.look.theme.Background, p.fontSize())
	}
	look := p.look
	show()
	if p.look.theme != look.theme {
		t.Error("made again with nothing changed")
	}
	writeGhosttyFile(t, home, ".config/ghostty/more", "font-size = 16\n")
	show()
	if p.fontSize() != 16 || p.tabs[0].fontSize != 16 || p.tabs[1].fontSize != 18 {
		t.Errorf("include edited: base %v, tabs %v, %v", p.fontSize(), p.tabs[0].fontSize, p.tabs[1].fontSize)
	}
	writeGhosttyFile(t, home, ghosttyThemeShipped, "background = #000001\n")
	show()
	if p.look.theme.Background != ui.Hex("#000001") {
		t.Errorf("theme edited: %v", p.look.theme.Background)
	}
	writeGhosttyFile(t, home, ghosttyAppSupport, "background = #000002\n")
	show()
	if p.look.theme.Background != ui.Hex("#000002") {
		t.Errorf("a file of the four added: %v", p.look.theme.Background)
	}

	p.g.settings.SetTermLook(settings.LookGocast)
	p.lookChanged()
	writeGhosttyFile(t, home, ghosttyAppSupport, "background = #000003\n")
	show()
	if p.conf.Light.Background.B == 3 || p.look.grounds != nil {
		t.Error("read with Gocast's look")
	}
	p.g.settings.SetTermLook(settings.LookGhostty)
	p.lookChanged()
	if p.look.theme.Background != ui.Hex("#000003") {
		t.Errorf("Ghostty's chosen again: %v", p.look.theme.Background)
	}
}

// The note under the look says whether the config gave the terminal its
// look, and whether some of it was not recognized (copy.md 20).
func TestGhosttyConfigHint(t *testing.T) {
	inLocale(t, "en")
	for _, c := range []struct{ conf, want string }{
		{"", locales.GeneralTermLookGhosttyHint()},
		{"# nothing\nkeybind = cmd+t=new_tab\nnot-a-key = 1\n", locales.GeneralTermLookGhosttyHint()},
		{"background = #102030\n", locales.GeneralTermLookConfigHint()},
		{"background = #102030\nforeground = papayawhip\n", locales.GeneralTermLookConfigPartlyHint()},
		{"theme = No Such Theme\n", locales.GeneralTermLookGhosttyPartlyHint()},
	} {
		useGhosttyConfig(t, map[string]string{ghosttyConfig: c.conf})
		p := &termPanel{g: &gocast{}}
		p.readConf()
		if got := p.confHint(); got != c.want {
			t.Errorf("%q: %q, want %q", c.conf, got, c.want)
		}
	}
}

// The config's window-padding is taken up to maxPadX and maxPadY at each
// side, a larger one as these, and then counted as not recognized, so
// that the smallest panel keeps a terminal of use.
func TestGhosttyPaddingBounds(t *testing.T) {
	inLocale(t, "en")
	for _, c := range []struct {
		conf                               string
		left, right, top, bottom, capsules float32
		partly                             bool
	}{
		{"window-padding-x = 200\nwindow-padding-y = 60\n", 200, 200, 60, 60, 60, false},
		{"window-padding-x = 500,10\nwindow-padding-y = 300\n", 200, 20, 60, 60, 60, true},
		{"window-padding-x = 4\nwindow-padding-y = 9999,4\n", 20, 20, 60, 12, 60, true},
	} {
		useGhosttyConfig(t, map[string]string{ghosttyConfig: c.conf})
		p := &termPanel{g: &gocast{}}
		p.readConf()
		l := ghosttyLook(p.conf)
		if l.padLeft != c.left || l.padRight != c.right || l.padTop != c.top || l.padBottom != c.bottom || l.padCapsules != c.capsules {
			t.Errorf("%q: room %v %v %v %v %v", c.conf, l.padLeft, l.padRight, l.padTop, l.padBottom, l.padCapsules)
		}
		want := locales.GeneralTermLookConfigHint()
		if c.partly {
			want = locales.GeneralTermLookConfigPartlyHint()
		}
		if got := p.confHint(); got != want {
			t.Errorf("%q: hint %q, want %q", c.conf, got, want)
		}
	}
	// The smallest panel keeps room for the terminal.
	if w, h := termMinWidth-2*maxPadX, termMinHeight-tabBarH-2*maxPadY; w < 200 || h < 100 {
		t.Errorf("the smallest terminal: %v×%v", w, h)
	}
}

// The gray text moves from the text toward the background by 35% at most,
// as far as it keeps 4.5:1 with it, and is the text where that is under
// 4.5:1 already (design.md 3.3b).
func TestSecondaryText(t *testing.T) {
	for _, c := range []struct {
		name, fg, bg string
		moved        bool
	}{
		{"Catppuccin Latte", "#4C4F69", "#EFF1F5", true},
		{"Solarized dark", "#839496", "#002B36", true},
		{"Solarized light", "#657B83", "#FDF6E3", false},
	} {
		fg, bg := ui.Hex(c.fg), ui.Hex(c.bg)
		got := secondaryText(fg, bg)
		if got != fg && contrast(got, bg) < 4.5 {
			t.Errorf("%s: %v, %.2f:1", c.name, got, contrast(got, bg))
		}
		if moved := got != fg; moved != c.moved {
			t.Errorf("%s: %v, moved %v, want %v", c.name, got, moved, c.moved)
		}
		if c.moved && contrast(got, bg) > 4.7 {
			t.Errorf("%s: %v, %.2f:1, stopped short of 4.5", c.name, got, contrast(got, bg))
		}
	}
	// Far apart, the full 35%.
	white, black := ui.Hex("#FFFFFF"), ui.Hex("#000000")
	if got := secondaryText(white, black); got != white.Mix(black, 0.35) {
		t.Errorf("white on black: %v", got)
	}
	k := groundTokens(termGround{bg: ui.Hex("#EFF1F5"), fg: ui.Hex("#4C4F69")}, false)
	if k.textSecondary != secondaryText(ui.Hex("#4C4F69"), ui.Hex("#EFF1F5")) || k.textOnGlass != k.textSecondary {
		t.Errorf("tokens: %v, %v", k.textSecondary, k.textOnGlass)
	}
}

// On a light background the surface is 60% toward white, but where that
// can't be told from the background (white, or near), 5% toward black;
// the selected tab 6% black. On a dark one, 6.5% toward white and the
// text at 10%, as before (design.md 3.3b).
func TestGroundTokensLight(t *testing.T) {
	white, black := ui.RGB(255, 255, 255), ui.RGB(0, 0, 0)
	for _, c := range []struct {
		bg      string
		surface ui.Color
	}{
		{"#FFFFFF", white.Mix(black, 0.05)},
		{"#FDF6E3", ui.Hex("#FDF6E3").Mix(black, 0.05)},
		{"#EEE8D5", ui.Hex("#EEE8D5").Mix(white, 0.6)},
		{"#E0E0E0", ui.Hex("#E0E0E0").Mix(white, 0.6)},
	} {
		bg := ui.Hex(c.bg)
		k := groundTokens(termGround{bg: bg, fg: ui.Hex("#333333")}, false)
		if k.surface != c.surface || k.overlay != c.surface || contrast(k.surface, bg) < minSurfaceContrast {
			t.Errorf("%s: surface %v, %.3f:1, want %v", c.bg, k.surface, contrast(k.surface, bg), c.surface)
		}
		if k.selection != ui.RGBA(0, 0, 0, 0.06) {
			t.Errorf("%s: selection %v", c.bg, k.selection)
		}
	}
	bg, fg := ui.Hex("#1E1E2E"), ui.Hex("#CDD6F4")
	if k := groundTokens(termGround{bg: bg, fg: fg}, false); k.surface != bg.Mix(white, 0.065) || k.selection != fg.Alpha(0.10) {
		t.Errorf("dark: surface %v, selection %v", k.surface, k.selection)
	}
}

// A theme of light:…,dark:… gives the panel the ground of the system's
// appearance, and its ui theme the appearance of that ground's background,
// so that fields and focus rings are drawn as the chrome; the terminals
// keep the colors of the system's appearance.
func TestGhosttyLookConditionalTheme(t *testing.T) {
	home := useGhosttyConfig(t, map[string]string{
		ghosttyThemeShipped: mochaTheme,
		"res/themes/Latte":  "background = #eff1f5\nforeground = #4c4f69\n",
		"res/themes/Nord":   "background = #2e3440\nforeground = #d8dee9\n",
		ghosttyConfig:       "theme = light:Mocha,dark:Latte\n",
	})
	frame := func(l *termLook, sysDark bool) (panelDark bool, fill ui.Color) {
		tt := ui.NewTester(func(c *ui.Context) {
			k := l.setTheme(c)
			fill, panelDark = l.fill(c, k), c.Theme().Dark
		}, 200, 100)
		tt.SetDark(sysDark)
		tt.Frame()
		return panelDark, fill
	}
	mocha, latte := ui.Hex("#1e1e2e"), ui.Hex("#eff1f5")
	l := ghosttyLook(ghosttyconf.Load(sysGhosttyEnv()))
	if !l.ownThemes() {
		t.Fatal("light and dark: not the system's themes")
	}
	for sysDark, want := range map[bool]struct {
		dark bool
		bg   ui.Color
	}{false: {true, mocha}, true: {false, latte}} {
		if dark, fill := frame(&l, sysDark); dark != want.dark || fill != want.bg {
			t.Errorf("inverted, system dark %v: panel dark %v, fill %v", sysDark, dark, fill)
		}
		if th, dk := l.termThemes(sysDark); th.Background != want.bg || dk != nil {
			t.Errorf("inverted, system dark %v: terminals %v, %v", sysDark, th.Background, dk)
		}
	}
	// Both dark: the panel dark in either, the terminals the system's.
	writeGhosttyFile(t, home, ghosttyConfig, "theme = light:Nord,dark:Mocha\n")
	l = ghosttyLook(ghosttyconf.Load(sysGhosttyEnv()))
	for sysDark, want := range map[bool]ui.Color{false: ui.Hex("#2e3440"), true: mocha} {
		if dark, fill := frame(&l, sysDark); !dark || fill != want {
			t.Errorf("both dark, system dark %v: panel dark %v, fill %v", sysDark, dark, fill)
		}
		if th, _ := l.termThemes(sysDark); th.Background != want {
			t.Errorf("both dark, system dark %v: terminals %v", sysDark, th.Background)
		}
	}
	// The panel gives its terminals the themes of the system's appearance
	// as it changes.
	p := &termPanel{g: &gocast{}}
	p.readConf()
	p.sysDark = true
	p.applyLook(p.prefs)
	if !p.themed {
		t.Error("made in dark mode: themes not dark")
	}
	p.sysDark = false
	p.applyLook(p.prefs)
	if p.themed {
		t.Error("light mode: themes not light")
	}
	// Without a config, as before: the system's themes picked by the
	// panel's appearance.
	l = ghosttyLook(nil)
	if th, dk := l.termThemes(false); th == nil || th != dk || l.ownThemes() {
		t.Errorf("no config: %v, %v", th, dk)
	}
}
