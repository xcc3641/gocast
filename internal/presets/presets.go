// Package presets reads and writes the floating terminal's presets: a name,
// a folder, a command and environment variables, kept as JSON in the app's
// support directory.
package presets

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Preset is a terminal the user can open: Command runs in Dir through the
// user's shell, as a login and interactive shell, so that their PATH and
// profile apply; an empty Command is the login shell.
type Preset struct {
	// ID tells presets apart across renames: tabs, usage and the search
	// list key by it. Load gives one to a preset without.
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Dir     string            `json:"dir"`
	Command string            `json:"command"`
	Env     map[string]string `json:"env,omitempty"`
	// Default is the preset the floating terminal opens the first time
	// (and for a new tab): at most one has it.
	Default bool `json:"default,omitempty"`
}

// File is the presets file.
type File struct {
	Presets []Preset `json:"presets"`
}

// NewID returns a new preset ID.
func NewID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// Load reads the presets at path: none where there is no file. A file
// that can't be read or parsed is an error, with no presets. Presets
// without an ID get one, and the file is written again with them.
func Load(path string) ([]Preset, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	ps := slices.DeleteFunc(f.Presets, func(p Preset) bool { return strings.TrimSpace(p.Name) == "" })
	changed := false
	seen := map[string]bool{}
	hasDefault := false
	for i := range ps {
		if ps[i].ID == "" || seen[ps[i].ID] {
			ps[i].ID = NewID()
			changed = true
		}
		seen[ps[i].ID] = true
		if ps[i].Default {
			if hasDefault {
				ps[i].Default = false
				changed = true
			}
			hasDefault = true
		}
	}
	if changed {
		return ps, Save(path, ps)
	}
	return ps, nil
}

// Save writes presets to path, making its directory.
func Save(path string, ps []Preset) error {
	if ps == nil {
		ps = []Preset{}
	}
	data, err := json.MarshalIndent(File{Presets: ps}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Find returns the index of the preset with the ID, or -1.
func Find(ps []Preset, id string) int {
	return slices.IndexFunc(ps, func(p Preset) bool { return p.ID == id })
}

// DefaultPreset returns the index of the default preset, or -1.
func DefaultPreset(ps []Preset) int {
	return slices.IndexFunc(ps, func(p Preset) bool { return p.Default })
}

// Problems with a preset, as Validate reports them.
type Problem int

const (
	OK Problem = iota
	NameEmpty
	NameTaken
	DirEmpty
)

// Validate checks p as it is about to be saved among ps (where it may be
// already, by its ID): the first problem of its name, and of its folder.
func Validate(p Preset, ps []Preset) (name, dir Problem) {
	n := strings.TrimSpace(p.Name)
	switch {
	case n == "":
		name = NameEmpty
	case slices.ContainsFunc(ps, func(o Preset) bool { return o.ID != p.ID && strings.EqualFold(strings.TrimSpace(o.Name), n) }):
		name = NameTaken
	}
	if strings.TrimSpace(p.Dir) == "" {
		dir = DirEmpty
	}
	return name, dir
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
func (p Preset) Argv(shell string, after After) []string {
	cmd := strings.TrimSpace(p.Command)
	if cmd == "" {
		return nil
	}
	return []string{shell, "-lic", cmd + "\n" + after.script(shell)}
}

// After is what the shell does once a preset's command ends, by itself or
// with ⌃C (design.md 3.6): says, in the terminal's dim grey (ANSI 90) on a
// line of its own, that it failed (Failed, around its exit code) or was
// not found (Missing); nothing when it ended with 0, 130 (⌃C) or 143
// (ended); sets the title Ended, which tells Gocast; and goes on as the
// user's shell, a plain terminal in the preset's folder.
type After struct {
	Failed  [2]string
	Missing string
	Ended   string
}

// AfterTitle is the title After sets once the command ends: the shell
// says so through the terminal's title (OSC 2), the one channel there is.
const AfterTitle = "gocast:command-ended"

// script is After in the language of shell: fish, or a POSIX shell. It
// starts on a line of its own, so a comment at the end of the command
// does not swallow it.
func (a After) script(shell string) string {
	q := shellQuote
	// A line of its own: at the start of the line, a line's width less one
	// of spaces stays on it and is wiped; past it, they wrap to a new one.
	line := func(code string) string {
		return `printf '%*s\r\033[K\033[90m%s\033[0m\n' "$__gocast_w" '' ` + code
	}
	if filepath.Base(shell) == "fish" {
		return strings.Join([]string{
			`set -l __gocast_s $status`,
			`set -l __gocast_w (math (tput cols 2>/dev/null; or echo 80) - 1)`,
			`switch $__gocast_s`,
			`case 0 130 143`,
			`case 127`,
			"\t" + strings.ReplaceAll(line(q(a.Missing)), `"$__gocast_w"`, `$__gocast_w`),
			`case '*'`,
			"\t" + strings.ReplaceAll(line(`(printf '%s%s%s' `+q(a.Failed[0])+` $__gocast_s `+q(a.Failed[1])+`)`), `"$__gocast_w"`, `$__gocast_w`),
			`end`,
			`printf '\033]2;%s\007' ` + q(a.Ended),
			`exec ` + q(shell) + ` -li`,
		}, "\n")
	}
	return strings.Join([]string{
		`__gocast_s=$?`,
		`__gocast_w=$(( ${COLUMNS:-$(tput cols 2>/dev/null || echo 80)} - 1 ))`,
		`case $__gocast_s in`,
		`0|130|143) ;;`,
		`127) ` + line(q(a.Missing)) + ` ;;`,
		`*) ` + line(`"$(printf '%s%s%s' `+q(a.Failed[0])+` "$__gocast_s" `+q(a.Failed[1])+`)"`) + ` ;;`,
		`esac`,
		`printf '\033]2;%s\007' ` + q(a.Ended),
		`exec ` + q(shell) + ` -li`,
	}, "\n")
}

// shellQuote is s quoted for a POSIX shell and fish.
func shellQuote(s string) string {
	if s != "" && strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789/._-") == "" {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
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

// ParseEnv reads environment variables written one per line, NAME=value;
// blank lines, lines starting with # and lines without a name are left out.
func ParseEnv(text string) map[string]string {
	env := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, _ := strings.Cut(line, "=")
		k = strings.TrimSpace(strings.TrimPrefix(k, "export "))
		if k == "" {
			continue
		}
		env[k] = strings.TrimSpace(v)
	}
	if len(env) == 0 {
		return nil
	}
	return env
}

// FormatEnv writes env one variable per line, as ParseEnv reads it.
func FormatEnv(env map[string]string) string {
	return strings.Join(Preset{Env: env}.Environ(), "\n")
}

// ShortDir is the preset's folder as the user reads it: ~ for the home
// directory.
func (p Preset) ShortDir() string {
	return ShortPath(p.Dir)
}

// ShortPath is a path as the user reads it: ~ for the home directory.
func ShortPath(dir string) string {
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

// LastDir is the last element of the preset's folder, as "my-app".
func (p Preset) LastDir() string {
	d := strings.TrimRight(p.ShortDir(), "/")
	if d == "~" || d == "" {
		return "~"
	}
	return filepath.Base(d)
}

// UserShell is the user's shell: $SHELL, else /bin/zsh.
func UserShell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	return "/bin/zsh"
}
