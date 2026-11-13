package settings

import (
	"maps"
	"slices"

	"gocast/internal/shortcut"
)

// Hotkey names what a hotkey of AppHotkeys or PresetHotkeys opens: an app
// by its bundle ID, or a terminal preset by its ID (one of the two).
type Hotkey struct {
	App, Preset string
}

// Hotkey is the hotkey of h, or "".
func (s Settings) Hotkey(h Hotkey) string {
	if h.App != "" {
		return s.AppHotkeys[h.App]
	}
	return s.PresetHotkeys[h.Preset]
}

// SetHotkey makes accel the hotkey of h; "" removes it. The maps are nil
// again once empty, which settings.json then leaves out.
func (s *Settings) SetHotkey(h Hotkey, accel string) {
	m := &s.PresetHotkeys
	key := h.Preset
	if h.App != "" {
		m, key = &s.AppHotkeys, h.App
	}
	if key == "" {
		return
	}
	if accel == "" {
		delete(*m, key)
		if len(*m) == 0 {
			*m = nil
		}
		return
	}
	if *m == nil {
		*m = map[string]string{}
	}
	(*m)[key] = accel
}

// RemoveHotkey removes the hotkey of h.
func (s *Settings) RemoveHotkey(h Hotkey) { s.SetHotkey(h, "") }

// Hotkeys are the apps and presets with a hotkey, the apps first, each by
// ID: always in the same order, for the first of two with the same hotkey
// (settings.json edited by hand) to be the one that has it.
func (s Settings) Hotkeys() []Hotkey {
	var hs []Hotkey
	for _, id := range slices.Sorted(maps.Keys(s.AppHotkeys)) {
		hs = append(hs, Hotkey{App: id})
	}
	for _, id := range slices.Sorted(maps.Keys(s.PresetHotkeys)) {
		hs = append(hs, Hotkey{Preset: id})
	}
	return hs
}

// HotkeyOwner is the app or preset other than except whose hotkey is
// accel, however either is spelled ("Cmd+Alt+T" is "Alt+Cmd+T"); ok is
// false for none.
func (s Settings) HotkeyOwner(accel string, except Hotkey) (h Hotkey, ok bool) {
	for _, h := range s.Hotkeys() {
		if h != except && shortcut.Same(accel, s.Hotkey(h)) {
			return h, true
		}
	}
	return Hotkey{}, false
}

// PrunePresetHotkeys removes the hotkeys of the presets that are gone, as
// exists says; it reports whether it removed any.
func (s *Settings) PrunePresetHotkeys(exists func(id string) bool) bool {
	pruned := false
	for id := range s.PresetHotkeys {
		if !exists(id) {
			s.RemoveHotkey(Hotkey{Preset: id})
			pruned = true
		}
	}
	return pruned
}
