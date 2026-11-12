package main

import (
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// tokens are design.md's colors (1.2), for the theme's appearance, shared
// by the search panel, the floating terminal and Settings.
type tokens struct {
	text, textSecondary, textOnGlass ui.Color
	selection, hairline, keycap      ui.Color
	surface, groupFill               ui.Color
	danger, warning                  ui.Color
}

func tokensFor(c *ui.Context) tokens {
	t := c.Theme()
	p := c.Preferences()
	var k tokens
	if t.Dark {
		k = tokens{
			text: ui.Hex("#F5F5F7"), textSecondary: ui.Hex("#A1A1A6"), textOnGlass: ui.Hex("#AEAEB2"),
			selection: ui.RGBA(255, 255, 255, 0.10), hairline: ui.RGBA(255, 255, 255, 0.10), keycap: ui.RGBA(255, 255, 255, 0.10),
			surface: ui.Hex("#1C1C1E"), groupFill: ui.Hex("#2C2C2E"),
			danger: ui.Hex("#FF6961"), warning: ui.Hex("#FFB340"),
		}
	} else {
		k = tokens{
			text: ui.Hex("#1D1D1F"), textSecondary: ui.Hex("#6E6E73"), textOnGlass: ui.Hex("#5E5E63"),
			selection: ui.RGBA(0, 0, 0, 0.08), hairline: ui.RGBA(0, 0, 0, 0.10), keycap: ui.RGBA(0, 0, 0, 0.06),
			surface: ui.Hex("#FFFFFF"), groupFill: ui.Hex("#F5F5F7"),
			danger: ui.Hex("#D70015"), warning: ui.Hex("#A05A00"),
		}
	}
	if p.HighContrast {
		k.selection = k.selection.Alpha(1.5)
		if t.Dark {
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

// listTheme is the theme of the panel's lists: rows chosen in the neutral
// selection color with their text unchanged, as Raycast's, and no hover.
func listTheme(c *ui.Context, k tokens) *ui.Theme {
	t := *c.Theme()
	t.Accent = k.selection
	t.InactiveAccent = k.selection
	t.AccentText = k.text
	t.SurfaceHover = ui.Transparent
	t.Radius = 8
	return &t
}

// panelGlass is the material of the search panel: the system's Liquid
// Glass where it has it, else the opaque surface (older macOS, Reduce
// Transparency), as design.md 2.1 asks.
type panelGlass struct{ fallback ui.Color }

func (g panelGlass) NativeGlass() (ui.NativeGlassSpec, bool) {
	return glass.Glass{Native: true}.NativeGlass()
}

func (g panelGlass) PaintMaterial(p *ui.Painter, box ui.Rect, radii [4]float32) {
	if p.NativeGlassSupported() && !p.Preferences().ReduceTransparency {
		p.Hole(box, radii)
		return
	}
	p.FillRadii(box, radii, g.fallback)
}

// keycap draws a key of a shortcut, as ⌘ or K, in the footer and menus;
// assistive technology skips it (the control says its shortcut).
func keycap(c *ui.Context, k tokens, label string, size float32) {
	ui.Box(c).Height(20).MinWidth(20).Padding(0, 5).Radius(5).Background(k.keycap).Center().AccessHidden(true).Children(func() {
		ui.Text(c, label).FontSize(size).TextColor(k.textOnGlass).SingleLine()
	})
}

// keycaps draws the keys of a shortcut, 4 apart.
func keycaps(c *ui.Context, k tokens, caps []string, size float32) {
	ui.Row(c).Gap(4).AlignItems(ui.Center).AccessHidden(true).Children(func() {
		for _, cp := range caps {
			keycap(c, k, cp, size)
		}
	})
}
