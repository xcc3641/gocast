# Changelog

All notable changes to Gocast are documented in this file. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
Gocast follows [Semantic Versioning](https://semver.org/).

The section of each version is the release notes that the update window
shows (MyGo reads `## [version]`), so write it for users, in Chinese and
English.

## [0.1.2] - 2026-11-13

小更新，功能和你的数据都不变。App 里附带的第三方许可说明更新到了现在用的版本。
A small update; features and your data stay the same. The third-party licenses that come with the app are now up to date.

## [0.1.1] - 2026-11-13

小更新，没有新功能。这一版主要用来确认自动更新顺利：更新后你的设置、预设、快捷键和剪贴板历史都会原样保留。
A small update with no new features. It makes sure updating goes smoothly: your settings, presets, shortcuts and clipboard history stay as they were.

## [0.1.0] - 2026-11-13

第一个版本。First release.

### 新增 Added

- 搜索面板：⌘Space 唤出（被聚焦搜索占用时先用 ⌥⇧Space），按中文名、英文名或拼音找应用和终端预设，↵ 打开，⌘K 看更多操作。打开时先列出常用的应用和预设；想要只有搜索框，在设置 › 通用里打开「打开时只显示搜索框」。
  Search panel: ⌘Space (⌥⇧Space while Spotlight has it) finds apps and terminal presets by their Chinese or English name or by pinyin; ↵ opens, ⌘K shows more actions. It opens with your most-used apps and presets; turn on Show only the search field in Settings › General for just the search field.
- 应用和预设的快捷键：给任意应用或终端预设设一个快捷键，在哪儿按都行。没开就打开，在后面就叫到前面，已在最前面再按一下就隐藏；预设在悬浮终端里打开。在设置 › 快捷键的列表里搜到它直接设，或在搜索面板里选中后按 ⌘K。
  App and preset shortcuts: give any app or terminal preset its own shortcut that works from anywhere. It opens the app, brings it to the front, or hides it if it's already in front; a preset opens in the floating terminal. Set one from the list in Settings › Shortcuts, or select it in the search panel and press ⌘K.
- 计算器：在搜索框里直接打算式、单位或汇率（如 `100 美元换日元`），最上面出答案，↵ 拷走。汇率每天联网取一次，断网用上次的；设置里可关掉「在搜索中计算」。
  Calculator: type math, units or currencies in the search panel (like `100 usd in jpy`) and the answer shows up on top; ↵ copies it. Exchange rates are fetched once a day and work offline; turn it off in Settings › General (Calculate in search).
- 悬浮终端：⌃\` 从屏幕上方落下，多个标签，预设在指定文件夹里直接运行命令。
  Floating terminal: ⌃\` drops it from the top of the screen, with tabs; presets run a command in a folder.
- 剪贴板历史：⌃⌘V 唤出（也可在搜索面板里找「剪贴板历史」），记下拷贝过的文字、链接、图片和文件，只存在这台 Mac 上；↵ 直接粘贴到刚才的应用，可置顶、按类型筛选，设置里可关掉、改保留时长、排除应用。
  Clipboard history: ⌃⌘V (or Clipboard History in the search panel) keeps the text, links, images and files you copy, only on this Mac; ↵ pastes into the app you were in; pin items, filter by type, and turn it off, set how long items are kept or exclude apps in Settings › Clipboard.
- 设置：快捷键（Gocast 自己的和每个应用、预设的）、终端预设、剪贴板、通用（登录时自动启动、打开时只显示搜索框、在搜索中计算、自动检查更新、终端字号和大小、失焦时收起）、关于。
  Settings: shortcuts (Gocast's own, and one per app or preset), terminal presets, clipboard, General (open at login, show only the search field, calculate in search, check for updates automatically, terminal font size and size, hide on click away) and About.
- 菜单栏图标和菜单，含「检查更新…」。
  Menu bar icon and menu, with Check for Updates….
- 简体中文和英文界面，深色和浅色外观。
  Simplified Chinese and English, light and dark appearance.
