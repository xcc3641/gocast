//go:build ignore

// gen-third-party-notices writes THIRD_PARTY_NOTICES.md: the license texts
// of the third-party software compiled into Gocast for darwin/arm64, and of
// the native library MyGo bundles into Gocast.app.
//
// Run it from the repository root whenever go.mod changes:
//
//	go run scripts/gen-third-party-notices.go          # rewrite the file
//	go run scripts/gen-third-party-notices.go -check   # fail if it is stale
//
// Sources, all read locally:
//   - Go modules: the license files (LICENSE*, COPYING*, NOTICE*, PATENTS*,
//     UNLICENSE*) in the module cache, at the module's root and in every
//     directory of a compiled package up to the root.
//   - Material inside the MyGo module that carries its own license: the
//     iTerm2 color schemes (their license is in ghostty_themes.go), the
//     Ebitengine shader compiler (internal/shader/LICENSE), CLDR data and
//     a MingCute icon.
//   - libghostty-vt, which plugins/terminal downloads and `mygo build`
//     copies into Contents/Resources: its license texts are kept in
//     scripts/notices/, for the Ghostty commit below. If MyGo moves to
//     another Ghostty commit the program stops, so that the components
//     and texts in scripts/notices/ are checked again.
//   - Go's own license, for the standard library and runtime.
//
// The output depends only on go.mod, go.sum and scripts/notices/, so it is
// the same on every machine.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// ghosttyCommit is the Ghostty commit whose libghostty-vt the texts in
// scripts/notices/libghostty-vt describe.
const ghosttyCommit = "befcdfd2c3a1cb24d9ec886e93c95b2b5daa7028"

const (
	mygoModule = "github.com/egoist/mygo"
	outFile    = "THIRD_PARTY_NOTICES.md"
	noticesDir = "scripts/notices"
)

type module struct {
	Path    string
	Version string
	Dir     string
	Main    bool
	Replace *module
}

type pkg struct {
	ImportPath string
	Dir        string
	Standard   bool
	Module     *module
	GoFiles    []string
}

// licenseFile names the files that hold a license or a notice.
var licenseFile = regexp.MustCompile(`(?i)^(licen[cs]e|copying|notice|patents|unlicense)([._-].*)?$`)

func main() {
	check := flag.Bool("check", false, "exit with status 1 if "+outFile+" is not up to date, instead of writing it")
	flag.Parse()

	out, err := generate()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen-third-party-notices:", err)
		os.Exit(1)
	}
	if *check {
		old, _ := os.ReadFile(outFile)
		if !bytes.Equal(old, out) {
			fmt.Fprintln(os.Stderr, outFile+" is out of date: run go run scripts/gen-third-party-notices.go")
			os.Exit(1)
		}
		return
	}
	if err := os.WriteFile(outFile, out, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "gen-third-party-notices:", err)
		os.Exit(1)
	}
}

