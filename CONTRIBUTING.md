# Contributing to Gocast

Thanks for helping. Bug reports and feature ideas go in the issue tracker;
security problems go through the private channel in [SECURITY.md](SECURITY.md).
Everyone taking part follows the [code of conduct](CODE_OF_CONDUCT.md).

## Prerequisites

- macOS 13 or later on Apple silicon. Gocast builds arm64 only for now.
- Go 1.27.3 or later (the `go` line of [go.mod](go.mod)). With an older Go
  1.21+ installed, the default `GOTOOLCHAIN=auto` downloads the right one.
- The `mygo` command-line tool of [MyGo](https://github.com/egoist/mygo),
  at the version go.mod pins. From the repository root:

  ```sh
  go install "github.com/egoist/mygo/cmd/mygo@$(go list -m -f '{{.Version}}' github.com/egoist/mygo)"
  ```

  It goes to `$(go env GOPATH)/bin` (usually `~/go/bin`), which needs to be
  on your `PATH`. Install it again whenever go.mod moves to another MyGo
  version. Use this command rather than `@latest`: Gocast depends on a MyGo
  commit that is newer than MyGo's latest release. `mygo doctor` checks the
  setup.

## Build and run

```sh
go run .            # build and start Gocast
mygo dev            # or: a "Gocast Dev" app bundle that rebuilds when you edit
go build -o build/gocast .
mygo build          # build/darwin-arm64/Gocast.app and gocast-<version>-arm64.dmg
```

Gocast is a menu-bar app: it has no Dock icon. Started by hand it shows the
search panel; quit it from the menu-bar icon (Quit Gocast) or with Ctrl-C.
`GOCAST_APPEARANCE=dark` (or `light`) and `MYGO_LOCALE=en` (or `zh-Hans`)
force the appearance and the language for one run, which is handy for
checking both themes and both languages.

A development build shares its data with an installed Gocast, in
`~/Library/Application Support/dev.gocast.app/`. More details, including
how to drive the app with `mygo-automation`, are in
[docs/dev-notes.md](docs/dev-notes.md).

## Checks

Run these before sending a change; CI runs the same:

```sh
gofmt -l .          # must print nothing
go vet ./...
mygo vet
go test ./...
go run scripts/gen-third-party-notices.go -check
```

Changes that touch the UI also need a run of the app in light and dark
appearance, in English and Simplified Chinese.

## Project layout

| Path | What |
| --- | --- |
| `main.go` | App setup, single instance, global shortcuts, settings, quitting |
| `launcher.go` | The search panel: field, results, actions capsule, ⌘K menu |
| `termpanel.go` | The floating terminal: tabs, banners, show and hide |
| `prefs.go` | The Settings window |
| `tray.go`, `updates.go` | The menu-bar icon and menu; the update window |
| `kit.go`, `theme.go` | Shared components and the color tokens they draw with |
| `internal/` | App scanning, search and pinyin matching, usage ranking, presets, settings, shortcuts, running apps |
| `locales/` | The UI text: `en.json` (source) and `zh-Hans.json`; `catalog_gen.go` is generated |
| `assets/` | Icons, with the scripts that draw them |
| `docs/` | `design.md` (the design spec), `copy.md` (the UI text), `dev-notes.md` |
| `scripts/` | `gen-third-party-notices.go` and the license texts it uses |

## UI and text

- UI changes follow [docs/design.md](docs/design.md). If a change needs
  the design to change, update or discuss design.md first; where the code
  has to deviate, write it down under "Deviations from design.md" in
  docs/dev-notes.md.
- All user-facing text goes through the catalogs, `locales/en.json` and
  `locales/zh-Hans.json`, never as literals in the code.
  [docs/copy.md](docs/copy.md) is the source of the wording: add or change
  the text there, then in both catalogs. After editing a catalog, run
  `mygo generate` (it rewrites `locales/catalog_gen.go`) and
  `mygo i18n check`.

## Commits and pull requests

- Keep a pull request to one change, with tests where the logic allows
  (`go test` runs without a window).
- Fill in the [pull request template](.github/pull_request_template.md):
  what changed, why, how you tested it, and light and dark screenshots for
  UI changes.
- Write commit messages that say what changed and why, in the imperative
  ("Fix …", "Add …"). Reference the design.md or copy.md section a change
  implements.
- Changes that users notice get a line under an `## [Unreleased]` section
  of [CHANGELOG.md](CHANGELOG.md), in Chinese and English.
- By contributing you agree that your contribution is licensed under the
  [Apache License 2.0](LICENSE).

## Third-party notices

[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) holds the licenses of
everything compiled into or bundled with the app, and `mygo build` copies it
into `Gocast.app/Contents/Resources`. It is generated; do not edit it by
hand. After any change to go.mod or go.sum, run:

```sh
go run scripts/gen-third-party-notices.go
```

and commit the result. The script reads the module cache, so it works
offline once the modules are downloaded. The texts of libghostty-vt's
components, which are not in the module cache, are kept in
`scripts/notices/`; when MyGo moves to another Ghostty commit the script
stops and asks for them to be checked and updated.

## Releasing

For maintainers. Releases are published as GitHub releases of
[xcc3641/gocast](https://github.com/xcc3641/gocast), which installed apps
check for updates: `updates` in mygo.json holds the public key and that
repository (see the Updates section of docs/dev-notes.md). Uploading needs
the GitHub CLI (`gh`), signed in with access to the repository.

1. Bump `version` in [mygo.json](mygo.json).
2. In CHANGELOG.md, turn `## [Unreleased]` into `## [x.y.z] - YYYY-MM-DD`
   for that version. The section is required: `mygo build` stops without
   it, and it is the release notes the update window shows, so write it for
   users, in Chinese and English.
3. Make sure the checks above pass and THIRD_PARTY_NOTICES.md is current.
   Commit, tag `vx.y.z` and push both.
4. Build and upload with the update signing key, which comes from the
   maintainers' secret store. Never commit it, paste it anywhere or keep it
   in the repository:

   ```sh
   MYGO_UPDATER_PRIVATE_KEY="$(cat path/to/mygo-update.key)" mygo build -upload
   ```

   This builds the app, the disk image and the signed update files and
   uploads them to a draft release for the version on xcc3641/gocast.
   Without the key, `mygo build` still builds the app and the disk image
   but skips the update files (fine for local builds), and `-upload`
   refuses to run.
5. Check the draft on GitHub (try the disk image), then publish it.
   Publishing makes it the latest release: installed apps find it on their
   next automatic check, or when the user chooses Check for Updates.

Developer ID signing and notarization are not set up yet (not decided).
Until they are, builds are ad-hoc signed and macOS asks users to confirm
the first time they open Gocast.
