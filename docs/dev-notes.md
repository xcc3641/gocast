# Gocast dev notes

Gocast v1 (0.1.0). It is written in Go with MyGo's native UI (the `ui` package, no webview).
- Module `gocast`, bundle id `com.hugo.gocast` (earlier builds: `dev.gocast.app`).
- MyGo `v0.9.1-0.20261111182651-0d644f4aca24` (the real module version, no `replace`).

The sources, in priority order:
- `docs/design.md` (wins on any conflict)
- `docs/copy.md` v1.10 (every string is in `locales/`)

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

**Language.** `MYGO_LOCALE=en` (or `zh-Hans`) sets the UI language for one run and saves nothing. This is MyGo's own variable, the same one `mygo dev -lang` uses. Use it to check the longer English strings.

**Automation:**
1. `go build -o build/gocast . && MYGO_AUTOMATION=/tmp/gocast-auto/app.sock ./build/gocast`
2. `mygo-automation open --socket /tmp/gocast-auto/app.sock`

The windows are `w1` the search panel, `w2` the floating terminal and `w3` Settings. `./build/gocast` run a second time shows the panel again. ⌘, in the panel opens Settings.

The automation plugin is always imported but listens only when `MYGO_AUTOMATION` is set. `mygo build` leaves it out (`mygo_noinspector`).

The single-instance lock is keyed by the app's name. An unpackaged build (`gocast`) therefore runs alongside an installed `Gocast.app`. The two share `~/Library/Application Support/com.hugo.gocast/`.

For real window captures, `screencapture -x -o -l <CGWindowID>` works. Get the ID from `CGWindowListCopyWindowInfo`, filtered by PID.

## Release build

`mygo build -o <dir>` produces:
- `<dir>/darwin-arm64/Gocast.app`
- `gocast-0.1.0-arm64.dmg`

Release builds are signed with the Developer ID in `macos.signingIdentity` (mygo.config.ts) under the hardened runtime; `-sign -` builds ad-hoc. Contents/Resources holds `libghostty-vt.dylib` and, through `resources` in mygo.config.ts, `THIRD_PARTY_NOTICES.md`, `LICENSE` and `NOTICE`. The default `<dir>` is `build/`. With `updates` in mygo.config.ts the output also has `install.sh` and `SHA256SUMS`; without `MYGO_UPDATER_PRIVATE_KEY` it skips the update files ("Not signing an update"), which local builds don't need.

