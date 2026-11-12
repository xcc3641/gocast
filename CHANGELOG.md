# Changelog

All notable changes to Gocast are documented in this file. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
Gocast follows [Semantic Versioning](https://semver.org/).

The section of each version is the release notes that the update window
shows (MyGo reads `## [version]`), so write it for users, in Chinese and
English.

## [0.1.0] - 2026-11-12

第一个版本。First release.

### 新增 Added

- 搜索面板：⌘Space 唤出（被聚焦搜索占用时先用 ⌥⇧Space），按中文名、英文名或拼音找应用和终端预设，↵ 打开，⌘K 看更多操作。
  Search panel: ⌘Space (⌥⇧Space while Spotlight has it) finds apps and terminal presets by their Chinese or English name or by pinyin; ↵ opens, ⌘K shows more actions.
- 悬浮终端：⌃\` 从屏幕上方落下，多个标签，预设在指定文件夹里直接运行命令。
  Floating terminal: ⌃\` drops it from the top of the screen, with tabs; presets run a command in a folder.
- 剪贴板历史：⌃⌘V 唤出（也可在搜索面板里找「剪贴板历史」），记下拷贝过的文字、链接、图片和文件，只存在这台 Mac 上；↵ 直接粘贴到刚才的应用，可置顶、按类型筛选，设置里可关掉、改保留时长、排除应用。
  Clipboard history: ⌃⌘V (or Clipboard History in the search panel) keeps the text, links, images and files you copy, only on this Mac; ↵ pastes into the app you were in; pin items, filter by type, and turn it off, set how long items are kept or exclude apps in Settings.
- 设置：快捷键、终端预设、通用（登录时自动启动、自动检查更新、终端字号和大小、失焦时收起）、关于。
  Settings: shortcuts, terminal presets, General (open at login, check for updates automatically, terminal font size and size, hide on click away) and About.
- 菜单栏图标和菜单，含「检查更新…」。
  Menu bar icon and menu, with Check for Updates….
- 简体中文和英文界面，深色和浅色外观。
  Simplified Chinese and English, light and dark appearance.
