package main

import (
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

// tokens are design.md's colors (1.2), for the theme's appearance, shared
// by the search panel, the floating terminal and Settings. The components
// drawn with them are in kit.go.
type tokens struct {
	text, textSecondary, textOnGlass ui.Color
	selection, hairline, keycap      ui.Color
	surface, groupFill               ui.Color
	// glassTint is laid over the system's glass by both glass windows,
	// at their own alpha (searchTint, terminalTint): the glass turns
	// mid-gray over a dark desktop in light mode (and over a light one in
	// dark mode), too close to textSecondaryOnGlass; the tint keeps it
	// light, or dark (design.md 1.2). Opaque here.
	glassTint ui.Color
	// panelBorder outlines the glass windows, capsuleBorder the capsules
	// floating in them; panelShadow is the windows' shadow.
	panelBorder, capsuleBorder, panelShadow ui.Color
	danger, warning                         ui.Color
	success                                 ui.Color // the system's green
}

// The alpha of glassTint over each glass window (design.md 1.2): the
// terminal's is thicker, for long reading of dense text; the same color,
// so that both look one material.
const (
	searchTint   = 0.72
	terminalTint = 0.88
)

func tokensFor(c *ui.Context) tokens {
	return tokensOf(c.Theme().Dark, c.Preferences().HighContrast)
}

func tokensOf(dark, highContrast bool) tokens {
	var k tokens
	if dark {
		k = tokens{
			text: ui.Hex("#F5F5F7"), textSecondary: ui.Hex("#A1A1A6"), textOnGlass: ui.Hex("#AEAEB2"),
			selection: ui.RGBA(255, 255, 255, 0.10), hairline: ui.RGBA(255, 255, 255, 0.10), keycap: ui.RGBA(255, 255, 255, 0.10),
			surface: ui.Hex("#1C1C1E"), groupFill: ui.Hex("#2C2C2E"),
			glassTint:   ui.Hex("#1C1C1E"),
			panelBorder: ui.RGBA(255, 255, 255, 0.18), capsuleBorder: ui.RGBA(255, 255, 255, 0.12),
			panelShadow: ui.RGBA(0, 0, 0, 0.55),
			danger:      ui.Hex("#FF6961"), warning: ui.Hex("#FFB340"), success: ui.Hex("#30D158"),
		}
	} else {
		k = tokens{
			text: ui.Hex("#1D1D1F"), textSecondary: ui.Hex("#6E6E73"), textOnGlass: ui.Hex("#505054"),
			selection: ui.RGBA(0, 0, 0, 0.08), hairline: ui.RGBA(0, 0, 0, 0.10), keycap: ui.RGBA(0, 0, 0, 0.06),
			surface: ui.Hex("#FFFFFF"), groupFill: ui.Hex("#F5F5F7"),
			glassTint:   ui.Hex("#F6F6F8"),
			panelBorder: ui.RGBA(0, 0, 0, 0.18), capsuleBorder: ui.RGBA(0, 0, 0, 0.08),
			panelShadow: ui.RGBA(0, 0, 0, 0.30),
			danger:      ui.Hex("#D70015"), warning: ui.Hex("#A05A00"), success: ui.Hex("#34C759"),
		}
	}
	if highContrast {
		k.selection = k.selection.Alpha(1.5)
		if dark {
			k.hairline = ui.RGBA(255, 255, 255, 0.25)
		} else {
			k.hairline = ui.RGBA(0, 0, 0, 0.25)
		}
	}
	return k
}

// themed returns the window's theme with design.md's colors where MyGo's
// differ (TextMuted, Danger, Warning), still following the system's
// accent and contrast.
func themed(c *ui.Context) *ui.Theme {
	k := tokensFor(c)
	t := *c.Theme()
	t.Text = k.text
	t.TextMuted = k.textSecondary
	t.Danger = k.danger
	t.Warning = k.warning
	return t.WithPreferences(c.Preferences())
}

// listTheme is the theme of lists in the selection gray: rows chosen in
// the neutral selection color with their text unchanged, as Raycast's,
// and no hover; the search results and Settings' sidebar.
func listTheme(c *ui.Context, k tokens) *ui.Theme {
	t := *c.Theme()
	t.Accent = k.selection
	t.InactiveAccent = k.selection
	t.AccentText = k.text
	t.SurfaceHover = ui.Transparent
	t.Radius = 8
	return &t
}

// --- The terminal's colors (design.md 3.3a) ---

// The 16 colors of programs, light and dark: the system's hues, a step
// darker in light mode, where white and bright white are grays.
var (
	lightPalette = [16]string{
		"#1D1D1F", "#C4150F", "#1F7A35", "#8F5A00", "#0A4FD1", "#8A3FB5", "#0B6F8F", "#6E6E73",
		"#5E5E63", "#E0281E", "#248A3D", "#A86A00", "#2563EB", "#A24FD0", "#0B7FA3", "#8E8E93",
	}
	darkPalette = [16]string{
		"#48484A", "#FF6961", "#30D158", "#FFD60A", "#5AA9FF", "#D18CFF", "#64D2FF", "#D1D1D6",
		"#8E8E93", "#FF8A80", "#5BE07F", "#FFE45C", "#8CC4FF", "#E2B0FF", "#A0E6FF", "#FFFFFF",
	}
)

// terminalContrast is the least contrast of a program's text with the
// glass (Options.MinimumContrast): colors short of it darken or lighten
// as little as reaches it.
const terminalContrast = 4.5

// terminalTheme is the terminal's colors on the tinted glass, for the
// accent of that appearance: the text's color, the accent's cursor and
// selection. Its background is the tint, opaque, which the glass shows
// through (the terminal is Transparent); the text under a block cursor
// and the minimum contrast take it.
func terminalTheme(dark bool, accent ui.Color) *terminal.Theme {
	k := tokensOf(dark, false)
	pal := lightPalette
	if dark {
		pal = darkPalette
	}
	th := &terminal.Theme{
		Foreground: k.text, Background: k.glassTint,
		Cursor: accent, CursorText: k.glassTint,
		Selection: accent.Alpha(0.28),
	}
	for i, h := range pal {
		th.Palette[i] = ui.Hex(h)
	}
	return th
}

// terminalThemes are the light and dark themes of the terminal for the
// user's accent color.
func terminalThemes(p ui.Preferences) (light, dark *terminal.Theme) {
	return terminalTheme(false, ui.LightTheme().WithPreferences(p).Accent),
		terminalTheme(true, ui.DarkTheme().WithPreferences(p).Accent)
}