**App icon (design.md 1.6).** mygo.config.ts's `icon` is `icon/icon.png` (1024) and `macos.icon` is `icon/AppIcon.icon` (Icon Composer). With Xcode 26's `actool`, `mygo build` compiles the `.icon` into `Assets.car` (`CFBundleIconName` AppIcon, which macOS 26 draws with its glass, dark and tinted looks) and uses the `AppIcon.icns` that `actool` renders from it; without it (or if it fails) the `.icon` is skipped and the `.icns` is made from `icon.png`. `go run` / `go build` have no icon (no bundle). All the icon files come from `icon/make-icon.py` (the designer's; change the shapes there, never the PNGs by hand).

**Third-party notices.** `THIRD_PARTY_NOTICES.md` is generated: run `go run scripts/gen-third-party-notices.go` after any change to go.mod (CI runs it with `-check`). It reads the license files of the compiled modules from the module cache, the license texts of libghostty-vt's components from `scripts/notices/`, and stops when MyGo bundles a libghostty-vt from another Ghostty commit than those texts describe.

## Layout

| File | What |
| --- | --- |
| `main.go` | The entry point: it only calls `app.Run` (internal/app) |
| `internal/app/app.go` | App setup, single instance, global shortcuts (register / suspend while recording / change), settings, quit handling |
| `internal/app/launcher.go` | Search panel: field, results, actions capsule, ⌘K menu, notices |
| `internal/app/termpanel.go` | Floating terminal (`plugins/terminal`): tabs, banners, end bar, show/hide |
| `internal/app/prefs.go` | Settings window: Shortcuts, Terminal Presets (editor), General, About |
| `internal/app/tray.go` | Tray icon and menu |
| `internal/app/updates.go` | Updates without a window: the checks and installs (`mygo.Updater`), the search panel's new version pill, About's status line |
| `internal/app/theme.go` | design.md 1.2 color tokens, keycaps |
| `internal/app/helpers_test.go` | `TestMain` and the fakes the tests share (running apps, global shortcuts) |
| `icon/` | The icons, all drawn by `make-icon.py`, and `icon.go`, which embeds the PNGs the app draws: `AppIcon.icon` and `icon.png` (the app's, also About's at 64), `tray.png` (tray, template), `logo-template.png` (Gocast's mark in the search bar and the terminal's tab bar, tinted `textSecondaryOnGlass`) |
| `CHANGELOG.md` | Keep a Changelog; the version's section is the update's release notes |
| `scripts/gen-third-party-notices.go` | Writes `THIRD_PARTY_NOTICES.md`; `scripts/notices/` holds the texts that are not in the module cache |
| `internal/apps` | App scan, localized names |
| `internal/search` | Matching: substring, word initials, pinyin (full and initials), ranking |
| `internal/usage` | Frecency, and which result a query chose |
| `internal/presets` | presets.json: IDs, the default preset, validation, env |
| `internal/settings` | settings.json |
| `internal/shortcut` | Accelerator ⇄ glyphs (⌥⇧Space), and the recorder's key → accelerator |
| `internal/running` | Running apps (the dot under their icons) |
| `locales/` | MyGo i18n catalogs: `en` (source) and `zh-Hans`; they follow the system language |

## Data

These files are in `~/Library/Application Support/com.hugo.gocast/`:
- `presets.json`. It is edited in Settings; editing it by hand works too, and it is reloaded when the panel or Settings opens. There are no presets on first run. The first terminal is then a plain shell, with the tip that points to Settings.
- `settings.json`. It holds the shortcuts, font size and keep-terminal-on-blur, 自动检查更新 (`autoUpdate`) and the time of the last update check (`updatesCheckedAt`), and only stores values that differ from the defaults.
- `usage.json`. Frecency.

The builds with the earlier bundle ID kept them in `dev.gocast.app/`. At launch, after the single-instance lock and before anything else touches the folder, `migrateDataDir` (migrate.go) copies that folder to `com.hugo.gocast/` when the new one does not exist: modes, modification times and symbolic links kept, into `.com.hugo.gocast.migrating/` first and renamed into place when complete (a copy cut short is removed at the next launch, which copies again). The old folder is never changed. When the copy fails, Gocast logs it and starts with a new, empty folder, the old one still there.

The builds that used MyGo's updater plugin kept 自动检查更新 in the plugin's `updater.json` (`automaticChecks`, and `lastCheck`), in MyGo's `PathUserData`, which is Application Support joined with the app's *name*: `~/Library/Application Support/Gocast/updater.json`. The first launch whose `settings.json` has no `autoUpdate` reads it once (`settings.MigrateUpdater`): `automaticChecks` false turns the switch off, anything else (no file, no field, a damaged file) leaves it on, and `lastCheck` becomes `updatesCheckedAt`; then it saves `settings.json` and never reads the file again. The file itself is left as it is.

Commands run through the user's login shell, as a login and interactive shell, so their PATH applies.

## Shortcuts

| Action | Shortcut |
| --- | --- |
| Search panel | **⌘Space**; ⌥⇧Space while Spotlight has ⌘Space, or if another app has it (then ⌘⌥⇧Space if that one is taken too) |
| Floating terminal | **⌃`** (fallback ⌃⌥`) |

Both can be changed in Settings → 快捷键, and a change takes effect at once.

### Keycap and capsule alignment (measured on 2× captures)

Each value below is the offset of the glyph's ink center from the center of its key, in DIP. Positive means low. The keys and segments themselves are exactly centered in the 36-high capsule: the box center is at 993.0 px and the capsule center at 993 px.

| Glyph | Before | After |
| --- | --- | --- |
| 12 pt in a capsule: ⌘, K | +0.75 | +0.25 |
| 12 pt in a capsule: ↵ | −0.75 | −0.25 |
| 12 pt in the ⌘K menu: ⌘, ⇧ | +0.75 | +0.25 |
| 12 pt in the ⌘K menu: C | +0.5 | 0 |
| 12 pt in the ⌘K menu: ↩ | −0.75 | −0.25 |
| 13 pt in the recorder: ⌘, ⌥, ⇧ | +0.25 | −0.25 |

