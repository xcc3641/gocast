# Gocast dev notes

Gocast v1 (0.1.0). It is written in Go with MyGo's native UI (the `ui` package, no webview).
- Module `gocast`, bundle id `dev.gocast.app`.
- MyGo `v0.9.1-0.20261111182651-0d644f4aca24` (the real module version, no `replace`).

The sources, in priority order:
- `docs/design.md` (wins on any conflict)
- `docs/brief.md` v1.3
- `docs/copy.md` v1.2 (every string is in `locales/`)
- the `[v1]` items in `docs/raycast-reference.md`

## Run

```sh
export PATH=/opt/homebrew/bin:$HOME/go/bin:$PATH
cd "<project dir>"
go run .
```

Gocast is a menu-bar app (LSUIElement). It has no Dock icon and no app-switcher entry.
- Started by hand, it shows the search panel. Started at login, it shows nothing.
- To quit, use the tray menu (退出 Gocast) or Ctrl-C.
- Only one copy runs. A second launch shows the first one's search panel.

**Appearance.** `GOCAST_APPEARANCE=dark` (or `light`) forces Gocast's appearance and leaves the Mac's alone. Use it for screenshots and for checking both themes.

**Automation:**
1. `go build -o build/gocast . && MYGO_AUTOMATION=/tmp/gocast-auto/app.sock ./build/gocast`
2. `mygo-automation open --socket /tmp/gocast-auto/app.sock`

The windows are `w1` the search panel, `w2` the floating terminal and `w3` Settings. `./build/gocast` run a second time shows the panel again. ⌘, in the panel opens Settings.

The automation plugin is always imported but listens only when `MYGO_AUTOMATION` is set. `mygo build` leaves it out (`mygo_noinspector`).

The single-instance lock is keyed by the app's name. An unpackaged build (`gocast`) therefore runs alongside an installed `Gocast.app`. The two share `~/Library/Application Support/dev.gocast.app/`.

For real window captures, `screencapture -x -o -l <CGWindowID>` works. Get the ID from `CGWindowListCopyWindowInfo`, filtered by PID.

## Release build

`mygo build --out <dir>` produces:
- `<dir>/darwin-arm64/Gocast.app`
- `gocast-0.1.0-arm64.dmg`

The bundle is ad-hoc signed, and `libghostty-vt.dylib` is bundled in Resources. The default `<dir>` is `build/`.

## Layout

| File | What |
| --- | --- |
| `main.go` | App setup, single instance, global shortcuts (register / suspend while recording / change), settings, quit handling |
| `launcher.go` | Search panel: field, results, actions capsule, ⌘K menu, notices |
| `termpanel.go` | Floating terminal (`plugins/terminal`): tabs, banners, end bar, show/hide |
| `prefs.go` | Settings window: Shortcuts, Terminal Presets (editor), General, About |
| `tray.go` | Tray icon and menu |
| `theme.go` | design.md 1.2 color tokens, keycaps |
| `assets/tray.png` | Tray icon; `assets/make-tray-icon.swift` draws it |
| `internal/apps` | App scan, localized names |
| `internal/search` | Matching: substring, word initials, pinyin (full and initials), ranking |
| `internal/usage` | Frecency, and which result a query chose |
| `internal/presets` | presets.json: IDs, the default preset, validation, env |
| `internal/settings` | settings.json |
| `internal/shortcut` | Accelerator ⇄ glyphs (⌥⇧Space), and the recorder's key → accelerator |
| `internal/running` | Running apps (the dot under their icons) |
| `locales/` | MyGo i18n catalogs: `en` (source) and `zh-Hans`; they follow the system language |

## Data

These files are in `~/Library/Application Support/dev.gocast.app/`:
- `presets.json`. It is edited in Settings; editing it by hand works too, and it is reloaded when the panel or Settings opens. There are no presets on first run. The first terminal is then a plain shell, with the tip that points to Settings.
- `settings.json`. It holds the shortcuts, font size and keep-terminal-on-blur, and only stores values that differ from the defaults.
- `usage.json`. Frecency.

Commands run through the user's login shell, as a login and interactive shell, so their PATH applies.

## Shortcuts

| Action | Shortcut |
| --- | --- |
| Search panel | **⌥⇧Space** (fallback ⌘⌥⇧Space if another app has it) |
| Floating terminal | **⌃`** (fallback ⌃⌥`) |

Both can be changed in Settings → 快捷键, and a change takes effect at once.

## Verified (2026-11-12, MacBook Pro M1 Pro, zh-Hans, dev build driven by mygo-automation)

`gofmt -l`, `go vet ./...`, `mygo vet` and `go test ./...` are all clean.

Screenshots are real window captures, in `.airbuild/artifacts/screens/v2/`.

**Search panel:**
- Compact at 66. Typing expands it, and a capture taken right after the keystroke already shows the results (no blank frame).
- `wx` → 微信 first, and app rows have no subtitle (`0-…`, `1-search-expanded-{light,dark}.png`).
- Preset row with `{folder} · {command}`, middle-truncated, tag 预设, primary action 在悬浮终端打开 (`2-search-preset-light.png`).
- ⌘K menu over the capsule (`3-search-actions-{light,dark}.png`).