func generate() ([]byte, error) {
	if _, err := os.Stat("go.mod"); err != nil {
		return nil, errors.New("run it from the repository root")
	}
	pkgs, err := deps()
	if err != nil {
		return nil, err
	}

	// The compiled packages of each third-party module.
	mods := map[string]*module{}
	dirs := map[string][]string{}
	byPath := map[string]pkg{}
	for _, p := range pkgs {
		byPath[p.ImportPath] = p
		if p.Standard || p.Module == nil || p.Module.Main {
			continue
		}
		m := p.Module
		if m.Replace != nil {
			m = &module{Path: m.Path, Version: m.Replace.Version, Dir: m.Replace.Dir}
		}
		if m.Dir == "" {
			return nil, fmt.Errorf("%s is not in the module cache: run go mod download", m.Path)
		}
		mods[m.Path] = m
		dirs[m.Path] = append(dirs[m.Path], p.Dir)
	}
	paths := make([]string, 0, len(mods))
	for p := range mods {
		paths = append(paths, p)
	}
	slices.Sort(paths)

	var b strings.Builder
	var summary []row
	unicode := false // whether something above uses Unicode data
	var body strings.Builder

	// Go modules.
	body.WriteString("## Go modules\n\n")
	body.WriteString("Compiled into the Gocast executable (`Contents/MacOS/Gocast`).\n\n")
	for _, path := range paths {
		m := mods[path]
		files, err := licenseFiles(m.Dir, dirs[path])
		if err != nil {
			return nil, err
		}
		if len(files) == 0 {
			return nil, fmt.Errorf("%s %s has no license file", m.Path, m.Version)
		}
		var ids []string
		fmt.Fprintf(&body, "### %s %s\n\n", m.Path, m.Version)
		for _, f := range files {
			text, err := readText(filepath.Join(m.Dir, f))
			if err != nil {
				return nil, err
			}
			id := classify(text)
			if note := fileNotes[m.Path+"/"+f]; note != "" {
				fmt.Fprintf(&body, "%s\n\n", note)
			}
			fmt.Fprintf(&body, "`%s` (%s):\n\n", f, id)
			writeText(&body, text)
			if !strings.HasPrefix(id, "patent") && !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
		summary = append(summary, row{m.Path, m.Version, strings.Join(ids, "; ")})
	}

	// Material in MyGo with licenses of its own.
	if mg := mods[mygoModule]; mg != nil {
		var extra strings.Builder
		var extraRows []row
		if _, ok := byPath[mygoModule+"/plugins/terminal"]; ok {
			text, err := themesLicense(filepath.Join(mg.Dir, "plugins/terminal/ghostty_themes.go"))
			if err != nil {
				return nil, err
			}
			extra.WriteString("### iTerm2-Color-Schemes\n\n")
			extra.WriteString("The terminal color themes embedded in MyGo's `plugins/terminal` (`ghostty_themes.go`), from https://github.com/mbadolato/iTerm2-Color-Schemes.\n\n")
			writeText(&extra, text)
			extraRows = append(extraRows, row{"iTerm2-Color-Schemes (in MyGo's plugins/terminal)", "as shipped with Ghostty", "MIT"})
		}
		if p, ok := byPath[mygoModule+"/ui"]; ok && fileContains(p, "MingCute") {
			extra.WriteString("### MingCute icon\n\n")
			extra.WriteString("The search icon of MyGo's `ui` combo box is MingCute's (https://github.com/Richard9394/MingCute), licensed under the Apache License, Version 2.0. Its text is the same as the Apache License 2.0 reproduced above for github.com/ebitengine/purego, and at https://www.apache.org/licenses/LICENSE-2.0.\n\n")
			extraRows = append(extraRows, row{"MingCute icon (in MyGo's ui)", "", "Apache-2.0"})
		}
		if _, ok := byPath[mygoModule+"/i18n"]; ok {
			v, err := cldrVersion(filepath.Join(mg.Dir, "i18n/format_tables.go"))
			if err != nil {
				return nil, err
			}
			fmt.Fprintf(&extra, "### Unicode CLDR %s\n\n", v)
			extra.WriteString("The locale data of MyGo's `i18n` package is generated from the Unicode Common Locale Data Repository. Unicode License V3, reproduced under [Unicode data](#unicode-data).\n\n")
			extraRows = append(extraRows, row{"Unicode CLDR data (in MyGo's i18n)", v, "Unicode-3.0"})
			unicode = true
		}
		if extra.Len() > 0 {
			body.WriteString("## Inside github.com/egoist/mygo\n\n")
			body.WriteString("Third-party material that the MyGo module includes in the code compiled into Gocast, besides the license files listed above.\n\n")
			body.WriteString(extra.String())
			summary = append(summary, extraRows...)
		}
	}

	// libghostty-vt, bundled into the app by plugins/terminal.
	if _, ok := byPath[mygoModule+"/plugins/terminal"]; ok {
		commit, err := pluginGhosttyCommit(filepath.Join(mods[mygoModule].Dir, "plugins/terminal/mygo-plugin.json"))
		if err != nil {
			return nil, err
		}
		if commit != ghosttyCommit {
			return nil, fmt.Errorf("MyGo now bundles libghostty-vt from Ghostty %s, but %s describes %s: check the components it builds in, update the texts there and ghosttyCommit", commit, noticesDir+"/libghostty-vt", ghosttyCommit)
		}
		body.WriteString("## libghostty-vt\n\n")
		fmt.Fprintf(&body, "`Contents/Resources/libghostty-vt.dylib`, which MyGo's `plugins/terminal` bundles: Ghostty's terminal library, built from https://github.com/ghostty-org/ghostty/tree/%s, with the libraries below compiled into it.\n\n", ghosttyCommit)
		for _, c := range ghosttyParts {
			text, err := readText(filepath.Join(noticesDir, c.file))
			if err != nil {
				return nil, err
			}
			fmt.Fprintf(&body, "### %s\n\n%s\n\n", c.name, c.about)
			writeText(&body, text)
			summary = append(summary, row{c.name + " (in libghostty-vt)", c.version, c.license})
		}
		summary = append(summary, row{"Unicode Character Database (in libghostty-vt, through uucode)", "", "Unicode-3.0"})
		unicode = true
	}

	// Unicode data, used by CLDR and by uucode.
	if unicode {
		text, err := readText(filepath.Join(noticesDir, "unicode.LICENSE"))
		if err != nil {
			return nil, err
		}
		body.WriteString("## Unicode data\n\n")
		body.WriteString("The Unicode Character Database (in libghostty-vt, through uucode) and Unicode CLDR (in MyGo's `i18n`) are under the Unicode License V3:\n\n")
		writeText(&body, text)
	}

	// Go.
	text, err := goLicense()
	if err != nil {
		return nil, err
	}
	body.WriteString("## Go\n\n")
	body.WriteString("The Go standard library and runtime, compiled into every Go program (https://go.dev).\n\n")
	writeText(&body, text)
	summary = append(summary, row{"Go standard library and runtime", "", "BSD-3-Clause"})

	b.WriteString("<!-- Generated by scripts/gen-third-party-notices.go. Do not edit: run `go run scripts/gen-third-party-notices.go`. -->\n\n")
	b.WriteString("# Third-party notices\n\n")
	b.WriteString("Gocast is licensed under the Apache License, Version 2.0 (see LICENSE and NOTICE). The Gocast app for macOS (Apple silicon) includes the third-party software below, under its own licenses. Where a component offers a choice of licenses, the license Gocast uses it under is named and reproduced.\n\n")
	b.WriteString("| Component | Version | License |\n| --- | --- | --- |\n")
	for _, r := range summary {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", r.name, r.version, r.license)
	}
	b.WriteString("\n")
	b.WriteString(body.String())
	return []byte(strings.TrimRight(b.String(), "\n") + "\n"), nil
}

type row struct{ name, version, license string }

// fileNotes say what a license file inside a module covers, where that is
// not the module as a whole.
var fileNotes = map[string]string{
	mygoModule + "/internal/shader/LICENSE": "MyGo's `internal/shader` and `internal/shaderir` are a fork of Ebitengine's Kage shader compiler (https://github.com/hajimehoshi/ebiten), Copyright The Ebitengine Authors, under the Apache License 2.0:",
}

// ghosttyParts are the components of libghostty-vt at ghosttyCommit.
var ghosttyParts = []struct {
	name, version, license, about, file string
}{
	{
		"Ghostty", ghosttyCommit[:12], "MIT",
		"https://github.com/ghostty-org/ghostty",
		"libghostty-vt/ghostty.LICENSE",
	},
	{
		"simdutf", "9.0.0", "Apache-2.0 OR MIT (used under MIT)",
		"https://github.com/simdutf/simdutf, vendored in Ghostty's `pkg/simdutf`.",
		"libghostty-vt/simdutf.LICENSE-MIT",
	},
	{
		"Highway", "66486a10623f", "Apache-2.0 OR BSD-3-Clause (used under BSD-3-Clause)",
		"https://github.com/google/highway, through Ghostty's `pkg/highway`.",
		"libghostty-vt/highway.LICENSE-BSD3",
	},
	{
		"uucode", "9d5552455141", "MIT",
		"https://github.com/jacobsandlund/uucode, Ghostty's Unicode tables. Its Unicode data is under the Unicode License V3, reproduced under [Unicode data](#unicode-data).",
		"libghostty-vt/uucode.LICENSE.md",
	},
	{
		"Bjoern Hoehrmann's UTF-8 decoder", "", "MIT",
		"Used by uucode.",
		"libghostty-vt/uucode.LICENSE_Bjoern_Hoehrmann",
	},
}

// deps lists the packages compiled into the app for darwin/arm64.
func deps() ([]pkg, error) {
	cmd := exec.Command("go", "list", "-deps", "-json=ImportPath,Dir,Standard,Module,GoFiles", ".")
	cmd.Env = append(os.Environ(), "GOOS=darwin", "GOARCH=arm64")
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list: %w", err)
	}
	var pkgs []pkg
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var p pkg
		if err := dec.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		pkgs = append(pkgs, p)
	}
	return pkgs, nil
}

