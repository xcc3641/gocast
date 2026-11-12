package presets

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestLoadMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "presets.json")
	ps, err := Load(path)
	if err != nil || len(ps) != 0 {
		t.Fatalf("no file: %+v, %v", ps, err)
	}
}

func TestLoadGivesIDs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "presets.json")
	os.WriteFile(path, []byte(`{"presets":[{"name":"a","dir":"~"},{"name":"b","dir":"~","default":true},{"name":"c","default":true}]}`), 0o644)
	ps, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 3 || ps[0].ID == "" || ps[0].ID == ps[1].ID {
		t.Fatalf("ids: %+v", ps)
	}
	if !ps[1].Default || ps[2].Default {
		t.Fatalf("one default: %+v", ps)
	}
	again, err := Load(path)
	if err != nil || !slices.EqualFunc(ps, again, func(a, b Preset) bool { return a.ID == b.ID && a.Name == b.Name }) {
		t.Fatalf("reload: %+v, %v", again, err)
	}
	if DefaultPreset(again) != 1 || Find(again, ps[2].ID) != 2 {
		t.Fatalf("find")
	}
}

func TestLoadBad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "presets.json")
	os.WriteFile(path, []byte("{nope"), 0o644)
	if _, err := Load(path); err == nil {
		t.Fatalf("bad file: no error")
	}
}

func TestValidate(t *testing.T) {
	ps := []Preset{{ID: "1", Name: "pi", Dir: "~"}}
	if n, d := Validate(Preset{ID: "2", Name: " ", Dir: ""}, ps); n != NameEmpty || d != DirEmpty {
		t.Errorf("empty: %v %v", n, d)
	}
	if n, _ := Validate(Preset{ID: "2", Name: "PI", Dir: "~"}, ps); n != NameTaken {
		t.Errorf("taken: %v", n)
	}
	if n, d := Validate(Preset{ID: "1", Name: "pi", Dir: "~"}, ps); n != OK || d != OK {
		t.Errorf("itself: %v %v", n, d)
	}
}

func TestEnv(t *testing.T) {
	env := ParseEnv("A=1\n\n# no\nexport B = two words \n=x\nC")
	if len(env) != 3 || env["A"] != "1" || env["B"] != "two words" || env["C"] != "" {
		t.Fatalf("parse: %q", env)
	}
	if got := FormatEnv(map[string]string{"B": "2", "A": "1"}); got != "A=1\nB=2" {
		t.Fatalf("format: %q", got)
	}
	if ParseEnv(" \n") != nil {
		t.Fatalf("empty env not nil")
	}
}

func TestArgv(t *testing.T) {
	if got := (Preset{}).Argv("/bin/zsh"); got != nil {
		t.Errorf("empty command: %q", got)
	}
	got := Preset{Command: "pi --continue"}.Argv("/bin/zsh")
	if !slices.Equal(got, []string{"/bin/zsh", "-lic", "pi --continue"}) {
		t.Errorf("argv: %q", got)
	}
	env := Preset{Env: map[string]string{"B": "2", "A": "1"}}.Environ()
	if !slices.Equal(env, []string{"A=1", "B=2"}) {
		t.Errorf("env: %q", env)
	}
	home, _ := os.UserHomeDir()
	if got := (Preset{Dir: "~/x"}).WorkDir(); got != filepath.Join(home, "x") {
		t.Errorf("workdir: %q", got)
	}
	if got := (Preset{Dir: "~/Projects/my-app/"}).LastDir(); got != "my-app" {
		t.Errorf("lastdir: %q", got)
	}
}