The text's line box was centered, which leaves baseline glyphs low and arrows, which sit on the math axis, high. `capNudge` moves each kind by half the measured offset, and a ±0.25 DIP difference is the half pixel that rounding leaves. ⌃ and \` stay high, as in the system's menus. The capsule labels 打开 and 操作 (13 pt) sit +0.25 and +0.5 DIP low and were left as they are.

### ⌘Space and Spotlight (design.md 4.4, copy.md v1.5 4.1)

- **Check.** `shortcut.SpotlightHasCmdSpace` reads `defaults export com.apple.symbolichotkeys -` and extracts `AppleSymbolicHotKeys.64` (“Show Spotlight search”) with `plutil`. It only reads. ⌘Space counts as Spotlight's when the entry is enabled with key code 49 and the ⌘ mask (1<<20), and also when the entry is missing or can't be read, since that is the system default.
- **Why a check is needed.** Carbon's `RegisterEventHotKey`, which MyGo uses, refuses a shortcut that another app registered (its error is “may be used by another application”), but it does not refuse Spotlight's symbolic hot key, so both would open. Each launch checks again: while Spotlight has ⌘Space, ⌥⇧Space is registered instead, and the row says 「⌘Space 被聚焦搜索占用」.
- **First launch** (`spotlightChecked` not yet in settings.json): if Spotlight has ⌘Space, Settings opens on 快捷键 with the notice at the top, instead of the search panel. Later launches don't open it.
  - 「打开系统设置」 opens `x-apple.systempreferences:com.apple.preference.keyboard?Shortcuts`. The Keyboard extension allows the URL scheme and maps that legacy ID; whether it lands on the Shortcuts sheet itself has not been checked.
  - Each time Gocast becomes active (`App.OnDidBecomeActive`) it checks again. If ⌘Space is free it takes it, the notice turns into 「已改用 ⌘Space」 for 3 s and then fades out over 150 ms.
  - 「就用 ⌥⇧Space」 saves `launcherDefaultAltShiftSpace: true`. ⌥⇧Space becomes the default from then on, Reset to Default included, and the notice is not shown again.
- **Recording ⌘Space** while Spotlight has it is refused in the same way as a conflict: red border, dot and 「⌘Space 被聚焦搜索占用」.
- **Another app holding the shortcut at launch** (registration fails): ⌥⇧Space is used, and the row shows the usual conflict message.
- **Trying it** without touching the Mac's settings: set `GOCAST_SPOTLIGHT_ENTRY=/path/entry.json`, a file holding the entry as JSON, e.g. `{"enabled":true,"value":{"parameters":[32,49,1048576]}}`. Edit the file, then bring Gocast to the front again.

### Widths at 640 (design.md 2.1–2.6, 3.2)

- **Panel.** The search panel is 640 wide. The window is the panel plus its shadow margins.
- **Result rows.** A preset's subtitle is at most 40% of the 624-wide row (`listNoteMax`, 249.6) and is cut in the middle. The title shrinks first, down to 160 or to its own width if that is less (`listTitleLeast`, measured with `MeasureText`), and is cut at its end. Only after that does the subtitle shrink. The empty-query list caps the folder on the right the same way and cuts it in the middle.
- **Capsules.**
  - The actions capsule never shrinks. `actionsCapsuleWidth` measures its texts, and the notice capsule gets `noticeRoom` = window width − actions capsule − 24 and is cut at its end. The full text goes into the notice's label and tooltip. The terminal's notice capsule uses the same rule.
  - The computed width matches the laid-out width exactly: 331.19 for "Open in Floating Terminal ↩ · Actions ⌘K". Design.md estimates about 315. Each outline key is its text + 10 + a 2 border, at least 22.
- **⌘K popup.** Still 320 wide. A name grows to fill the row and is cut at its end, the keys never shrink, and the gap between name and keys is 8 + 4 = 12.
- **No-results line.** It is split around the query, using `queryMark` in the catalog string. Only the query shrinks, and the line is at most 544 (640 − 96).
- **Terminal size.**
  - Defaults are 50% wide and 60% high. Height runs 30–90. The minimum is still 640×300 px.
  - The width's least is `settings.MinTermWidth(workArea)` = max(30, ⌈640 / work-area width × 100⌉): 43 on a 1512-wide display, 30 on a 2560-wide one. The work area is that of the display the terminal is on, or of the one under the mouse when it is hidden (`termPanel.workAreaWidth`).
  - The slider's track starts at that least. The stored `terminalWidthPercent` stays as set (30 stays 30 in settings.json) and is shown and used as max(stored, least) (`Settings.TermWidthOn`), so a wider display gets it back. On a 1512-wide display, 43% is 650, not 640.
  - Dragging an edge clamps the width to the least too and saves the rounded percent.

