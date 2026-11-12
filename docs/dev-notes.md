# Gocast dev notes

First runnable skeleton (v0.1.0). Go + MyGo native UI (`ui` package, no webview),
module `gocast`, bundle id `dev.gocast.app`, MyGo `v0.9.1-0.20261111182651-0d644f4aca24`
(real module version, no `replace`).

## Run

```sh
export PATH=/opt/homebrew/bin:$HOME/go/bin:$PATH
cd "<project dir>"
go run .
```

Gocast is a menu-bar app (LSUIElement): no Dock icon, no window at launch.
Quit from the tray menu (退出 Gocast) or Ctrl-C in the terminal.
A second launch while one is running just shows the search panel of the first.

For automation: `go build -o build/gocast . && MYGO_AUTOMATION=/tmp/gocast-auto/app.sock ./build/gocast`,
then `mygo-automation open --socket /tmp/gocast-auto/app.sock`. The automation
plugin is imported unconditionally but only listens when `MYGO_AUTOMATION` is set;
`mygo build` leaves it out (`mygo_noinspector`).

Release bundle: `mygo build` → `build/darwin-arm64/Gocast.app` + `gocast-0.1.0-arm64.dmg`
(ad-hoc signed; `libghostty-vt.dylib` is bundled in Resources).

## Layout

| File | What |
| --- | --- |
| `main.go` | app setup, single instance, global shortcuts, quit handling |
| `launcher.go` | search panel (apps + presets) |
| `termpanel.go` | floating terminal (`plugins/terminal`), one tab per preset |
| `tray.go` | tray menu |
| `automation.go` | mygo-automation plugin import |
| `internal/apps` | app scan + localized names (system app folders are listed in launcher.go) |
| `internal/presets` | presets.json load/save, defaults |
| `internal/search` | matching: substring, word initials, pinyin full/initials, usage ranking |
| `locales/` | MyGo i18n catalogs, `en` (source) + `zh-Hans`, follows system language |

## Shortcuts

- Search panel: **⌥⇧Space** (fallback ⌘⌥⇧Space if taken). Not ⌥Space — Raycast owns it.
- Floating terminal: **⌃`** (fallback ⌃⌥`).
- In the panel: ↩ open, ↑↓ select, esc clears the query, then closes.
- In the terminal: ⌘1…⌘9 / ⌘⇧[ ⌘⇧] switch tabs, the ⌃ button / shortcut hides it; the sessions keep running while hidden.

## Presets

`~/Library/Application Support/dev.gocast.app/presets.json` (same path under
`go run`, `mygo dev` and the bundle). Created with defaults on first run:

```json
{ "presets": [
  { "name": "Shell", "dir": "~" },
  { "name": "pi", "dir": "<this project>", "command": "pi" }
] }
```

Commands run through the login shell (`zsh -lic <command>`) so the user's PATH
applies. Reloaded when the file changes (checked when the panel opens).
Tray → 编辑预设文件… opens it. App usage counts are stored alongside (`usage.json`).

Behaviour: missing folder → opens in ~ and shows 「找不到文件夹 {folder}，已改在你的主文件夹里打开。」;
the command ends → 「命令已结束（退出码 {code}）」 + 重新运行.

## Verified (2026-11-12, MacBook Pro M1 Pro, system language zh-Hans-CN)

- `gofmt`, `go vet ./...`, `mygo vet`: clean.
- `go test ./...`: apps, presets, search packages pass.
- mygo-automation screenshots in `.airbuild/artifacts/screens/`:
  - `search-pinyin-wx-weixin.png` — `wx` / `weixin` finds 微信 (pinyin initials / full pinyin).
  - `search-english-saf.png` — `saf` finds Safari.
  - `terminal-pi.png` — pi preset running in the project folder, full panel width.
- Hiding and reshowing the terminal keeps the session (same pi process).
- Quit (SIGTERM / tray) exits the app and ends the child processes.
- `mygo build` succeeds; the bundle launches and quits cleanly.

## Bugs fixed during verification

- **Quit hung.** The panels hide instead of closing (`OnClose` → `PreventDefault`),
  and that also cancelled `App.Quit`. So the app never quit and left `zsh -lic pi` orphans.
  Fix: `OnBeforeQuit` sets `quitting` and the hide-on-close handler lets the close through
  (`gocast.hideOnClose`).
- **pi drew at 80 columns.** The session started before the view had sized it, and pi
  keeps its first width. Fix: a new session is started in the frame that first shows it (`session.pending`).

## Manual tests still needed (automation can't cover these)

- Real keyboard: global shortcuts, Enter in the panel (automation sends synthetic keys).
- IME: candidate window position while typing Chinese in the search field and the terminal.
- ⌘V image paste into pi (plugins/terminal sends Kitty 5522 / empty bracketed paste by default — needs a real check).
- Tray menu clicks (automation can't reach the status-item menu: `menu "搜索…"` → no menu item).

## Open issues

- `minimumSystemVersion` is 13.0 (MyGo's floor); the brief says macOS 12+.
- The brief wants a universal (arm64 + Intel) build; `mygo build` produced arm64 only.
- No Developer ID on this Mac: the bundle is ad-hoc signed, not notarized. Any
  release must ship `libghostty-vt.dylib` inside the bundle (mygo build already does that and signs it).
- `mygo build` warns "The app does not import gocast/locales". This is a false positive
  (launcher.go, termpanel.go and tray.go import it; `go list -deps` lists it). It is a MyGo bug:
  `compiledDirs` (cmd/mygo/i18n.go) splits `go list` output with `strings.Fields`, which breaks
  on the spaces in this project path (".../Application Support/..."). Report it upstream; harmless.
- Copy: the strings come from `docs/copy.md` (v1.1). `docs/design.md` and brief v1.3 have not been applied yet; that is the next run. These are not in it and were written here:
  编辑预设文件…, 正在查找应用…, 打不开“{name}”…. The no-presets hint says “去设置里添加预设”
  but there is no settings window yet (Settings/Check for Updates are shown disabled).
- The default pi preset folder is this machine's project path, hardcoded in `internal/presets`.
- An unbundled binary gets app names in English from Launch Services; the code
  corrects this with `Localized(App.Locale())`.
