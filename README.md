# Gocast

**One key. Apps and terminal, right there.**

English · [简体中文](README.zh-Hans.md)

Gocast is a small launcher for the Mac. Press ⌘Space to find and open an app. Press ⌃\` and a terminal drops down from the top of the screen, ready in the folder you work in, already running the command you use there.

## What it does

**Search panel.** Type a few letters and press ↵. Apps are found by their English name, their Chinese name, or pinyin (`wx` finds 微信). The ones you open most come first. ⌘K shows more actions, such as Show in Finder or Copy Path.

**Floating terminal.** It floats above every app, on the screen your mouse is on, and slides away when you click somewhere else. Hiding it doesn't stop anything: your commands keep running and come back as you left them. Open more tabs with ⌘T.

**Presets.** A preset remembers which command to run in which folder, for example `pi` in `~/Projects/my-app`. Add presets in Settings, then type a preset's name in the search panel to open it in the floating terminal.

**Clipboard history.** Press ⌃⌘V to find the text, links, images and files you copied before, and press ↵ to paste into the app you were in. Pin the ones you use often. Paste an image into a terminal and you get the image's file path on this Mac.

**Calculator.** Type math, units or currencies right in the search panel, like `(12+8)*3`, `10 ft in m` or `100 usd in jpy`. The answer shows up on top; press ↵ to copy it. Exchange rates are fetched once a day, so it keeps working offline.

It lives in the menu bar, with no Dock icon. It follows your Mac's light or dark appearance and speaks English and Simplified Chinese. It updates itself.

## Install

**Disk image.** Download the latest `.dmg` from [Releases](https://github.com/xcc3641/gocast/releases/latest), open it, and drag Gocast to Applications. The first time, open System Settings › Privacy & Security and click Open Anyway.

**Terminal.** One line. It downloads the latest Gocast from this project's releases, checks it is the official build (and refuses it if it was changed), puts it in Applications (or `~/Applications` if it can't), needs no admin password, and opens Gocast. macOS will not ask you to click Open Anyway. The [install script](install.sh) is in this repository; you can read it first.

```sh
curl -fsSL https://github.com/xcc3641/gocast/releases/latest/download/install.sh | sh
```

To uninstall:

```sh
curl -fsSL https://github.com/xcc3641/gocast/releases/latest/download/install.sh | sh -s -- --uninstall
```

Your presets, settings and clipboard history stay.

**Requires** macOS 13 or later on Apple silicon. Intel support is planned.

## First run

Gocast wants ⌘Space, which Spotlight uses by default. If Spotlight still has it, Gocast starts on ⌥⇧Space and opens Settings › Shortcuts to tell you. Turn off “Show Spotlight search” in System Settings › Keyboard › Keyboard Shortcuts › Spotlight, and Gocast switches to ⌘Space on its own. Or keep ⌥⇧Space, or set any shortcut you like.

## Keys

| Key | Where | Does |
| --- | --- | --- |
| ⌘Space | anywhere | Open the search panel |
| ⌃\` | anywhere | Show or hide the floating terminal |
| ⌃⌘V | anywhere | Open clipboard history |
| ↵ | search panel | Open the selected app or preset |
| ⌘K | search panel | More actions |
| ⌘, | search panel | Settings |
| Esc | search panel | Clear the search, press again to hide |
| ⌘T | floating terminal | New tab |
| ⌘W | floating terminal | Close tab |

All three shortcuts can be changed in Settings › Shortcuts.

## Your data

Gocast has no account and sends nothing about you anywhere. It only goes online to check for updates and to fetch exchange rates once a day, and that request carries nothing you typed. Turn either off in Settings › General (Check for updates automatically, Calculate in search). Your presets and settings are plain files in `~/Library/Application Support/dev.gocast.app/`.

Clipboard history stays on this Mac too, in the same folder, and is never uploaded. Passwords from password managers are never kept. Turn it off, clear it, or exclude apps in Settings › Clipboard.

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

Gocast is built with [MyGo](https://github.com/egoist/mygo). Its terminal runs on libghostty-vt from [Ghostty](https://ghostty.org), pinyin search uses [go-pinyin](https://github.com/mozillazg/go-pinyin), and exchange rates come from [Frankfurter](https://frankfurter.dev), which publishes central bank reference rates.

Gocast is inspired by Raycast and by Ghostty's quick terminal. It is not affiliated with or endorsed by either, and uses none of their code or artwork.

## License

[Apache License 2.0](LICENSE). See [NOTICE](NOTICE) and [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) for the software Gocast bundles.
