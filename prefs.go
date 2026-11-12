package main

import (
	"log"

	"github.com/egoist/mygo"
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