### Right line (design.md 2.1a, measured on 2× captures)

Whatever is at the search panel's right ends on one line 20 from its right edge (`rightLine` in kit.go; `TestRightLine` checks the sums).

| Item | Sum | Before (ink, DIP from the edge) | After |
| --- | --- | --- | --- |
| Result row tag 应用 / App | list 8 + row 12 (`listRowPadR`) | 21.5 / 20.5 | 20.5 / 19.5 |
| Empty-query list keycaps (⌃\`, ⌘,) | the same | 20 | 20 |
| Empty-query list folder (~, a project folder) | the same | 20.5 / 21.5 | 19.5 / 20.5 |
| Capsule's last key K | inset 8 + border 1 + `capsulePadR` 3 + `segmentPadR` 8 (was 6) | 18 | 20 |
| ⌘K popup keys | inset 8 + `menuPad` 6 + `menuRowPadR` 6 (was 8; the left stays 8) | 22 | 20 |
| ⌘K popup filter field | inset 8 + `menuTextInsetR` 12 | 22 (frame) | 20 (frame) |
| Search bar spinner | the bar's right padding 20 | 19.5 | 19.5 |

Keys are boxes and end on the line exactly. Text glyphs end inside their advance (应用 and digits 1.5, App and ~ 0.5), so `listNote` reaches `noteBearing` = 1 past the row's padding; every item then ends within 0.5 of the line, in both languages. Left side, for reference: the result icons' ink at 20 from the panel's left (cell at 18), titles at 54; in the popup, the icons at 14 (ink 15), the filter text at 14, the names at 26.

Checked on captures in light mode with the query "t3" and ⌘K open, without the popup, in English, with an empty query in both languages, and on a build with the spinner forced on.

### Compact capsule (design.md 2.4, 2.6, measured on 2× captures)

With the search bar alone (66 high) the 「操作 ⌘K」 capsule is centred in it, 15 above and 15 below, with the same right margin 8, so K stays on the right line. Before, it sat at the bottom-right inset 8, as in the expanded panel, 21.5 above and 8.5 below (its box, from the panel's outer edges).
- `capsuleBottom(h)` in launcher.go takes the panel's height as drawn. At 66 it gives 14.5, counted from inside the panel's 0.5 border, which is where absolute children are placed. From 72.5 up it gives 8, so the expanded capsule is unchanged. In between it moves smoothly: as the panel grows it comes down to the bottom inset, and as it shrinks it rides the bottom edge up to the centre, with no jump.
- In compact mode the segment is no longer dimmed: ⌘K works there (it opens Gocast's own actions). Only the no-results capsule is dimmed (design.md 2.6).
- When ⌘K grows the panel into Gocast's own actions, the capsule is gone. Esc or ⌘K folds the list back, and the capsule comes back as it folds. Before, it stayed hidden until the panel was shown again.
- Measured, light and dark: box edges at 30 px and 102 px of 132, so 15.0 above and 15.0 below. The light rim's outer line is 14.5 / 14.5. The segment's centre is 33.0. K's box ends 41 px from the panel's outer right edge, 20.5, which is 20.0 inside the 0.5 border, the same as before. The expanded capsule is at the same pixel rows before and after.

### Copies and names (design.md 2.3)

- `apps.Unique`: copies with one bundle ID are one app. The one kept is in /Applications or /System/Applications, else ~/Applications, else the first found.
- `apps.Places`: apps of one name (any case) with different IDs all stay. After the name, styled as a preset's subtitle (13 secondary, at most 40%, cut in the middle), each shows its bundle's file name without .app, or nothing when that is its name: Siri (`com.apple.campo`, Siri AI.app) shows 「Siri AI」, Siri (`com.apple.siri.launcher`, Siri.app) nothing. Only those whose file names collide too (any case) show their folder, as Finder names it, or the whole path if they share it. Assistive technology reads it after the name: 「Siri，Siri AI，应用」.
- On a typical Mac no bundle ID has two copies, so the list is the same before and after `Unique`; of the system's apps only the Siri AI row gets a subtitle.
- Background-only (`LSBackgroundOnly`) apps and agents (`LSUIElement`) are listed too; none is left out.

### ⌘K with an empty query (design.md 2.5a, copy.md v1.7)

- In compact mode ⌘K does not open the popup card. The panel itself grows into a list of Gocast's own actions (`gocastActions`, drawn by `ownList`). Its rows, section title and divider are the kit's `listRow`, `listSection` and `listDivider`, which the result rows use too.
- The order is 打开悬浮终端 (the terminal shortcut's keycaps), then 用预设打开 with the presets, then 添加预设, a divider, 设置… (⌘,) and 退出 Gocast. If there are no presets, the section is left out.
- The keyboard stays in the search field. ↑↓ / ⌃N⌃P / ⌃J⌃K move, ↵ or a click runs, and Esc or ⌘K folds the list back to 66. Typing a character ends the list and searches. The list has no filter field and no capsule.
- Height: 66 + rows 40 + section 28 + divider 9 + 8, at most 475 (`ownListHeight`). That gives 351 with 2 presets and 243 with none, both measured on captures.
- The panel no longer takes ⌘Q (design.md 2.5a). Quitting happens from the row or the tray menu.
- With a result chosen, ⌘K still opens that result's popup.

## Verified (0.1.0, Apple silicon, zh-Hans, dev build driven by mygo-automation)

`gofmt -l`, `go vet ./...`, `mygo vet` and `go test ./...` are all clean.

The checks below were made on real window captures (the screenshots are not in the repository).

**Search panel:**
- Compact at 66. Typing expands it, and a capture taken right after the keystroke already shows the results (no blank frame).
- `wx` → 微信 first, and app rows have no subtitle.
- Preset row with `{folder} · {command}`, middle-truncated, tag 预设, primary action 在悬浮终端打开.
- ⌘K menu over the capsule, light and dark.

**Floating terminal:**
- Opening a preset from search shows the panel at the top with its command running.
- ⌘T adds a plain shell tab.
- Hiding the panel (收起) and showing it again keeps both tabs, and the shell keeps its earlier output.
- A preset whose command ends shows 命令已结束, 关闭标签 and 重新运行.
- A plain shell that exits closes its tab, as designed.

**Settings:**
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

**Design review.** On-screen region captures (`screencapture -R`) of the panel's rect over the real desktop, in light mode:
- **Glass.** The glass now has a tint, rgba(246,246,248,0.72) in light mode and rgba(28,28,30,0.72) in dark mode, so the light panel no longer turns gray.
- **应用 tag contrast** (text #5E5E63):
  - Over the real desktop: 6.1:1 on a plain row and 5.1:1 on the selected row.
  - Over a pure black backdrop (worst case): 4.6:1 plain and 3.85:1 selected. The selected row is still under 4.5 there.
- **App icons.** Cell 26, with the row's left padding 10 and a gap of 10, so the title starts at 54. A preset's square is 22, centered in the cell. The running dot is centered at row y 35.
- **Gocast mark.** `logo-template.png` drawn 20×20 (its squircle is 17 seen), in `textSecondaryOnGlass` (#505054 / #AEAEB2 measured on the captures), through a white copy of the template tinted by `ImageOptions.Tint`. In the search bar it sits half a DIP below the row's middle (`Margin(1,0,0,0)`), so its ink centre (34.0 from the panel's top) meets the 20 pt text's: caps 34.25, the Chinese placeholder 34.5.
- **⌘K menu.** Its bottom is 8 above the capsule.

The earlier checks still hold:
- Quitting ends the child processes.
- A full-screen program (e.g. an editor or a TUI) uses the full panel width.

## Deviations from design.md

1. **Terminal drop (1.5, 3.2).** AppKit keeps a window's top under the menu bar; a frameless window is a titled one underneath, so `constrainFrameRect` applies. The panel can't slide in from above the screen.
   - Showing: it fades in while dropping from 12 above its place (180 ms ease-out). It is held at the menu bar for the first frames, so only the last 8 of the 12 show (measured 33→41).
   - Hiding: it fades out while rising 8 (120 ms ease-in), no higher than the menu bar.
   - With Reduce Motion it only fades.
2. **Delete confirmation (4.4).** It is drawn inside the editor's modal, not as a `ui.AlertDialog`, because AlertDialog has no destructive style.
   - 删除 has `danger` text.
   - 取消 is focused and is the default, as for destructive choices on the Mac.
   - Esc goes back to the editor.
3. **Settings sidebar (4.2).** It is a custom list, not `ui.Sidebar`, because `SidebarItem` takes SVG icons and not SF Symbols. It keeps Sidebar's behavior: one Tab stop, ↑/↓ choose the page, and the accent-colored selection.
4. **Preset editor (4.4).** It is a `ui.Modal` with a `ui.Form`; MyGo has no `ui.Dialog` widget.
5. **List bottom fade (2.3).** Not done. MyGo has no mask, and design.md says this is not a defect.
6. **Empty query (2.6).** The list of suggestions (frecency) is built but off (`suggestionsEnabled = false` in launcher.go) until the product decision on it is made. Compact is the default.
7. **Tray menu.** 设置… shows ⌘, on the right as well, the Mac's usual Settings shortcut; it also works in the search panel.
8. **Same-name apps (2.3; a product decision).** Not the folder first: the bundle's file name without .app, and nothing where that is the name (「Siri」 with 「Siri AI」, and plain 「Siri」). Only apps whose file names collide too get their folder, or their whole path if they share it, cut in the middle.
9. **Right line (2.1a).** The gray texts at the right (tags, folders) reach 1 past the row's 12 padding, so that their glyphs, not their advances, end on the line with the keys.

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
  - a real update, downloaded and installed in the background, then Relaunch from the pill, the Gocast group, the tray and About (needs a published release on xcc3641/gocast);
  - the tray's 检查更新… and its 重新打开以更新到 {版本} (automation can't reach the tray; `TestUpdateTrayItem` checks the item).
- **Accessibility settings:** Reduce Transparency (the fallback to `surface`), Increase Contrast, and Reduce Motion (fades only).
- **English UI** (system language en).

## Updates (design.md 4.4, 4.5; copy.md 12)

- **No window.** `updates.go` calls `mygo.Updater.Check` and the update's `Install` itself; the updater plugin is gone, with its window, its strings and `icon/update-icon.png`. `Install` downloads, checks the signature and stages the new bundle, which a helper swaps in once Gocast exits, so a version ready runs at the next launch even without Relaunch. Relaunch is `mygo.App.Relaunch("--relaunched")`: the new Gocast sees the argument and shows nothing as it opens, as at login. A quit canceled (`OnQuitCanceled`) offers Relaunch again.
- **Timer.** With 自动检查更新 on, the next check is due at `nextCheck(last, launched, now)`: 10 s after launch, or a day after the last check (`updatesCheckedAt`, saved as each check begins) when that is later, so a relaunch doesn't check again; a last check in the future (the clock set back) counts for none. `schedule` arms one `time.AfterFunc` for it, posted to the main thread, and checks at once when it is due; it runs again after each check, as the switch changes and on `Power.OnResume` (the timer counts no time asleep). Off, nothing is armed; a check asked for (About, the tray) still checks and downloads. A check asked for while one runs joins it.
- **Ready.** The version installed shows in four places: the pill at the bottom left of the search panel (until its × for this run; a 4 s notice covers it), the Gocast group's first row 重新打开以更新到 {版本} (never chosen first, never scored, found by update/relaunch/更新/重新打开), the tray line in place of 检查更新…, and About's primary 重新打开. A check that finds the same version doesn't download it again; a newer one replaces it.
- **About.** The button (检查更新, dimmed while checking or downloading or when the build can't update; 重新打开 once ready) and a status line 32 high under it, empty or saying 正在检查… / 已是最新版本 (4 s) / 正在下载 {版本}… / {版本} 已就绪 / the failures, with 看看改了什么 linking to the website's changelog (`/zh/changelog/#v{版本}` in Chinese). A background check shows checking, downloading and ready, but not 已是最新版本 nor failures. The tray's 检查更新… opens About and checks there.
- **General.** 自动检查更新 is the second row of the first group, under 登录时自动启动, kept in `settings.json` (`autoUpdate`, on by default). Off, a version downloaded stays ready.
- **Tests.** `updates_test.go`, over `fakeUpdater` (helpers_test.go): fake checks, installs, relaunch, clock and timers; `TestMain` makes the real updater unreachable. `GOCAST_SCREENS=… go test -run TestScreensUpdates ./internal/app` renders the pill, the row and About's states.
- **What About says when 检查更新 is pressed:**
  - `go run` / `go build` (dev): the button is dimmed and the line says 开发版本不会自己更新。 (en: Development builds don’t update themselves.); a bundle that can't update itself says Gocast 没法在当前位置自己更新…
  - `mygo build`, with the feed: it checks `https://github.com/xcc3641/gocast/releases/latest/download/update-darwin-arm64.json`. Until a release is published that is a 404, logged as `updates: …404 Not Found`, and the line says 现在检查不了更新，稍后会自动再试。
- **Feed.** mygo.config.ts has `updates: { publicKey: "7xqUsSnSTZgGQ69hBuh+pm1jexm7Z3Ngl0lQ17EKPRQ=", github: "xcc3641/gocast" }`. The repository must be public, with releases tagged `v0.1.0` …; it doesn't exist yet. `mygo build` needs no key: without one it builds the app and the disk image and skips the update files.
- **Releasing.** The update signing key and the release steps are in RELEASING.md.

## Open issues

- **Platform:**
  - `minimumSystemVersion` is 13.0, MyGo's floor.
  - `mygo build` makes arm64 only. A universal (Intel too) build is planned for a later release.
- **Signing.** Developer ID Application, hardened runtime, library validation on. The disk image is notarized and stapled after `mygo build` (RELEASING.md); `macos.notarize` is not used because it needs a notarytool keychain profile.
- **`mygo build` locales warning.** It warns "The app does not import gocast/locales". This is a false positive and harmless: MyGo's `compiledDirs` splits `go list` output with `strings.Fields`, which breaks when the project's path has a space in it. It should be reported upstream.
- **Unbundled app names.** An unbundled binary gets app names in English from Launch Services; `Localized(App.Locale())` corrects them.

## Terminal theming options

This is what `plugins/terminal` allows, at the MyGo version in go.mod. Sources: `plugins/terminal/{terminal.go,theme.go,ghostty.go,font.go,paint.go,view.go}` and `docs/plugins/terminal.md`.

**Summary:**

| Want | Possible? | How |
| --- | --- | --- |
| Glass showing through the terminal | Yes | `Options.Transparent` |
| A semi-transparent background (a tint) | Yes, indirectly | `Transparent` plus a translucent background on an element under the view. The theme's `Background` alpha is ignored. |
| Foreground, background, cursor, cursor text, selection colors | Yes | `Theme` fields |
| A custom 16-color palette | Yes | `Theme.Palette [16]ui.Color`. Colors 16–255 are always xterm's cube and grays. |
| Ghostty themes | Yes | `GhosttyTheme(name)`, `GhosttyThemes()`, `ParseGhosttyTheme(data)` |
| Separate light and dark themes, switched live | Yes, automatically | `Options.Theme` + `Options.DarkTheme`, chosen each frame from the window's `c.Theme().Dark` |
| Change colors in a running terminal | Yes | `SetTheme(light, dark)`, `SetMinimumContrast(r)` |
| Font family, size, weight, line height, cell width, features | Yes, live | `Font` + `SetFont` |
| Padding | Only at `New` | `Options.Padding{X, Y}`. There is no setter. |
| Cursor shape and blink | Only at `New` | `Options.Cursor` (block, bar, underline) and `NoBlink`. Programs may change the shape. |

**Transparency.** `Options.Transparent` leaves the terminal's background undrawn, so the window's material (or anything under the view) shows through. This works like Ghostty's background-opacity:
- Cells with a background color of their own still draw it.
- A background a program sets with OSC 11 is also drawn, over the whole view.

`theme.apply` passes only RGB to libghostty-vt, so a translucent `Theme.Background` is not possible. For a tint, paint it under the view:

```go
ui.Column(c).Grow(1).Background(k.glassTint).Children(func() { // e.g. rgba(246,246,248,0.88)
	terminal.View(c, term).Grow(1)
})
// with
terminal.New(terminal.Options{Transparent: true, Theme: light, DarkTheme: dark, …})
```

Still set `Theme.Background` to the color the terminal shows on: the tint over the average glass, e.g. `#F6F6F8` / `#1C1C1E`. It is used in three places:
- the text under a block cursor (when `CursorText` is zero);
- the reference for `MinimumContrast` (cells without a background are measured against `Theme.Background`, not against the real glass);
- the light/dark answer to programs (CSI ? 996 n, mode 2031) and OSC 11 queries.

**Colors.** `terminal.Theme`:

```go
type Theme struct {
	Foreground, Background ui.Color
	Cursor, CursorText     ui.Color // zero: foreground / background
	Selection, SelectionText ui.Color // zero: foreground at 0.3 alpha / the cell's own colors
	Search, SearchText, SearchSelected, SearchSelectedText ui.Color // zero: Ghostty's amber/orange
	Palette [16]ui.Color // black, red, green, yellow, blue, magenta, cyan, white, then bright
}
```

`Selection` may be translucent: it is filled over the cells with its alpha. Programs can still change colors (OSC 4/10/11/12) until the terminal resets.

The defaults are `terminal.LightTheme()` and `terminal.DarkTheme()`: MyGo's window background and text with VS Code's palette. Today Gocast uses these, through `Options.Theme == nil`.

**Light and dark.** Give both themes. The view picks `DarkTheme` when the window's theme is dark, every frame, so `GOCAST_APPEARANCE` and a system switch apply live with no code:

```go
opts.Theme, opts.DarkTheme = gocastLight(), gocastDark()
// later, e.g. if the tokens change:
term.SetTheme(gocastLight(), gocastDark())
```

When the scheme flips, programs that asked (mode 2031) are told, as in Ghostty.

**Minimum contrast.** `Options.MinimumContrast` (a WCAG ratio, 1–21; 0 = off) and `term.SetMinimumContrast(4.5)`. A cell's foreground is darkened or lightened toward black or white, keeping its hue, until it reaches the ratio against the cell's background (`Theme.Background` for cells without one). Faint text is then drawn at half alpha, so faint text stays below the ratio.

**Ghostty themes.**
- `terminal.GhosttyTheme("Catppuccin Latte")` returns a new `*Theme`. Several hundred themes are embedded in `ghostty_themes.go` (iTerm2-Color-Schemes, MIT).
- A file in `~/.config/ghostty/themes/<name>` comes first, and an absolute path names a theme file.
- `terminal.GhosttyThemes()` lists the names, for a picker.
- `terminal.ParseGhosttyTheme([]byte)` reads Ghostty's format (`palette = N=#rrggbb`, `background`, `foreground`, `cursor-color`, `cursor-text`, `selection-background`, `selection-foreground`). Missing colors come from Light/DarkTheme.

The result is a plain `*Theme`, so it can be adjusted afterwards (e.g. `th.Cursor = accent`).

**Font.** `terminal.Font{Family, Size, Weight, Features, CellWidth, LineHeight, Thicken}`:
- `Family` is a comma list as `ui.Element.Font` takes it ("monospace" by default; app fonts via `ui.RegisterFont`).
- `Size` defaults to 13.
- `LineHeight` is a multiplier (1.2 = Ghostty's adjust-cell-height 20%).
- Features are OpenType tags ("-calt").
- `term.SetFont(f)` applies live and resets zoom (Gocast already uses it for the font-size setting and ⌘+/−).

**Padding.** `Options.Padding{X, Y}` in DIPs (default 6×4; negative = none). It is fixed at `New`: there is no `SetPadding`. `PaddingColor` (`PaddingBackground` / `PaddingExtend` / `PaddingExtendAlways`) extends cell backgrounds into the padding. With `Transparent` and `PaddingBackground` the padding shows the glass.

To change the room around the grid live (e.g. room for a floating capsule at the bottom), pad or margin the element around `terminal.View`. With `Transparent` it looks the same as terminal padding. The grid resizes either way, so programs get SIGWINCH.

**Cursor.** `Options.Cursor` (`CursorBlock`, `CursorBar`, `CursorUnderline`) and `Options.NoBlink` are fixed at `New`; programs can change the shape (DECSCUSR). Its color is `Theme.Cursor`, which can be the accent: `th.Cursor = accent`, `th.CursorText = th.Background`.

**Not possible:**
- per-cell or whole-terminal opacity other than `Transparent` (no background-opacity number);
- blur or vibrancy of the terminal's own (that comes from the window's `Vibrancy`);
- a palette beyond the 16 colors;
- changing padding or cursor style after `New`.
