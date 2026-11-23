package settings

import (
	"path/filepath"
	"reflect"
	"testing"
)

// A group hidden whole hides its items, but those shown one by one
// meanwhile; shown again, each is as it was set one by one, those shown
// meanwhile shown (design.md 4.4: Chess hidden, System Apps off,
// Calendar on, System Apps on: all back but Chess, Calendar on).
func TestHiddenGroup(t *testing.T) {
	const (
		cal   = "app:com.apple.iCal"
		chess = "app:com.apple.Chess"
		notes = "app:com.apple.Notes"
		g     = GroupSystemApps
	)
	var s Settings
	s.SetHidden(chess, g, true)
	if !s.Hidden(chess, g) || s.Hidden(cal, g) {
		t.Fatalf("Chess hidden alone: chess %v, cal %v", s.Hidden(chess, g), s.Hidden(cal, g))
	}
	s.SetGroupHidden(g, true)
	for _, id := range []string{cal, chess, notes, "app:com.example.New"} {
		if !s.Hidden(id, g) {
			t.Errorf("group off: %s shows", id)
		}
	}
	if s.Hidden("app:com.example.Other", GroupOtherApps) {
		t.Error("group off: another group's hidden")
	}
	s.SetHidden(cal, g, false)
	if s.Hidden(cal, g) || !s.Hidden(notes, g) || !s.GroupHidden(g) {
		t.Errorf("Calendar on: cal %v, notes %v, group %v", s.Hidden(cal, g), s.Hidden(notes, g), s.GroupHidden(g))
	}

	// Kept in settings.json as it is.
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	if out, err := Load(path); err != nil || !reflect.DeepEqual(out, s) {
		t.Fatalf("Load = %+v, %v; want %+v", out, err, s)
	}

	s.SetGroupHidden(g, false)
	if s.Hidden(cal, g) || s.Hidden(notes, g) || !s.Hidden(chess, g) {
		t.Errorf("group on: cal %v, notes %v, chess %v", s.Hidden(cal, g), s.Hidden(notes, g), s.Hidden(chess, g))
	}
	if s.ShownItems != nil || s.HiddenGroups != nil {
		t.Errorf("group on: shown %v, groups %v", s.ShownItems, s.HiddenGroups)
	}

	// Chess shown while the group is off: shown once it is on again;
	// Notes hidden meanwhile: hidden.
	s.SetGroupHidden(g, true)
	s.SetHidden(chess, g, false)
	s.SetHidden(notes, g, false)
	s.SetHidden(notes, g, true)
	s.SetGroupHidden(g, false)
	if s.Hidden(chess, g) || !s.Hidden(notes, g) || s.Hidden(cal, g) {
		t.Errorf("again: chess %v, notes %v, cal %v", s.Hidden(chess, g), s.Hidden(notes, g), s.Hidden(cal, g))
	}
}
