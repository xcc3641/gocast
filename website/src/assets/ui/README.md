# Drawn-window assets (exported by Pluso from the running system, 2026-11-13)

- `icons/*.png` — 128×128 app/file icons, from NSWorkspace (the same call Gocast uses):
  wechat, safari, vscode, notes, calendar, finder, weread (微信读书), wireless-diagnostics (无线诊断), chatgpt, pdf.
- `sym/<name>-<pt>[-semibold]@2x.png` — SF Symbols, white on transparent, (2·pt)×(2·pt) px.
  Drawn exactly as Gocast's symbol renderer does: glyph scaled to 85% of a pt×pt box and centered,
  so use each as `mask-image` on a pt×pt box (`mask-size: 100%`), color from CSS.
- `landscape-546x364.png` — clipboard preview (show at 273×182). `landscape-thumb-52.png` — row thumbnail (26×26, center-cropped like the app).
- `logo-template.png` — Gocast mark (copy of /icon/logo-template.png), use as mask at 20×20.
- Keycap glyphs (⌃ ⌥ ⌘ ⇧ ↩ ` Space) are plain text in the app — no images needed.
