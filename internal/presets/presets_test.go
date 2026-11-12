package presets

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestLoadWritesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "presets.json")
	ps, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 2 || ps[0].Name != "Shell" || ps[1].Command != "pi" {
		t.Fatalf("defaults: %+v", ps)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("not written: %v", err)
	}
	again, err := Load(path)
	if err != nil || !slices.EqualFunc(ps, again, func(a, b Preset) bool { return a.Name == b.Name && a.Dir == b.Dir && a.Command == b.Command }) {
		t.Fatalf("reload: %+v, %v", again, err)
	}
}

func TestLoadBad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "presets.json")
	os.WriteFile(path, []byte("{nope"), 0o644)
	ps, err := Load(path)
	if err == nil || len(ps) != 2 {
		t.Fatalf("bad file: %v %v", ps, err)
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
}
