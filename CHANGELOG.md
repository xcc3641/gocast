# Changelog

All notable changes to Gocast are documented in this file. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
Gocast follows [Semantic Versioning](https://semver.org/).

The section of each version is the release notes that the update window
shows (MyGo reads `## [version]`), so write it for users, in Chinese and
English.

## [0.2.0] - 2026-11-19

- 设置 › 悬浮终端的「出现位置」改成一排六张小图：顶部、中央、底部、左侧、右侧、鼠标处，每张都画出终端会在哪儿，点哪张选哪张。
- 「鼠标旁边」改成「鼠标处」：唤出悬浮终端时，终端里正在输入的那一行对准鼠标。跑 agent 时就是底下的输入框，眼睛不用找。放不下时贴着屏幕边，不会出屏。

- Position in Settings › Floating Terminal is now a row of six pictures: Top, Center, Bottom, Left, Right and At Pointer. Each one shows where the terminal goes; click the one you want.
- Near Pointer is now At Pointer: when the floating terminal appears, the line you’re typing on lines up with the pointer. With an agent running, that’s its input box at the bottom, right where you’re looking. If there’s no room, it stays inside the screen.

## [0.1.9] - 2026-11-19

- 悬浮终端可以选在哪儿出现：顶部居中（和以前一样）、屏幕中央、底部居中、左侧、右侧，或者鼠标旁边。在设置 › 悬浮终端的「出现位置」里选。
- 选「鼠标旁边」时，终端出现在鼠标下方，下面放不下就到上方。
- 「出现位置」旁边有一个小屏幕示意图，换选项时能看到终端会摆在哪儿。
- 修好了笔记往下滚时，文字会压到窗口顶栏上的问题。

- Choose where the floating terminal appears: top center (as before), center, bottom center, left, right, or near the pointer. Find it under Position in Settings › Floating Terminal.
- Near Pointer opens the terminal just below the pointer, or above it if there’s no room.
- A small picture of your screen next to Position shows where the terminal will go as you change it.
- Fixed text in a note showing over the window’s top bar when you scroll.

## [0.1.8] - 2026-11-17

- 新增笔记：在搜索里选「笔记」，会浮出一个笔记窗口，随手就能记。第一行就是标题，一次看一篇。
- 写 Markdown 边写边排版：标题、粗体、列表、待办、引用、代码、链接都直接显示成排好的样子，⌘B、⌘I、⇧⌘9 这些快捷键和 Raycast 一样。
- 按 ⌘P 浏览和搜索所有笔记，标题和正文都能搜到。常看的按 ⇧⌘P 置顶，再用 ⌘1 到 ⌘9 直接打开。
- 停笔就自动保存，每篇一个 Markdown 文件，只存在这台 Mac 上。删掉的笔记进废纸篓，能找回来。
- 「笔记」「新建笔记」「搜索笔记」都能在设置 › 快捷键里设快捷键。

- New: Notes. Choose Notes in search and a notes window floats up, ready to write in. The first line is the title, and you see one note at a time.
- Markdown formats as you type: headings, bold, lists, to-dos, quotes, code and links show up formatted right away, and shortcuts like ⌘B, ⌘I and ⇧⌘9 match Raycast.
- Press ⌘P to browse and search all your notes by title or text. Pin the ones you use with ⇧⌘P, then open them with ⌘1 to ⌘9.
- Notes save as soon as you stop typing, one Markdown file each, and stay on this Mac. Deleted notes go to the Trash, so you can get them back.
- Notes, New Note and Search Notes can each have a shortcut in Settings › Shortcuts.

## [0.1.7] - 2026-11-16

- 悬浮终端不再跟着 Gocast 停：退出、崩溃或更新以后，终端里正在跑的命令接着跑，标签和输出都还在。想全部停掉，在菜单栏点「结束所有终端」。
- 在悬浮终端里按住 ⌘，会浮出终端里能用的快捷键，标签上也标出 1 到 9。
- 搜索里能直接做系统操作：锁定屏幕、睡眠、关闭显示器、调音量、切换深浅色、清倒废纸篓、推出磁盘等 24 条。重新启动、关机这类会先问一句。
- 也能摆窗口：左右半屏、四分之一、三分之一、最大化、居中、移到另一块屏幕等 22 条。第一次用要在“辅助功能”里打开 Gocast。
- 这些都能在设置 › 快捷键的「系统与窗口」里设快捷键。

- The floating terminal no longer stops with Gocast: after a quit, crash or update, running commands keep going, and your tabs and output are still there. To stop them all, choose Close All Terminals from the menu bar.
- Hold ⌘ in the floating terminal to see its shortcuts, with tabs numbered 1 to 9.
- Run system actions from search: lock the screen, sleep, sleep displays, change the volume, switch between light and dark, empty the Trash, eject disks and more, 24 in all. Restart, Shut Down and the like ask first.
- Arrange windows too: halves, quarters, thirds, maximize, center, move to another display and more, 22 in all. The first time, turn on Gocast in Accessibility.
- Give any of them a shortcut under System & Window in Settings › Shortcuts.

## [0.1.6] - 2026-11-13

- 预设的命令结束后（包括按 ⌃C），这个标签直接变回普通终端，停在预设的文件夹里，可以接着敲。正常结束不留字，出错才留一行灰字。
- 可以给悬浮终端设一个默认文件夹，新开的终端从这里开始；没填文件夹的预设也用它。
- 设置里有了单独的「悬浮终端」页：默认文件夹、字号、大小这些终端设置和预设都在这里，通用页更清爽了。
- 搜索结果里应用图标下面不再有表示正在运行的小点。

- When a preset’s command ends, including with ⌃C, the tab turns into a plain terminal in the preset’s folder, so you can keep typing. Nothing is printed when it ends normally; a grey line appears only if it fails.
- Set a default folder for the floating terminal: new terminals start there, and so do presets without a folder.
- The floating terminal has its own page in Settings, with the default folder, font size, size and presets together. General is shorter.
- App icons in search results no longer have a dot showing they are running.

## [0.1.5] - 2026-11-13

修好了剪贴板历史里看图时会先撑到原尺寸再缩回去的问题，现在一选中就是原本该有的大小。
Fixed an image in clipboard history briefly appearing at full size before shrinking: it now opens at its final size.

## [0.1.4] - 2026-11-13

剪贴板历史里，截图软件拷来的图片现在能直接看到了：列表里是缩略图，右边显示整张图，下面多一行尺寸。这类条目仍算「文件」，粘贴出去还是文件。图是从原文件读的，没有另存一份，原文件删掉或挪走后就看不到了。
Images you copy as files, the way screenshot apps do it, now show up in clipboard history: a thumbnail in the list, the whole picture on the right, and its dimensions underneath. They still count as files. The picture is read from the original file and not stored separately, so it’s gone once that file is moved or deleted.

## [0.1.3] - 2026-11-13

更新不再弹窗口了。Gocast 会在后台把新版本下好，只在搜索面板左下角、⌘K 里和菜单栏轻轻提示一句「重新打开」；点了几秒内换上，不点也没关系，下次退出或重启 Mac 就是新版。关于页的「检查更新」也直接在那里显示结果。
Updates no longer ask first. Gocast downloads the new version in the background and only mentions it in the search panel, ⌘K and the menu bar: relaunch when you like, or it switches over the next time Gocast quits. Check for Updates in About shows its result right there.

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
