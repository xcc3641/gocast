# AGENTS.md

Gocast is a macOS launcher written in Go with MyGo's native UI. Module `gocast`.

## Layout

- `main.go`: the entry point. It only calls `app.Run()`.
- `internal/app/`: the app (package `app`): search panel, floating terminal, clipboard history, calculator, Settings, tray, updates. `helpers_test.go` holds `TestMain` and the fakes the tests share.
- `internal/`: the other packages: `apps`, `search`, `usage`, `presets`, `settings`, `shortcut`, `running`, `clipboard`, `calc`, `sysfmt`.
- `locales/`: the UI text catalogs (`en.json` is the source, `zh-Hans.json`); `mygo generate` writes `catalog_gen.go`. mygo.json's `i18n.dir` points here.
- `icon/`: the icons, drawn by `make-icon.py`; `icon.go` embeds the PNGs the app draws. Never edit the PNGs by hand.
- `scripts/`: `gen-third-party-notices.go`, which writes `THIRD_PARTY_NOTICES.md`, and the license texts it uses.
- `docs/`: `design.md` (the design spec, wins on any conflict), `copy.md` (the UI text), `dev-notes.md`.
- `mygo.json`: the app's configuration. `entitlements.plist`: what the app is signed with.
- `CONTRIBUTING.md` (setup and checks), `RELEASING.md` (cutting a release), `CHANGELOG.md`.

## Tests

Only write a test for behaviour that can actually break. If the code under test could change without the test noticing, or the test would fail on a harmless rewording, don't write it.

**Write tests for:**

- What input does: keys, clicks and typing in the search panel, the floating terminal, the clipboard history and Settings, and the state or effect they lead to (which row is chosen, what opens, which shortcut gets registered, what is saved).
- Logic that doesn't need a view: search and ranking, the calculator, presets, settings and their files, shortcuts and their glyphs, the clipboard store, moving the data folder.
- A regression you just fixed. Say in the test's comment what went wrong.

**Don't write tests that:**

- Check that a list of labels shows on screen. That repeats the view code and breaks whenever the wording changes.
- Check default settings or the order of rows in a form.
- Test MyGo itself, for example that an animation moves from 0 to 1.
- Depend on the wall clock. Pass a fixed time (as `clipNow` does) and move a `ui.Tester` on with `Advance`, rather than sleeping or reading the time twice and hoping.

**Never touch the real system.** No test may open, hide or quit apps, register global shortcuts, read or write the real pasteboard, post key events, change system settings, use the keychain, or read or write `~/Library/Application Support/com.hugo.gocast` (or the old `dev.gocast.app`). A CI runner with no login session crashes on some of these, and a test must not act on the user's Mac.

**How:**

- Everything the app asks the system goes through a package variable (`sysIsRunning`, `sysOpen`, `globalShortcuts`, `fileExists`, the `clip*` functions, ...). Tests swap it for a fake (the clipboard store takes a `memPasteboard`) and restore the real one with `defer` or `t.Cleanup`. Fakes more than one test file needs go in `internal/app/helpers_test.go`, as the running apps (set up in `TestMain`) and `useFakeShortcuts` do. When you add a new call to the system, add its variable and its fake too.
- Give every test its own data folder with `t.TempDir()`.
- Build views with `ui.NewTester` (see `panelTester`, `testLauncher`), set the language with `inLocale`, and check the app's state afterwards.
- Screenshots: the `TestScreens…` tests write PNGs into `GOCAST_SCREENS` (an absolute path; tests run in `internal/app`) and skip when it isn't set. Look at them; don't assert on pixels.
- Start each test with a one-sentence comment saying the behaviour it checks, in the style of the code.

## Before committing

```sh
gofmt -l .          # must print nothing
go vet ./...
go test ./...
```

`mygo vet` and `go run scripts/gen-third-party-notices.go -check` run in CI too. Run a new UI test with `-count=5`. Fix or delete a flaky test; don't retry it.
