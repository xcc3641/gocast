package settings

import "slices"

// The groups of Settings › Search & Shortcuts, in its order (design.md
// 4.4, copy.md 19.2): Gocast's own items, the system's and the window's
// actions with Keep Awake, the terminal presets, the apps under
// /System/Applications, and the others.
const (
	GroupGocast     = "gocast"
	GroupSystem     = "system"
	GroupPresets    = "presets"
	GroupSystemApps = "systemApps"
	GroupOtherApps  = "otherApps"
)

// Groups are the groups, in order.
var Groups = []string{GroupGocast, GroupSystem, GroupPresets, GroupSystemApps, GroupOtherApps}

// Hidden reports whether the search leaves out the item with the ID, of
// group: while the group is hidden, unless shown one by one since; else
// if hidden one by one (design.md 4.4).
func (s Settings) Hidden(id, group string) bool {
	if s.GroupHidden(group) {
		return !slices.Contains(s.ShownItems[group], id)
	}
	return slices.Contains(s.HiddenItems, id)
}

// SetHidden hides the item with the ID, of group, or shows it. While the
// group is hidden, it is among the items shown one by one or not, and
// among those hidden one by one or not, so that it stays as it is set
// once the group shows again.
func (s *Settings) SetHidden(id, group string, hidden bool) {
	if s.GroupHidden(group) {
		shown := slices.DeleteFunc(s.ShownItems[group], func(x string) bool { return x == id })
		if !hidden {
			shown = append(shown, id)
		}
		s.setShown(group, shown)
	}
	s.HiddenItems = slices.DeleteFunc(s.HiddenItems, func(x string) bool { return x == id })
	if hidden {
		s.HiddenItems = append(s.HiddenItems, id)
	}
	if len(s.HiddenItems) == 0 {
		s.HiddenItems = nil
	}
}

// setShown makes ids the items of group shown one by one, the map nil
// again once empty.
func (s *Settings) setShown(group string, ids []string) {
	if len(ids) == 0 {
		delete(s.ShownItems, group)
		if len(s.ShownItems) == 0 {
			s.ShownItems = nil
		}
		return
	}
	if s.ShownItems == nil {
		s.ShownItems = map[string][]string{}
	}
	s.ShownItems[group] = ids
}

// GroupHidden reports whether the group is hidden whole.
func (s Settings) GroupHidden(group string) bool { return slices.Contains(s.HiddenGroups, group) }

// SetGroupHidden hides the group whole, or shows it again: each of its
// items then as it was set one by one, those shown while it was hidden
// shown (design.md 4.4).
func (s *Settings) SetGroupHidden(group string, hidden bool) {
	s.HiddenGroups = slices.DeleteFunc(s.HiddenGroups, func(x string) bool { return x == group })
	s.setShown(group, nil)
	if hidden {
		s.HiddenGroups = append(s.HiddenGroups, group)
	}
	if len(s.HiddenGroups) == 0 {
		s.HiddenGroups = nil
	}
}

// Collapsed reports whether Settings › Search & Shortcuts shows the
// group collapsed.
func (s Settings) Collapsed(group string) bool { return slices.Contains(s.CollapsedGroups, group) }

// SetCollapsed collapses the group, or expands it.
func (s *Settings) SetCollapsed(group string, collapsed bool) {
	s.CollapsedGroups = slices.DeleteFunc(s.CollapsedGroups, func(x string) bool { return x == group })
	if collapsed {
		s.CollapsedGroups = append(s.CollapsedGroups, group)
	}
	if len(s.CollapsedGroups) == 0 {
		s.CollapsedGroups = nil
	}
}
