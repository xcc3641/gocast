# Gocast

**One key. Apps and terminal, right there.**

English · [简体中文](README.zh-Hans.md)

Gocast is a small launcher for the Mac. Press ⌘Space to find and open an app. Press ⌃\` and a terminal drops down from the top of the screen, ready in the folder you work in, already running the command you use there.

<!-- Screenshots go here once the app icon is final:
     docs/screenshots/search-panel.png, docs/screenshots/floating-terminal.png -->

## What it does

**Search panel.** Type a few letters and press ↵. Apps are found by their English name, their Chinese name, or pinyin (`wx` finds 微信). The ones you open most come first. ⌘K shows more actions, such as Show in Finder or Copy Path.

**Floating terminal.** It floats above every app, on the screen your mouse is on, and slides away when you click somewhere else. Hiding it doesn't stop anything: your commands keep running and come back as you left them. Open more tabs with ⌘T.

**Presets.** A preset remembers which command to run in which folder, for example `pi` in `~/Projects/my-app`. Add presets in Settings, then type a preset's name in the search panel to open it in the floating terminal.

It lives in the menu bar, with no Dock icon. It follows your Mac's light or dark appearance and speaks English and Simplified Chinese. It updates itself.

## Install

Download the latest `.dmg` from [Releases](https://github.com/xcc3641/gocast/releases/latest), open it, and drag Gocast to Applications.

**Requires** macOS 13 or later on Apple silicon. Intel support is planned.

## First run

Gocast wants ⌘Space, which Spotlight uses by default. If Spotlight still has it, Gocast starts on ⌥⇧Space and opens Settings › Shortcuts to tell you. Turn off “Show Spotlight search” in System Settings › Keyboard › Keyboard Shortcuts › Spotlight, and Gocast switches to ⌘Space on its own. Or keep ⌥⇧Space, or set any shortcut you like.

## Keys

| Key | Where | Does |
| --- | --- | --- |
| ⌘Space | anywhere | Open the search panel |
| ⌃\` | anywhere | Show or hide the floating terminal |
| ↵ | search panel | Open the selected app or preset |
| ⌘K | search panel | More actions |
| ⌘, | search panel | Settings |
| Esc | search panel | Clear the search, press again to hide |
| ⌘T | floating terminal | New tab |
| ⌘W | floating terminal | Close tab |

Both shortcuts can be changed in Settings › Shortcuts.

## Your data

Gocast has no account and sends nothing about you anywhere. It only goes online to check for updates, which you can turn off in Settings › General. Your presets and settings are plain files in `~/Library/Application Support/dev.gocast.app/`.

## Build from source

You need a Mac with Apple silicon on macOS 13 or later, Go 1.27.3 or later, and the `mygo` command-line tool at the version `go.mod` pins (installed below; `~/go/bin` must be on your `PATH`).

```sh
git clone https://github.com/xcc3641/gocast.git
cd gocast
go install "github.com/egoist/mygo/cmd/mygo@$(go list -m -f '{{.Version}}' github.com/egoist/mygo)"
go run .            # run it
go test ./...       # run the tests
mygo build          # build Gocast.app and a .dmg into build/darwin-arm64/
```

See [CONTRIBUTING.md](CONTRIBUTING.md) before sending a change. Interface changes follow [docs/design.md](docs/design.md); wording follows [docs/copy.md](docs/copy.md).

## Thanks

Gocast is built with [MyGo](https://github.com/egoist/mygo). Its terminal runs on libghostty-vt from [Ghostty](https://ghostty.org), and pinyin search uses [go-pinyin](https://github.com/mozillazg/go-pinyin).

Gocast is inspired by Raycast and by Ghostty's quick terminal. It is not affiliated with or endorsed by either, and uses none of their code or artwork.

## License

[Apache License 2.0](LICENSE). See [NOTICE](NOTICE) and [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) for the software Gocast bundles.
