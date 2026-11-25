package app

import (
	"reflect"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"gocast/internal/settings"
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
		if l.dark != nil || l.surface != nil || l.tint != terminalTint {
			t.Errorf("surface: dark %v, surface %v, tint %v; want the system's, the glass at %v", l.dark, l.surface, l.tint, terminalTint)
		}
		if l.cursor != terminal.CursorBlock || l.noBlink || l.contrast != terminalContrast ||
			l.gridPadding != (terminal.Padding{X: -1, Y: -1}) {
			t.Errorf("cursor %v, noBlink %v, contrast %v, padding %+v", l.cursor, l.noBlink, l.contrast, l.gridPadding)
		}
		if l.padX != termPadX || l.padBottom != termPadBottom || l.padCapsules != termPadCapsules {
			t.Errorf("room = %v, %v, %v", l.padX, l.padBottom, l.padCapsules)
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
	l := ghosttyLook()
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
	if l.surface == nil || *l.surface != ui.Hex("#282C34") || l.surface.A != 255 {
		t.Errorf("surface %v, want #282C34, opaque", l.surface)
	}
	if l.padX != 20 || l.padBottom != 12 || l.padCapsules != 60 || l.gridPadding != (terminal.Padding{X: -1, Y: -1}) {
		t.Errorf("room = %v, %v, %v, %+v", l.padX, l.padBottom, l.padCapsules, l.gridPadding)
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
		ghostty, ghosttyDark := chrome(dark, func(*ui.Context) termLook { return ghosttyLook() })
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
	if want := gocastLook(ui.Preferences{}); p.look.family != "" || p.look.contrast != terminalContrast || p.look.surface != nil ||
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