// licenseFiles returns the license files of a module that cover the given
// package directories, relative to the module's root, in order.
func licenseFiles(root string, pkgDirs []string) ([]string, error) {
	seen := map[string]bool{}
	var files []string
	for _, d := range pkgDirs {
		for {
			rel, err := filepath.Rel(root, d)
			if err != nil || strings.HasPrefix(rel, "..") {
				break
			}
			if !seen[rel] {
				seen[rel] = true
				entries, err := os.ReadDir(d)
				if err != nil {
					return nil, err
				}
				for _, e := range entries {
					if !e.IsDir() && licenseFile.MatchString(e.Name()) {
						files = append(files, filepath.ToSlash(filepath.Join(rel, e.Name())))
					}
				}
			}
			if rel == "." {
				break
			}
			d = filepath.Dir(d)
		}
	}
	// The module's own files first, then those of its directories.
	slices.SortFunc(files, func(a, b string) int {
		da, db := strings.Count(a, "/"), strings.Count(b, "/")
		if (da == 0) != (db == 0) {
			if da == 0 {
				return -1
			}
			return 1
		}
		return strings.Compare(a, b)
	})
	return files, nil
}

// classify names the license of a text, as an SPDX identifier.
func classify(text string) string {
	if m := regexp.MustCompile(`SPDX-License-Identifier:\s*(.+)`).FindStringSubmatch(text); m != nil {
		return strings.TrimSpace(m[1])
	}
	switch {
	case strings.Contains(text, "Additional IP Rights Grant (Patents)"):
		return "patent grant"
	case strings.Contains(text, "Apache License") && strings.Contains(text, "Version 2.0"):
		return "Apache-2.0"
	case strings.Contains(text, "Permission is hereby granted, free of charge"):
		return "MIT"
	case strings.Contains(text, "Redistribution and use in source and binary forms") && strings.Contains(text, "Neither the name"):
		return "BSD-3-Clause"
	case strings.Contains(text, "Redistribution and use in source and binary forms"):
		return "BSD-2-Clause"
	}
	return "see text"
}