**Floating terminal:**
- Opening the pi preset from search shows the panel at the top with pi running (`4-terminal-pi-{light,dark}.png`).
- ⌘T adds a plain shell tab (`4b-terminal-shell-tab-dark.png`).
- I hid the panel (收起) and showed it again. Both tabs were still there, and the shell still had its earlier output.
- A preset whose command ends shows 命令已结束, 关闭标签 and 重新运行 (`5-terminal-finished-light.png`).
- A plain shell that exits closes its tab, as designed.

**Settings** (`6-…`–`10-settings-*.png`):
- **Shortcuts:**
  - Recording saves at once, and the new shortcut takes effect.
  - Esc cancels.
  - A clash with the other row shows the red border, the dot and the message, and keeps the old shortcut.
  - 恢复默认 shows only when the value isn't the default.
- **Presets:**
  - List with the 默认 badge, and the empty state.
  - The editor's errors: empty name, and a name already used (case-insensitive).
  - Enter saves, Esc cancels.
  - Env and the default flag are saved.
  - The delete confirmation works, and Esc returns from it to the editor.
- **General:** the font stepper is saved, and open terminals change size live.
- **Other:** About reads "版本 dev" when unpackaged, and the sidebar works with ↑/↓. ⌘W hides the window, and it reopens on the last page.
- **Material:** the sidebar is on the sidebar material, confirmed in a capture of the real window.

The earlier skeleton checks still hold (`.airbuild/artifacts/screens/`):
- Quitting ends the child processes.
- pi uses the full panel width.

## Deviations from design.md

1. **Terminal drop (1.5, 3.2).** AppKit keeps a window's top under the menu bar; a frameless window is a titled one underneath, so `constrainFrameRect` applies. The panel can't slide in from above the screen.
   - Showing: it drops the 8 DIP from the work area's top edge while fading in (200 ms ease-out).
   - Hiding: the reverse (150 ms ease-in).
   - With Reduce Motion it only fades.

   This is the opacity fallback that design.md 1.5 allows. Before this fix (in 17f5b93) it moved only those 8 DIP and didn't fade.
2. **Tray icon (1.6).** `command.square` is drawn ahead of time by AppKit into `assets/tray.png` and embedded. `sfsymbols.Image` returns a `ui.Bitmap` whose pixels can't be read back, and the tray only takes PNG bytes. The drawn magnifier is the fallback.
3. **Delete confirmation (4.4).** It is drawn inside the editor's modal, not as a `ui.AlertDialog`, because AlertDialog has no destructive style.
   - 删除 has `danger` text.
   - 取消 is focused and is the default, as for destructive choices on the Mac.
   - Esc goes back to the editor.
4. **Settings sidebar (4.2).** It is a custom list, not `ui.Sidebar`, because `SidebarItem` takes SVG icons and not SF Symbols. It keeps Sidebar's behavior: one Tab stop, ↑/↓ choose the page, and the accent-colored selection.
5. **Preset editor (4.4).** It is a `ui.Modal` with a `ui.Form`; MyGo has no `ui.Dialog` widget.
6. **List bottom fade (2.3).** Not done. MyGo has no mask, and design.md says this is not a defect.
7. **Empty query (2.6).** The list of suggestions (frecency) is built but off (`suggestionsEnabled = false` in launcher.go) until decided. Compact is the default.
8. **Tray menu.** 设置… shows ⌘, on the right as well, the Mac's usual Settings shortcut; it also works in the search panel.

## Manual tests still needed

Automation can't cover these:
- **Global shortcuts** on a real keyboard (automation sends keys to the window, not through the system hot keys), and shortcut recording on a real keyboard.
- **IME:**
  - composing Chinese in the search field: nothing is acted on while composing, and Enter commits;
  - the candidate window position in the field and in the terminal.
- **⌘V image paste into pi** (plugins/terminal's Kitty 5522 / empty bracketed paste).
- **Tray:** clicks on the icon and menu (unreachable for automation), and the icon itself in the menu bar.
- **Folder picker** (系统选文件夹) in the preset editor.
- **System prompts:** the login-item prompt for 登录时自动启动.
- **Update check:**
  - a real update, with Install and Relaunch;
  - in dev it always reads 检查失败, because updates are disabled.
- **Accessibility settings:** Reduce Transparency (the fallback to `surface`), Increase Contrast, and Reduce Motion (fades only).
- **English UI** (system language en).

## Open issues

- **Updates feed.** None is configured (`updates` in mygo.json). Until it is, packaged builds also read 现在检查不了更新…
- **Platform:**
  - `minimumSystemVersion` is 13.0, MyGo's floor (brief v1.3 agrees).
  - `mygo build` makes arm64 only. The universal build is planned for the second release (brief).
- **Signing.** There is no Developer ID on this Mac, so the bundle is ad-hoc signed and not notarized.
- **App icon.** None yet (design.md 1.6). About uses `command.square` 48 as a placeholder.
- **`mygo build` locales warning.** It warns "The app does not import gocast/locales". This is a false positive and harmless: MyGo's `compiledDirs` splits `go list` output with `strings.Fields`, which breaks on the space in "Application Support". It should be reported upstream.
- **Unbundled app names.** An unbundled binary gets app names in English from Launch Services; `Localized(App.Locale())` corrects them.
