package main

import (
	"log"

	"github.com/egoist/mygo"

	"gocast/internal/presets"
)

// prefsWindow is the Settings window: until it is built, the presets file.
type prefsWindow struct{ g *gocast }

func newPrefsWindow(g *gocast) *prefsWindow { return &prefsWindow{g: g} }

// openSettings shows Settings on a page ("" for the last one shown).
func (g *gocast) openSettings(page string) {
	if err := mygo.Shell.OpenPath(g.presetsPath()); err != nil {
		log.Print(err)
	}
}

// editPreset shows Settings with the editor of the preset with the ID.
func (g *gocast) editPreset(id string) { g.openSettings("presets") }

// addPreset shows Settings with the editor of a new preset.
func (g *gocast) addPreset() { g.openSettings("presets") }

// defaultPreset returns the default preset, or nil for a plain terminal.
func (g *gocast) defaultPreset() *presets.Preset {
	if i := presets.DefaultPreset(g.presets); i >= 0 {
		p := g.presets[i]
		return &p
	}
	return nil
}