// themesLicense returns the license that ghostty_themes.go quotes in its
// doc comment.
func themesLicense(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var lines []string
	in := false
	for _, l := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(l, "//") {
			if in {
				break
			}
			continue
		}
		t := strings.TrimPrefix(strings.TrimPrefix(l, "//"), "\t")
		if strings.TrimSpace(t) == "MIT License" {
			in = true
		}
		if in {
			lines = append(lines, strings.TrimRight(t, " "))
		}
	}
	if len(lines) == 0 || !strings.Contains(strings.Join(lines, "\n"), "Mark Badolato") {
		return "", fmt.Errorf("no license found in %s", path)
	}
	return strings.TrimSpace(strings.Join(lines, "\n")), nil
}

// cldrVersion reads the CLDR version that MyGo's locale tables come from.
func cldrVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	m := regexp.MustCompile(`from CLDR ([0-9][0-9.]*[0-9])`).FindSubmatch(data)
	if m == nil {
		return "", fmt.Errorf("no CLDR version in %s", path)
	}
	return string(m[1]), nil
}

// pluginGhosttyCommit reads the Ghostty commit of the libghostty-vt that
// plugins/terminal bundles.
func pluginGhosttyCommit(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var cfg struct {
		Libraries []struct {
			Name   string
			Source string
		}
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	for _, l := range cfg.Libraries {
		if l.Name == "libghostty-vt" {
			i := strings.LastIndex(l.Source, "/")
			return l.Source[i+1:], nil
		}
	}
	return "", fmt.Errorf("%s names no libghostty-vt", path)
}

// goLicense returns the license of the Go distribution.
func goLicense() (string, error) {
	out, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		return "", fmt.Errorf("go env GOROOT: %w", err)
	}
	root := strings.TrimSpace(string(out))
	// Some packagers, such as Homebrew, keep it beside GOROOT.
	for _, p := range []string{filepath.Join(root, "LICENSE"), filepath.Join(root, "..", "LICENSE")} {
		if text, err := readText(p); err == nil && strings.Contains(text, "The Go Authors") {
			return text, nil
		}
	}
	return "", fmt.Errorf("no Go LICENSE in or beside %s", root)
}

func fileContains(p pkg, s string) bool {
	for _, f := range p.GoFiles {
		if data, err := os.ReadFile(filepath.Join(p.Dir, f)); err == nil && bytes.Contains(data, []byte(s)) {
			return true
		}
	}
	return false
}

// readText reads a text file with Unix line endings and no trailing space.
func readText(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.Trim(strings.Join(lines, "\n"), "\n"), nil
}

// writeText writes a license text as a fenced block, with a fence longer
// than any run of backticks in it.
func writeText(b *strings.Builder, text string) {
	fence := "```"
	for strings.Contains(text, fence) {
		fence += "`"
	}
	fmt.Fprintf(b, "%stext\n%s\n%s\n\n", fence, text, fence)
}
