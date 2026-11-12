// Package presets reads and writes the floating terminal's presets: a name,
// a working directory, a command and environment variables, kept as JSON
// in the app's support directory.
package presets

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Preset is a terminal session the user can open: Command runs in Dir
// through the user's shell, as a login and interactive shell, so that
// their PATH and profile apply; an empty Command is the login shell.
type Preset struct {
	Name    string            `json:"name"`
	Dir     string            `json:"dir"`
	Command string            `json:"command"`
	Env     map[string]string `json:"env,omitempty"`
}

// File is the presets file.
type File struct {
	Presets []Preset `json:"presets"`
}

// DefaultPiDir is where the default "pi" preset starts: Gocast's own
// project, on the developer's Mac. Users change it in presets.json.
const DefaultPiDir = "~"

// Defaults are the presets of a new presets file.
func Defaults() []Preset {
	return []Preset{
		{Name: "Shell", Dir: "~"},
		{Name: "pi", Dir: DefaultPiDir, Command: "pi"},
	}
}

// Load reads the presets at path, writing the defaults there first when
// there is no file. A file that can't be read or parsed is an error, and
// the defaults are returned with it, so that the terminal still works.
func Load(path string) ([]Preset, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		ps := Defaults()
		return ps, Save(path, ps)
	}
	if err != nil {
		return Defaults(), err
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return Defaults(), fmt.Errorf("%s: %w", path, err)
	}
	ps := slices.DeleteFunc(f.Presets, func(p Preset) bool { return strings.TrimSpace(p.Name) == "" })
	if len(ps) == 0 {
		ps = Defaults()
	}
	return ps, nil
}

// Save writes presets to path, making its directory.
func Save(path string, ps []Preset) error {
	data, err := json.MarshalIndent(File{Presets: ps}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ExpandHome replaces a leading ~ with the home directory.
func ExpandHome(p string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	if p == "~" {
		return home
	}
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		return filepath.Join(home, rest)
	}
	return p
}

// WorkDir is the directory the preset starts in: Dir with ~ expanded, or
// the home directory when empty.
func (p Preset) WorkDir() string {
	if strings.TrimSpace(p.Dir) == "" {
		return ExpandHome("~")
	}
	return ExpandHome(p.Dir)
}

// Argv is the program to run for the preset, through shell: nil (the
// terminal's login shell) without a command, else shell -lic command.
func (p Preset) Argv(shell string) []string {
	cmd := strings.TrimSpace(p.Command)
	if cmd == "" {
		return nil
	}
	return []string{shell, "-lic", cmd}
}

// Environ is Env as KEY=value pairs, sorted by key.
func (p Preset) Environ() []string {
	var env []string
	for k, v := range p.Env {
		env = append(env, k+"="+v)
	}
	slices.Sort(env)
	return env
}

// ShortDir is the preset's folder as the user reads it: ~ for the home
// directory.
func (p Preset) ShortDir() string {
	dir := p.Dir
	if strings.TrimSpace(dir) == "" {
		return "~"
	}
	if home, err := os.UserHomeDir(); err == nil {
		if rest, ok := strings.CutPrefix(dir, home); ok && (rest == "" || rest[0] == '/') {
			dir = "~" + rest
		}
	}
	return dir
}

// UserShell is the user's shell: $SHELL, else /bin/zsh.
func UserShell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	return "/bin/zsh"
}
