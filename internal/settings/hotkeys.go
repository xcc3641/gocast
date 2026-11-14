package settings

import (
	"maps"
	"slices"

	"gocast/internal/shortcut"
)

// Hotkey names what a hotkey of AppHotkeys, PresetHotkeys or
// ActionHotkeys opens or runs: an app by its bundle ID, a terminal preset
// by its ID, or one of Gocast's actions by its ID (one of the three).
type Hotkey struct {
	App, Preset, Action string
}

// Hotkey is the hotkey of h, or "".
func (s Settings) Hotkey(h Hotkey) string {
	switch {
	case h.App != "":
		return s.AppHotkeys[h.App]
	case h.Action != "":
		return s.ActionHotkeys[h.Action]
	}
	return s.PresetHotkeys[h.Preset]
}

// SetHotkey makes accel the hotkey of h; "" removes it. The maps are nil
// again once empty, which settings.json then leaves out.
func (s *Settings) SetHotkey(h Hotkey, accel string) {
	m := &s.PresetHotkeys
	key := h.Preset
	switch {
	case h.App != "":
		m, key = &s.AppHotkeys, h.App
	case h.Action != "":
		m, key = &s.ActionHotkeys, h.Action
	}
	if key == "" {
		return
	}
	if accel == "" {
		delete(*m, key)
		if len(*m) == 0 {
			*m = nil
		}
		if h.App != "" {
			s.SetAppHotkeyName(h.App, "")
		}
		return
	}
	if *m == nil {
		*m = map[string]string{}
	}
	(*m)[key] = accel
}

// AppHotkeyName is the name the app with the bundle ID id had when its
// hotkey was set, or "" (none stored: set by an older build).
func (s Settings) AppHotkeyName(id string) string { return s.AppHotkeyNames[id] }

// SetAppHotkeyName stores name as the name of the app with the bundle ID
// id, which has a hotkey; "" removes it. The map is nil again once empty.
func (s *Settings) SetAppHotkeyName(id, name string) {
	if id == "" {
		return
	}
	if name == "" {
		delete(s.AppHotkeyNames, id)
		if len(s.AppHotkeyNames) == 0 {
			s.AppHotkeyNames = nil
		}
		return
	}
	if s.AppHotkeyNames == nil {
		s.AppHotkeyNames = map[string]string{}
	}
	s.AppHotkeyNames[id] = name
}

// RemoveHotkey removes the hotkey of h, and for an app the name stored
// with it.
func (s *Settings) RemoveHotkey(h Hotkey) { s.SetHotkey(h, "") }

// Hotkeys are the apps, presets and actions with a hotkey, in that
// order, each by ID: always in the same order, for the first of two with
// the same hotkey (settings.json edited by hand) to be the one that has
// it.
func (s Settings) Hotkeys() []Hotkey {
	var hs []Hotkey
	for _, id := range slices.Sorted(maps.Keys(s.AppHotkeys)) {
		hs = append(hs, Hotkey{App: id})
	}
	for _, id := range slices.Sorted(maps.Keys(s.PresetHotkeys)) {
		hs = append(hs, Hotkey{Preset: id})
	}
	for _, id := range slices.Sorted(maps.Keys(s.ActionHotkeys)) {
		hs = append(hs, Hotkey{Action: id})
	}
	return hs
}

// HotkeyOwner is the app, preset or action other than except whose hotkey is
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
