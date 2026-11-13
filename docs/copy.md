# Gocast 界面文案 v1.20（搜索面板 · 悬浮终端 · 设置）

v1.20 改动：11.3 设置 › 快捷键改成一整张能搜的列表（照 Raycast）：搜索框、四个筛选、行里直接设、撞了可以「改用在这里」。去掉「添加应用…」「选择预设…」和原来的空状态。11.4 补设置列表的读屏（含「未设快捷键」）；「已设快捷键」的空状态改成说应用或预设，因为 Gocast 三项一直列在上面。

v1.19 改动：新增第 11 节「给应用和预设设快捷键」（⌘K 菜单、面板里录制、设置 › 快捷键「应用和预设」分组、找不到应用、结果行键帽的读屏）。仍叫「快捷键」，不跟 Raycast 叫热键。

v1.18 改动：计算器加 7.6 单位和币种的中文全名（公里、公斤、港币、澳元、加元；数据单位写清 1000 还是 1024）。读屏补「还没打完」那句。

v1.17 改动：计算器跟 design.md 2.3a 对齐。卡片上不放「计算器」标记，只留在读屏里。补两句：算不出时「算不出来」，第一次取汇率时「正在获取汇率…」。

v1.16 改动：新增第 7 节「计算器」（卡片、小字、汇率日期、没汇率时的提示、⌘K、设置开关、读屏、README 草稿）。统一用词加「计算器」「答案」「汇率」。

v1.15 改动：搜索面板打开就带推荐（2.2a）。两组叫「推荐」和「Gocast」，不叫「命令」。原来空输入 ⌘K 的「Gocast 操作」列表并进 Gocast 组，读屏跟着改。设置 › 通用加开关「打开时只显示搜索框」（不叫紧凑模式）。

v1.14 改动：剪贴板历史不加密了。设置里的说明和 README 改成「只存在这台 Mac 上，不会上传」，不提加密。「读不出历史」只剩文件坏了这一种情况，文案不变。
v1.13 改动：参考 Tinycast 的三点。「粘贴到“{应用名}”」就是粘贴去向，去向是悬浮终端时写「粘贴到悬浮终端」。往终端贴图片时主操作不改名，README 说一句。README 已写入剪贴板历史（6.11）。

v1.12 改动：第 6 节跟 design.md 对齐。补「读不出剪贴板历史」，以及这时的清空确认。拷贝或粘贴后面板直接收起，所以去掉「已拷贝」提示。剪贴板快捷键只放在设置 › 快捷键。入口那一行的主操作是「打开」。
v1.11 改动：新增第 6 节「剪贴板历史」（入口、视图、操作、权限提示、设置页、清空确认、读屏）。统一用词加「剪贴板历史」「置顶」「拷贝」。菜单栏和设置 › 快捷键各加一行。
v1.10 改动：例子里的个人项目名换成通用的 `~/Projects/my-app`。
v1.9 改动：接上 MyGo 更新器。关于页只留「检查更新」按钮；新版本、已最新、失败都改由更新窗口来说（安装更新 / 稍后提醒 / 跳过这个版本）。通用里加「自动检查更新」开关。窗口文案去掉感叹号，跟全应用语气一致。
v1.8 改动：同名应用的副标题和读屏。两个 Siri 不再读成一样。
v1.7 改动：空输入 ⌘K 是面板自己展开的列表，没有「搜索操作…」过滤框；整张列表读屏叫「Gocast 操作」。
v1.6 改动：搜索框空着时 ⌘K 弹出 Gocast 自己的操作（打开悬浮终端、用预设打开、添加预设、设置…、退出 Gocast）。有结果选中时仍用原来那一组。
v1.5 改动：设置·快捷键页顶上补「⌘Space 被聚焦搜索占用」提示（标题、说明、打开系统设置 / 就用 ⌥⇧Space、成功后「已改用 ⌘Space」、录制框下一行）。默认唤出键改为 ⌘Space。
v1.4 改动：设置·通用「悬浮终端」加一行分组说明（拖终端边缘改大小，这里会跟着变）；宽度、高度两行都只留「占当前屏幕的百分之几」。底边改的是高度，宽度是拖左右两边，不能写在宽度那一行。提示胶囊的 × 读屏用「关闭」。
v1.3 改动：设置·通用补上悬浮终端的宽度、高度。
v1.2 改动（回应 design.md 搜索面板改版）：结果行读屏加上「正在运行」；打开失败提示改在左下角胶囊里，用词不变。
v1.1 改动（回应 design.md 第 9 节）：新增 2.1 底栏、2.2 ⌘K 菜单、3 节「编辑预设」按钮、4.3「失焦时收起悬浮终端」、4 节侧边栏名称确认。另把 ⌘K 里的「新开标签」统一成「新标签」，和悬浮终端里一致。

名字：Gocast。一句话：按一下，应用和终端都到。
给：设计（排版）、开发（接入）。简体中文为主，英文对照放在每条后面。`{x}` 是程序填入的内容。

## 统一用词（全应用只用这一个说法，不混用）

| 概念 | 中文 | English | 不要用 |
|---|---|---|---|
| 搜索框那块浮层 | 搜索面板 | Search panel | 启动器、Spotlight |
| 浮在最上层的终端 | 悬浮终端 | Floating terminal | 热键窗口、Quake 终端 |
| 在某个文件夹跑某条命令的存档 | 预设 | Preset | 配置、模板、Profile、会话 |
| 悬浮终端里的一页 | 标签 | Tab | 会话、窗口 |
| 唤出或收起的按键 | 快捷键 | Shortcut | 热键、Hotkey |
| 命令运行的位置 | 文件夹 | Folder | 目录、路径（路径只在显示 ~/Projects/my-app 时出现） |
| 要运行的程序 | 命令 | Command | 脚本、程序 |
| 以前拷贝过、Gocast 记下来的内容 | 剪贴板历史 | Clipboard history | 粘贴板、剪切板、剪贴簿、复制历史 |
| 让一条留在最上面、不过期 | 置顶 | Pin | 钉住、固定、收藏 |
| 放进剪贴板 | 拷贝 | Copy | 复制（Mac 菜单里是「拷贝」） |
| 搜索框里直接算 | 计算器 | Calculator | 计算、换算器 |
| 算出来的那个数 | 答案 | Answer | 结果（结果指搜索结果） |
| 货币之间换算用的比率 | 汇率 | Exchange rate | 牌价、汇价 |

原则：用户说得出口的词。不出现"会话""进程""PTY""Kitty"这类词。

## 1. 菜单栏图标（托盘菜单）

| 位置 | 中文 | English |
|---|---|---|
| 菜单项 1 | 搜索… | Search… |
| 菜单项 2 | 悬浮终端 | Floating Terminal |
| 菜单项 2a | 剪贴板历史 | Clipboard History |
| 分隔线 | | |
| 菜单项 3 | 设置… | Settings… |
| 菜单项 4 | 检查更新… | Check for Updates… |
| 分隔线 | | |
| 菜单项 5 | 退出 Gocast | Quit Gocast |

菜单项右侧显示当前快捷键。图标悬停提示：Gocast。

## 2. 搜索面板

| 位置 | 中文 | English |
|---|---|---|
| 输入框占位 | 搜索应用或终端预设 | Search apps or terminal presets |
| 结果行右侧标记（应用） | 应用 | App |
| 结果行右侧标记（预设） | 预设 | Preset |
| 预设行副标题 | `{文件夹} · {命令}`，如 `~/Projects/my-app · pi`；命令为空写 `{文件夹} · 终端` | `{folder} · {command}`; if empty `{folder} · Terminal` |
| 同名应用的副标题 | `{文件名}`（不带 .app），如 `Siri AI`；文件名和显示名一样的那条不加；文件名也相同时写 `{文件夹}` | `{file name}` without .app; folder only if file names match too |
| 无结果标题 | 没有找到“{输入}” | No results for “{query}” |
| 无结果说明 | 试试应用的另一种叫法，比如英文名或拼音。 | Try another name for the app, like its English name or pinyin. |
| 无结果且没有任何预设时，追加一行 | 想在某个文件夹里直接运行命令？去设置里添加预设。 | Want to run a command in a folder? Add a preset in Settings. |
| 打开失败提示 | 打不开“{名称}”，它可能已被移动或删除。 | Couldn’t open “{name}”. It may have been moved or deleted. |
| 预设的文件夹不存在 | 找不到文件夹 {文件夹}，请在设置里修改这个预设。 | Can’t find the folder {folder}. Edit this preset in Settings. |

说明：中文应用名和拼音（如 wx → 微信）要能搜到，占位文案里不展开，只在"无结果"里提示一次。打开失败提示出现在左下角胶囊里，4 秒后淡出，用词和上面同一条。

### 2.1 操作胶囊（右下角，浮在列表上）

通栏底栏已去掉。右下角只放主操作和「操作」。出错和拷贝成功改在左下角另一枚胶囊，4 秒后淡出。

| 位置 | 中文 | English |
|---|---|---|
| 主操作（选中应用） | 打开 | Open |
| 主操作（选中预设） | 在悬浮终端打开 | Open in Floating Terminal |
| 次入口 | 操作 | Actions |
| 左下角提示：拷贝成功 | 已拷贝路径 | Path copied |
| 左下角提示：打开失败 | 用上面 2 节「打不开…」那条 | same as “Couldn’t open…” |

### 2.2 ⌘K 操作菜单

| 位置 | 中文 | English |
|---|---|---|
| 过滤框占位 | 搜索操作… | Search actions… |
| 过滤无结果 | 没有匹配的操作 | No matching actions |
| 应用 · 1 | 打开 | Open |
| 应用 · 2 | 在访达中显示 | Show in Finder |
| 应用 · 3 | 拷贝路径 | Copy Path |
| 应用 · 4（仅在运行时） | 退出 {应用名} | Quit {app} |
| 预设 · 1 | 在悬浮终端打开 | Open in Floating Terminal |
| 预设 · 2 | 新标签 | New Tab |
| 预设 · 3 | 编辑预设 | Edit Preset |
| 预设 · 4 | 在访达中显示文件夹 | Show Folder in Finder |

用词说明：Mac 上统一说「拷贝」「访达」，跟系统菜单一致；「预设 · 2」用「新标签」，和悬浮终端里的按钮同一个词，不写「新开标签」。退出项带上应用名，避免和「退出 Gocast」混淆。

### 2.2a 打开时的推荐列表（搜索框空着时）

按 ⌘Space 直接展开，分两组。原来空输入 ⌘K 那张「Gocast 操作」列表并进第二组；空输入时也有选中的行了，所以 ⌘K 就是 2.2 那张普通卡片。

| 位置 | 中文 | English |
|---|---|---|
| 第一组小标题 | 推荐 | Suggestions |
| 第二组小标题 | Gocast | Gocast |
| Gocast 组 · 行 | 剪贴板历史 / 打开悬浮终端 / 添加预设 / 设置… / 退出 Gocast | Clipboard History / Open Floating Terminal / Add Preset / Settings… / Quit Gocast |
| Gocast 组行的主操作 | 打开；「退出 Gocast」那行是「退出」 | Open; Quit for Quit Gocast |

用词说明：
- 第二组不叫「命令」。「命令」在 Gocast 里专指预设里运行的那条命令（pi、npm run dev），文件夹 · 命令的副标题天天在眼前，再拿它当组名会混。用「Gocast」，和搜索结果里这些行右侧的标记同一个词。
- 是「添加预设」，不是「新建预设」，跟设置里的按钮同一句。
- 推荐组的行和搜索结果长得一样（应用行、预设行，右侧标记「应用」「预设」）。Gocast 自己的项目进了推荐组时，右侧标记写「Gocast」。没用过时放进去的几个常用项也一样，不加「示例」之类的字。
- Gocast 组的行右侧放键帽：剪贴板历史用当前的剪贴板快捷键，打开悬浮终端用终端快捷键，设置… 用 ⌘,，其余不放。不再写「用预设打开」小标题：预设在推荐组里，也能直接搜。

打字以后照旧是搜索结果，不分这两组。打过字再清空，回到这张推荐列表。

### 2.3 读屏（VoiceOver）

图标下的小圆点只给看得见的人看。读屏用下面这句，不要读「圆点」。

| 位置 | 中文 | English |
|---|---|---|
| 应用行 | {名称}，应用 | {name}, App |
| 应用行（正在运行） | {名称}，应用，正在运行 | {name}, App, Running |
| 同名应用行（有副标题时） | {名称}，{副标题}，应用[，正在运行] | {name}, {subtitle}, App[, Running] |
| 预设行 | {名称}，预设，{文件夹} | {name}, Preset, {folder} |
| 推荐组小标题 | 推荐，标题 | Suggestions, heading |
| Gocast 组小标题 | Gocast，标题 | Gocast, heading |
| Gocast 组的行 | 就用可见的名称，如「剪贴板历史」 | The visible name, e.g. “Clipboard History” |
| 推荐组的行 | 同上面应用行、预设行 | Same as app and preset rows above |

「正在运行」只加在应用上，预设不加（预设开着时是悬浮终端里的标签，不在搜索结果里标运行）。

## 3. 悬浮终端

| 位置 | 中文 | English |
|---|---|---|
| 标签标题 | 预设名称；没有预设时叫“终端” | Preset name; “Terminal” if none |
| 新建标签按钮（提示） | 新标签（⌘T） | New tab (⌘T) |
| 新标签菜单顶部 | 用预设打开 | Open with preset |
| 新标签菜单底部 | 普通终端 | Plain terminal |
| 关闭标签（提示） | 关闭标签（⌘W） | Close tab (⌘W) |
| 收起（提示） | 收起（再按 {快捷键} 回来） | Hide (press {shortcut} to come back) |
| 命令结束提示 | 命令已结束（退出码 {code}） | Command finished (exit code {code}) |
| 命令结束按钮 | 重新运行 | Run Again |
| 命令结束次按钮 | 关闭标签 | Close Tab |
| 命令不存在 | 找不到命令“{命令}”。请检查这个预设里的命令是否写对。 | Can’t find the command “{command}”. Check the command in this preset. |
| 命令不存在时的主按钮 | 编辑预设 | Edit Preset |
| 命令不存在时的次按钮 | 关闭标签 | Close Tab |
| 文件夹不存在 | 找不到文件夹 {文件夹}，已改在你的主文件夹里打开。 | Can’t find the folder {folder}. Opened in your home folder instead. |
| 首次打开、没有预设时（终端上方一行，可关闭） | 这是一个普通终端。想一键在某个文件夹里运行命令，去设置里添加预设。 | This is a plain terminal. To run a command in a folder with one key, add a preset in Settings. |
| 上面那行的按钮 | 添加预设 | Add Preset |
| 提示胶囊右上的 ×（读屏） | 关闭 | Dismiss |

说明：收起不结束命令，所以"收起"的提示里不用"关闭"二字；"关闭标签"才会结束它。提示胶囊上的 × 只关这条提示，读屏用「关闭」，不要写「收起」或「关闭标签」。

## 4. 设置

窗口标题：设置 / Settings。四个分组，顺序如下。侧边栏四项就用下面 4.1–4.4 的小标题：快捷键 / 终端预设 / 通用 / 关于（Shortcuts / Terminal Presets / General / About）。侧边栏写「终端预设」是为了在设置里一眼看出是哪块；其他地方都只说「预设」。

### 4.1 快捷键 / Shortcuts

默认：打开搜索面板 ⌘Space（被聚焦搜索占用时先用 ⌥⇧Space）；显示或收起悬浮终端 ⌃`。第一次打开若 ⌘Space 还在聚焦搜索上，自动打开本页，把提示块放在分组上面。

| 位置 | 中文 | English |
|---|---|---|
| 行标签 1 | 打开搜索面板 | Open search panel |
| 行标签 2 | 显示或收起悬浮终端 | Show or hide floating terminal |
| 行标签 3 | 打开剪贴板历史 | Open clipboard history |
| 录制中 | 请按下新的快捷键 | Press the new shortcut |
| 录制中提示 | 按 esc 取消 | Press esc to cancel |
| 恢复默认 | 恢复默认 | Reset to Default |
| 冲突 | 这个快捷键已被别的应用占用，换一个试试。 | Another app already uses this shortcut. Try a different one. |
| 两个行互相冲突 | 这个快捷键已用于“{另一项}”。 | This shortcut is already used for “{other}”. |
| 占用提示标题 | ⌘Space 已被聚焦搜索占用 | Spotlight is using ⌘Space |
| 占用提示说明 | 两个会一起弹出来。去系统设置关掉“显示聚焦搜索”。现在先用 ⌥⇧Space。 | Both would open at once. Turn off “Show Spotlight search” in System Settings. For now, using ⌥⇧Space. |
| 占用提示主按钮 | 打开系统设置 | Open System Settings |
| 占用提示次按钮 | 就用 ⌥⇧Space | Use ⌥⇧Space |
| 占用已解除 | 已改用 ⌘Space | Now using ⌘Space |
| 录制框说明（被聚焦搜索占用） | ⌘Space 被聚焦搜索占用 | Spotlight is using ⌘Space |

### 4.2 终端预设 / Terminal Presets

| 位置 | 中文 | English |
|---|---|---|
| 小标题说明 | 预设会记住在哪个文件夹运行什么命令。在搜索面板里输入预设名就能打开。 | A preset remembers which command to run in which folder. Type its name in the search panel to open it. |
| 空状态标题 | 还没有预设 | No presets yet |
| 空状态说明 | 比如：在 my-app 文件夹里直接启动 pi。 | For example: start pi right inside your my-app folder. |
| 添加按钮 | 添加预设 | Add Preset |
| 编辑页字段：名称 | 名称 | Name |
| 名称占位 | 如：my-app · pi | e.g. my-app · pi |
| 字段：文件夹 | 文件夹 | Folder |
| 文件夹按钮 | 选择… | Choose… |
| 字段：命令 | 命令 | Command |
| 命令占位 | 留空则打开普通终端 | Leave empty for a plain terminal |
| 字段：环境变量（折叠，默认收起） | 环境变量（可选） | Environment variables (optional) |
| 环境变量格式提示 | 每行一个，如 NAME=value | One per line, like NAME=value |
| 设为默认 | 设为默认预设 | Make Default |
| 默认标记 | 默认 | Default |
| 默认预设说明 | 按悬浮终端快捷键时，第一次会用它打开。 | The first time you press the terminal shortcut, this preset opens. |
| 保存 / 取消 | 保存 / 取消 | Save / Cancel |
| 删除 | 删除预设 | Delete Preset |
| 删除确认标题 | 删除“{名称}”？ | Delete “{name}”? |
| 删除确认说明 | 正在运行的标签不会受影响，但之后搜不到这个预设了。 | Tabs already running stay open, but you won’t find this preset in search anymore. |
| 名称为空 | 给预设起个名字。 | Give this preset a name. |
| 名称重复 | 已经有叫“{名称}”的预设了。 | A preset named “{name}” already exists. |
| 文件夹为空 | 请选择一个文件夹。 | Choose a folder. |

### 4.3 通用 / General

| 位置 | 中文 | English |
|---|---|---|
| 开关 | 登录时自动启动 | Open Gocast at login |
| 开关说明 | 打开后，开机就能用快捷键，不用先手动打开。 | Then your shortcuts work right after you log in. |
| 开关 | 打开时只显示搜索框 | Show only the search field |
| 开关说明 | 打开后，按 {搜索快捷键} 只出现搜索框，不列推荐，打字才出结果。 | When on, {shortcut} shows just the search field. Results appear as you type. |
| 分组标题 | 悬浮终端 | Floating Terminal |
| 分组说明 | 也可以直接拖终端边缘来改大小，这里会跟着变。 | You can also drag the terminal's edges. These values update as you do. |
| 终端字号 | 字号 | Font size |
| 宽度 | 宽度 | Width |
| 宽度说明 | 占当前屏幕的百分之几。 | Percent of the current screen. |
| 宽度值 | {n}% | {n}% |
| 高度 | 高度 | Height |
| 高度说明 | 占当前屏幕的百分之几。 | Percent of the current screen. |
| 高度值 | {n}% | {n}% |
| 开关 | 失焦时收起悬浮终端 | Hide floating terminal when you click away |
| 开关说明 | 关掉后，切去别的窗口时终端还留在屏幕上。 | When off, the terminal stays on screen while you work in other windows. |
| 外观 | 外观 | Appearance |
| 外观选项 | 跟随系统 | Match system |
| 外观说明 | 深色或浅色跟着系统走。 | Light or dark follows your Mac. |
| 开关 | 自动检查更新 | Check for updates automatically |
| 开关说明 | 打开后，有新版本会自己来问你。关掉就只在点“检查更新”时查。 | When on, Gocast asks you when a new version is ready. When off, it only checks if you ask. |

第一版外观只有“跟随系统”一项，需要时再加选项；不要显示一个只有一项的下拉框，用一行说明文字即可。
「打开时只显示搜索框」默认关。不叫「紧凑模式」：用户看不出「紧凑」紧的是什么，开关名直接说看到的样子。放在通用里哪一行以 design.md 为准。
「悬浮终端」的分组说明放在标题下面、分组上面。宽度和高度两行共用同一句说明；拖边缘改大小写在分组说明里，不要分别写在宽度或高度下面。

### 4.4 关于 / About

| 位置 | 中文 | English |
|---|---|---|
| 应用名 + 一句话 | Gocast　按一下，应用和终端都到。 | Gocast　One key. Apps and terminal, right there. |
| 版本 | 版本 {version} | Version {version} |
| 按钮 | 检查更新 | Check for Updates |
| 页脚 | 不放。 | None. |

点「检查更新」打开更新窗口（和菜单栏「检查更新…」同一个）。检查中、已最新、有新版本、失败都由窗口来说，关于页底下不再另写一行。

### 4.5 更新窗口（MyGo 更新器，用 Options.Strings 覆盖，不要用插件自带那套带感叹号的）

| 位置 | 中文 | English |
|---|---|---|
| 窗口标题 | 软件更新 | Software Update |
| 检查中 | 正在检查… | Checking… |
| 取消 / 好 | 取消 / 好 | Cancel / OK |
| 已最新标题 | 已是最新版本 | You’re Up to Date |
| 已最新说明 | %[1]s %[2]s 已经是最新版本。 | %[1]s %[2]s is the latest version. |
| 开发版 | 开发版本不会自己更新。 | Development builds don’t update themselves. |
| 没法更新标题 | 没法在这里更新 | Updates Unavailable |
| 没法更新说明 | %[1]s 没法在当前安装位置自己更新。如果是用其他方式装的，请在那里更新。 | %[1]s can’t update itself here. If you installed it another way, update it there. |
| 出错标题 | 更新出错 | Update Error |
| 检查失败 | 现在检查不了更新，请稍后再试。 | Couldn’t check for updates. Try again later. |
| 安装失败 | 安装更新时出错，请稍后再试。 | Couldn’t install the update. Try again later. |
| 有新版本标题 | %[1]s 可以更新了 | %[1]s is ready to update |
| 有新版本说明 | %[1]s %[2]s 可以安装了，你现在用的是 %[3]s。 | %[1]s %[2]s is ready. You’re on %[3]s. |
| 更新说明小标题 | 更新说明 | Release Notes |
| 自动下载勾选 | 以后自动下载并安装更新 | Automatically download and install updates |
| 跳过 | 跳过这个版本 | Skip This Version |
| 稍后 | 稍后提醒 | Remind Later |
| 安装 | 安装更新 | Install Update |
| 下载中 | 正在下载… | Downloading… |
| 进度 | %[1]s / %[2]s | %[1]s / %[2]s |
| 大小单位 | %s MB | %s MB |
| 安装中 | 正在安装… | Installing… |
| 装好标题 | 可以重新启动了 | Ready to Relaunch |
| 装好说明 | %[1]s %[2]s 已经装好，下次打开就会用新版本。现在重新启动也可以。 | %[1]s %[2]s is installed and will open next time. You can relaunch now. |
| 装好后稍后 | 稍后 | Later |
| 立即重启 | 重新启动 | Relaunch |

`%[1]s` 是应用名 Gocast，`%[2]s` 是新版本号，`%[3]s` 是当前版本号。菜单栏「检查更新…」跟插件的 MenuItem 用同一句，不要另写。

## 6. 剪贴板历史

这一节的用词先看最上面「统一用词」里新加的三行：剪贴板历史、置顶、拷贝。范围以需求为准，布局、尺寸和位置以 design.md 为准；文案和设计冲突时，以 design.md 为准，我再跟着改。

### 6.1 入口

| 位置 | 中文 | English |
|---|---|---|
| 搜索面板结果行 | 剪贴板历史 | Clipboard History |
| 上面那行的右侧标记 | Gocast | Gocast |
| 上面那行也能搜到的词（不显示） | 剪贴板、粘贴板、剪切板、历史、clipboard、paste、history | same |
| 打开时的 Gocast 组（2.2a，放在第一行） | 剪贴板历史 | Clipboard History |
| 菜单栏菜单（放在「悬浮终端」下面） | 剪贴板历史 | Clipboard History |

主操作：打开（↵）。右侧标记不写「命令」：「命令」在 Gocast 里专指预设里运行的那条命令。键帽：Gocast 组和菜单栏这一项右侧显示当前的剪贴板快捷键（默认 ⌃⌘V）。
用户自己会说「粘贴板」「剪切板」，所以这些词都要能搜到，但界面上只写「剪贴板」。

### 6.2 剪贴板视图

| 位置 | 中文 | English |
|---|---|---|
| 搜索框占位 | 搜索剪贴板历史 | Search clipboard history |
| 类型筛选（⌘P）· 默认 | 全部类型 | All Types |
| 类型筛选 · 其余 | 文字 / 链接 / 图片 / 文件 | Text / Links / Images / Files |
| 分组标题 | 已置顶 / 今天 / 昨天 / 更早 | Pinned / Today / Yesterday / Earlier |
| 图片行标题 | 图片 {宽}×{高} | Image {w}×{h} |
| 多个文件的行标题 | {第一个文件名} 等 {n} 个文件 | {first file} and {n−1} more |
| 来源应用未知 | 未知应用 | Unknown app |

搜索框占位不带「…」，和主搜索面板「搜索应用或终端预设」一致。文字行显示内容的第一行，空白行跳过，过长从末尾截断。链接行显示完整地址，过长从中间省略。

### 6.3 预览下面的信息

| 位置 | 中文 | English |
|---|---|---|
| 来源 | 来源 | Source |
| 时间 | 拷贝时间 | Copied |
| 文字 | 字数 | Characters |
| 图片 | 尺寸 / 大小 | Dimensions / Size |
| 文件 | 位置 / 大小 | Location / Size |
| 链接 | 链接 | Link |
| 文件已不在 | 这个文件已被移动或删除。 | This file has been moved or deleted. |

时间用系统的相对写法：「今天 14:05」「昨天 09:12」「11月10日 18:30」，跟系统语言和 12/24 小时制走，不要自己拼。字数按字符算，中文一个字算一个。

### 6.4 空状态

| 场景 | 标题 | 说明 | 按钮 |
|---|---|---|---|
| 一条都没有 | 还没有剪贴板历史 | 拷贝点什么，它就会出现在这里。 | 无 |
| 记录已关闭 | 剪贴板历史已关闭 | 打开后，你拷贝的内容会记在这里。 | 打开 |
| 搜不到 | 没有找到“{输入}” | 试试内容里的原话。这里不按拼音搜。 | 无 |
| 筛选后没有 | 没有{类型} | 无 | 无 |
| 读不出历史（文件坏了） | 读不出剪贴板历史 | 保存的历史打不开了。去设置 › 剪贴板点“清空历史…”，从头开始记录。 | 无 |

English: No clipboard history yet / Copy something and it shows up here. · Clipboard history is off / Turn it on and what you copy is kept here. / Turn On · No results for “{query}” / Try words from the item itself. Pinyin isn’t matched here. · No {type}（No text / No links / No images / No files）· Can’t read clipboard history / Your saved history can’t be opened. Go to Settings › Clipboard and click Clear History… to start fresh.

「打开」直接打开记录开关，不跳到设置。「读不出」这一条不放按钮，只说去哪里清空；设置里的「清空历史…」这时不能灰掉。主搜索面板能用拼音搜，这里不能，所以在「搜不到」里说清楚。

### 6.5 操作胶囊和 ⌘K 菜单

| 位置 | 中文 | English | 键 |
|---|---|---|---|
| 主操作（可以直接粘贴时） | 粘贴到“{应用名}” | Paste to {app} | ↵ |
| 主操作（去向是悬浮终端时） | 粘贴到悬浮终端 | Paste to Floating Terminal | ↵ |
| 主操作（不知道刚才是哪个应用时） | 粘贴 | Paste | ↵ |
| 主操作（没有辅助功能权限时） | 拷贝到剪贴板 | Copy to Clipboard | ↵ |
| 次入口 | 操作 | Actions | ⌘K |
| ⌘K · 1 | 粘贴到“{应用名}” | Paste to {app} | ↵ |
| ⌘K · 2（只对文字） | 粘贴为纯文本 | Paste as Plain Text | |
| ⌘K · 3 | 拷贝到剪贴板 | Copy to Clipboard | ⌘↵ |
| ⌘K · 4（只对链接） | 打开链接 | Open Link | ⌘O |
| ⌘K · 5（只对文件） | 在访达中显示 | Show in Finder | |
| ⌘K · 6 | 置顶 / 取消置顶 | Pin / Unpin | ⌘. |
| ⌘K · 7 | 删除 | Delete | ⌃X |
| ⌘K · 8 | 清空历史… | Clear History… | ⌃⇧X |

主操作写的就是按回车会发生的事：没有权限时回车只会拷贝，就别写「粘贴」。不写「复制」，Mac 菜单里是「拷贝」。「清空历史…」和设置页按钮是同一个动作，用同一句、同一个确认框。

粘贴去向（参考 Tinycast）就是上面「粘贴到“{应用名}”」这一句，写在右下操作胶囊里（design.md），不另写一句。应用名用系统显示的名字（备忘录、终端、iTerm）；去向是 Gocast 自己的悬浮终端时写「粘贴到悬浮终端」（Paste to Floating Terminal），不写“Gocast”。

往终端里粘贴图片：贴进去的是这张图在本机的文件路径。主操作还是「粘贴到“{应用名}”」，不改成「粘贴路径」：要的就是把这张图交给终端里的程序。界面里不加提示，只在 README 里说一句（见 6.11）。

取消置顶的那条变成最新一条：提示还是「已取消置顶」，不另写。

### 6.6 提示（左下角胶囊，4 秒后淡出）

拷贝或粘贴成功后面板直接收起，不出提示。

| 场景 | 中文 | English |
|---|---|---|
| 没有权限时回车 | 已拷贝，按 ⌘V 粘贴。 | Copied. Press ⌘V to paste. |
| 删除一条 | 已删除 | Deleted |
| 置顶 / 取消置顶 | 已置顶 / 已取消置顶 | Pinned / Unpinned |

### 6.7 辅助功能权限（第一次按回车想直接粘贴时，在剪贴板视图顶上出现，和 ⌘Space 那块提示同一种样式）

| 位置 | 中文 | English |
|---|---|---|
| 标题 | 直接粘贴需要你允许一次 | Allow Gocast to paste for you |
| 说明 | Gocast 会替你按一下 ⌘V，系统要求先在“辅助功能”里打开 Gocast。只用来粘贴。 | Gocast presses ⌘V for you. macOS asks you to turn on Gocast in Accessibility first. It’s only used to paste. |
| 主按钮 | 打开系统设置 | Open System Settings |
| 次按钮 | 先只拷贝 | Just Copy |
| 允许后 | 现在可以直接粘贴了 | Ready to paste |

这次回车已经把内容拷贝到剪贴板了，提示出现的同时左下角给「已拷贝，按 ⌘V 粘贴。」。点「先只拷贝」后这块提示不再出现，回车一直是拷贝；想改回来去设置 › 剪贴板。从系统设置切回来时再查一次，有权限了就换成「现在可以直接粘贴了」，3 秒后收起。

### 6.8 设置 › 剪贴板 / Clipboard

侧边栏加一项「剪贴板」，建议顺序：快捷键 / 终端预设 / 剪贴板 / 通用 / 关于。位置以 design.md 为准。

| 位置 | 中文 | English |
|---|---|---|
| 开关 | 记录剪贴板历史 | Keep clipboard history |
| 开关说明 | 只存在这台 Mac 上，不会上传。 | Stored only on this Mac, never uploaded. |
| 保留时长 | 保留 | Keep for |
| 保留选项 | 1 天 / 1 周 / 1 个月 / 3 个月 / 一直保留 | 1 Day / 1 Week / 1 Month / 3 Months / Forever |
| 保留说明 | 置顶的不会过期。 | Pinned items never expire. |
| 直接粘贴开关 | 回车直接粘贴 | Press Return to paste |
| 直接粘贴说明 | 需要在系统设置的“辅助功能”里打开 Gocast。关掉后，回车只拷贝。 | Needs Gocast turned on in Accessibility. When off, Return only copies. |
| 没有权限时的说明 | 还没在“辅助功能”里打开 Gocast。 | Gocast isn’t turned on in Accessibility yet. |
| 上面那行的按钮 | 打开系统设置 | Open System Settings |
| 分组标题 | 不记录的应用 | Don’t keep from these apps |
| 分组说明 | 从这些应用拷贝的内容不会记下来。密码管理器拷贝的密码本来就不记。 | Nothing you copy in these apps is kept. Passwords from password managers are never kept anyway. |
| 加号 | 添加应用… | Add App… |
| 减号（读屏） | 移除 | Remove |
| 选应用的对话框按钮 | 添加 | Add |
| 按钮 | 清空历史… | Clear History… |

「回车直接粘贴」默认开。点了「先只拷贝」就把它关掉，以后不再问；想改回来就在这里打开。
剪贴板快捷键不放在这一页，只在设置 › 快捷键的第三行「打开剪贴板历史」，冲突提示沿用 4.1。

### 6.9 清空确认

| 位置 | 中文 | English |
|---|---|---|
| 标题 | 清空剪贴板历史？ | Clear clipboard history? |
| 说明 | 置顶的会留下，其他都会删掉，不能恢复。 | Pinned items stay. Everything else is deleted for good. |
| 说明（读不出历史时） | 保存的历史会全部删掉，包括置顶的，不能恢复。 | All saved history is deleted, including pinned items. This can’t be undone. |
| 按钮 | 清空 / 取消 | Clear / Cancel |

置顶的要删，就在那条上单独删。读不出历史时置顶的也留不住，换用第二句说明。

### 6.10 读屏（VoiceOver）

| 位置 | 中文 | English |
|---|---|---|
| 整张列表 | 剪贴板历史 | Clipboard History |
| 文字行 | {第一行}，文字，{时间}[，已置顶] | {first line}, Text, {time}[, Pinned] |
| 链接行 | {地址}，链接，{时间}[，已置顶] | {address}, Link, {time}[, Pinned] |
| 图片行 | 图片，{宽}乘{高}，{时间}[，已置顶] | Image, {w} by {h}, {time}[, Pinned] |
| 文件行 | {文件名}，文件，{时间}[，已置顶] | {file name}, File, {time}[, Pinned] |
| 多个文件行 | {第一个文件名}等 {n} 个文件，{时间}[，已置顶] | {first file} and {n−1} more, Files, {time}[, Pinned] |
| 分组标题 | 今天，标题 | Today, heading |
| 预览区 | 预览 | Preview |
| 类型筛选 | 类型，{当前类型} | Type, {current} |

读「乘」不读「×」。时间读系统的相对写法（今天 14:05）。

### 6.11 README（已写入 README.md / README.zh-Hans.md）

能做什么：

中文：按 ⌃⌘V，找回之前拷贝过的文字、链接、图片和文件，按 ↵ 粘贴到刚才的应用。常用的可以置顶。往终端里粘贴图片时，贴进去的是图片在本机的文件路径。

English: Press ⌃⌘V to find the text, links, images and files you copied before, and press ↵ to paste into the app you were in. Pin the ones you use often. Paste an image into a terminal and you get the image's file path on this Mac.

快捷键表加一行：⌃⌘V · 任何地方 · 打开剪贴板历史 / anywhere / Open clipboard history。三个全局快捷键都可以在设置 › 快捷键里改。

你的数据：

中文：剪贴板历史也只存在这台 Mac 上（同一个文件夹），不会上传。密码管理器拷贝的密码不会记。可以在设置 › 剪贴板里关掉、清空，或者选哪些应用不记。

English: Clipboard history stays on this Mac too, in the same folder, and is never uploaded. Passwords from password managers are never kept. Turn it off, clear it, or exclude apps in Settings › Clipboard.

卸载说明改成：预设、设置和剪贴板历史会留着。 / Your presets, settings and clipboard history stay.

## 7. 计算器

范围以计算器需求 v1 为准，卡片的样子和位置以 design.md 为准。用词：算出来的叫「答案」，不叫「结果」：「结果」在 Gocast 里指搜索结果，卡片下面就是一列搜索结果，两个词放一起会混。搜索框占位「搜索应用或终端预设」不改，会算这件事靠 README 和用的人自己发现，跟 Raycast 一样。

### 7.1 卡片

| 位置 | 中文 | English |
|---|---|---|
| 左边 | 算式，照用户打的显示，全角符号换成半角 | The expression as typed, full-width symbols normalized |
| 右边 | 答案。单位用符号：`3.048 m`、`86 °F`、`50 kg`、`32 px`；货币用代码：`15,432 JPY` | Answer with unit symbols or currency codes |
| 时间答案 | {h} 小时 {m} 分（如「2 小时 25 分」）；不满 1 小时写「{m} 分」，有天写「{d} 天 {h} 小时」 | {h} h {m} min; {d} d {h} h |
| 左边小字（有单位或货币时） | 单位或币种的中文全名，如「美元」「英尺」「人民币」 | Full name, e.g. “US dollars”, “feet”, “Chinese yuan” |
| 右边小字（换算） | 目标单位全名，如「米」「日元」 | Target full name, e.g. “meters”, “Japanese yen” |
| 右边小字（汇率） | {币种全名} · 汇率更新于 {日期} | {currency} · Rates from {date} |
| 没有汇率又没联网（右边） | 需要联网获取一次汇率 | Go online once to get exchange rates |
| 第一次正在取汇率（右边） | 正在获取汇率… | Getting exchange rates… |
| 算不出（右边，如除以 0、单位换不了） | 算不出来 | Can’t calculate this |

卡片上不放「计算器」标记（design.md：箭头和左右两栏已经说明它是什么），「计算器」只在读屏里出现。
「算不出来」跟全应用「打不开…」「读不出…」「找不到…」同一种口气，不写「无法计算」。打到一半（结尾是运算符）不算算不出，留着上一次的答案变灰，见 design.md。
纯四则、百分比没有小字。小字写全名是为了让人看清 Gocast 是怎么理解的：打 `¥5000` 时，左边小字写「人民币」或「日元」，看一眼就知道对不对；不对就改写成 `5000 jpy`。不另加提示。
日期跟剪贴板 6.3 一样用系统的相对写法：今天 / 昨天 / 11月10日。断网时继续用旧汇率，日期本身就说明了新旧，不另写「离线」。

### 7.2 操作胶囊和 ⌘K

| 位置 | 中文 | English | 键 |
|---|---|---|---|
| 主操作 | 拷贝答案 | Copy Answer | ↵ |
| ⌘K · 1 | 拷贝答案 | Copy Answer | ↵ |
| ⌘K · 2 | 粘贴到“{应用名}” | Paste to {app} | ⌘↵ |
| ⌘K · 2（不知道刚才是哪个应用时） | 粘贴 | Paste | ⌘↵ |
| ⌘K · 3 | 拷贝算式和答案 | Copy Expression and Answer | |
| ⌘K · 4 | 拷贝纯数字 | Copy Number Only | |

「粘贴到“{应用名}”」和剪贴板 6.5 同一句，去向是悬浮终端时写「粘贴到悬浮终端」。没有辅助功能权限时 ⌘↵ 只拷贝，提示用剪贴板 6.6 那句「已拷贝，按 ⌘V 粘贴。」。拷贝成功面板直接收起，不出提示。
「拷贝算式和答案」拷出来是 `100 USD = 15,432 JPY`；「拷贝纯数字」拷出来是 `15432.12`，没有分组、没有单位。

### 7.3 设置 › 通用

| 位置 | 中文 | English |
|---|---|---|
| 开关 | 在搜索中计算 | Calculate in search |
| 开关说明 | 在搜索框里打算式、单位或货币，最上面直接出答案。汇率每天联网取一次。 | Type math, units or currencies in the search panel to get the answer on top. Exchange rates are fetched once a day. |

放在通用哪一行以 design.md 为准。

### 7.4 读屏（VoiceOver）

| 位置 | 中文 | English |
|---|---|---|
| 卡片 | {算式} 等于 {答案}，计算器 | {expression} equals {answer}, Calculator |
| 带单位或货币时 | 用全名读：「100 美元 等于 15,432 日元，计算器」 | Read full names: “100 US dollars equals 15,432 Japanese yen, Calculator” |
| 汇率日期 | 接在后面：「，汇率更新于 11月13日」 | Append: “, rates from November 13” |
| 没有汇率 | {算式}，需要联网获取一次汇率，计算器 | {expression}, go online once to get exchange rates, Calculator |
| 正在取汇率 | {算式}，正在获取汇率，计算器 | {expression}, getting exchange rates, Calculator |
| 算不出 | {算式}，算不出来，计算器 | {expression}, can’t calculate this, Calculator |
| 打到一半（旧答案灰掉，不能拷走） | {算式}，还没打完，上一个答案是 {答案}，计算器 | {expression}, not finished, previous answer {answer}, Calculator |

符号读成字：× 读「乘」，÷ 读「除以」，^ 读「的…次方」，% 读「百分之」。

### 7.6 单位和币种的全名（小字和读屏用）

原则：用大家嘴上的叫法，不用教科书叫法。英文用 CLDR 的复数全名，不改。

| 单位 / 币种 | 中文 | 不用 |
|---|---|---|
| km | 公里 | 千米 |
| kg | 公斤 | 千克 |
| km/h | 公里每小时 | 千米每小时 |
| km² | 平方公里 | 平方千米 |
| KB / MB / GB / TB / PB | KB（1000 字节）/ MB（1000 KB）/ GB（1000 MB）/ TB（1000 GB）/ PB（1000 TB） | 千字节、兆字节、吉字节… |
| KiB / MiB / GiB / TiB / PiB | KiB（1024 字节）/ MiB（1024 KiB）/ GiB（1024 MiB）/ TiB（1024 GiB）/ PiB（1024 TiB） | 二进制千字节… |
| HKD | 港币 | 港元 |
| AUD | 澳元 | 澳大利亚元 |
| CAD | 加元 | 加拿大元 |

数据单位的小字要让人看出按 1000 还是 1024 算，这是大家真会搞错的地方；「吉字节」「二进制千字节」没人这么说。其余单位和币种用现在 locales 里的写法。时长答案里用「天 / 小时 / 分 / 秒」（「2 小时 25 分」），单位全名里写「分钟」，两处不冲突。

### 7.5 README（已写入 README.md / README.zh-Hans.md）

能做什么，接在「预设」后面：

中文：**计算器。** 在搜索框里直接打算式、单位或汇率，比如 `(12+8)*3`、`10 ft in m`、`100 美元换日元`，最上面就是答案，按 ↵ 拷走。

English: **Calculator.** Type math, units or currencies right in the search panel, like `(12+8)*3`, `10 ft in m` or `100 usd in jpy`. The answer shows up on top; press ↵ to copy it.

你的数据，第一段改成：

中文：Gocast 没有账号，不会把你的任何信息发到别处。它只在检查更新和每天取一次汇率时联网，取汇率时不带你输入的任何内容。两样都能在设置 › 通用里关掉（“自动检查更新”“在搜索中计算”）。

English: Gocast has no account and sends nothing about you anywhere. It only goes online to check for updates and to fetch exchange rates once a day, and that request carries nothing you typed. Turn either off in Settings › General (Check for updates automatically, Calculate in search).

致谢加一句：

中文：汇率来自 [Frankfurter](https://frankfurter.dev)，用的是各国央行公布的参考汇率。

English: Exchange rates come from [Frankfurter](https://frankfurter.dev), which publishes central bank reference rates.

更新日志（CHANGELOG.md）也是给人看的字，而且现在网站的 /changelog/ 两页直接从它生成，改一句网页就跟着变。0.1.0 那条已补：计算器一行、搜索面板打开时先列推荐、设置里「打开时只显示搜索框」。设置分页名照 locales 写：快捷键、终端预设、剪贴板、通用、关于。

## 5. 系统弹窗里会出现的文字（系统要求写明用途）

| 场景 | 中文 | English |
|---|---|---|
| 终端访问桌面/文稿/下载等文件夹时系统弹授权 | 悬浮终端里运行的命令需要访问这个文件夹。 | Commands you run in the floating terminal need access to this folder. |
| 登录项首次被系统提示 | Gocast 想在你登录时自动启动，好让快捷键随时可用。 | Gocast wants to open at login so your shortcuts are always ready. |

## 给设计
- 搜索面板里一行最多放：图标、名称、右侧标记；预设行和同名应用行多一段副标题，副标题过长从中间省略（保留文件夹最后一段和命令开头）。
- “退出码”只在非 0 时显示；退出码 0 时只写“命令已结束”。
- 全部文案要同时能放进英文，英文通常更长 20% 左右，别把按钮宽度卡死。

## 给开发
- 文案放进一个语言表（zh-Hans / en），跟系统语言走，不要写死在界面代码里。
- “已改在你的主文件夹里打开”这条说明做兜底：预设文件夹丢了不要让终端打不开。
- ⌘Space 被聚焦搜索占用的提示只在第一次打开时出现；从系统设置切回来再查一次，空出来就换成 ⌘Space 并显示「已改用 ⌘Space」。点「就用 ⌥⇧Space」后默认改成它，不再提示。别的应用占着检测不到，不要用这组文案。

## 8. 签名和公证之后的安装说明（已验过，已写入新版 README）

装好的版本双击直接能开以后，README 两份照下面改；网站支持页由网站那边对齐同样的意思。

磁盘映像一段，删掉「第一次打开时……仍要打开」，改成：

中文：**磁盘映像。** 到 [Releases](…) 下载最新的 `.dmg`，打开后把 Gocast 拖进“应用程序”文件夹，双击就能打开。安装包经过 Apple 公证。

English: **Disk image.** Download the latest `.dmg` from [Releases](…), open it, and drag Gocast to Applications. Double-click to open it; the app is notarized by Apple.

终端一段，删掉最后的「这样装的不用点「仍要打开」。」/ "macOS will not ask you to click Open Anyway."，其余不变。

不写「Developer ID」「签名」这些词：读的人只关心能不能直接打开。「经过 Apple 公证」留着，是给会去查的人一个可以核对的说法。

## 9. README 改版（2026-11-13，参考 Tinycast 的结构）

README 只留门面、安装和链接，细节交给网站：居中图标 + 标语、一排链接（网站 / 安装 / 更新日志 / 支持）、一排徽章、一张主图（docs/images/search-panel-en.png 和 search-panel-zh-Hans.png，深色，空白示例数据）、两三句介绍 + 「看看它能做的全部 →」、安装（已公证，双击就能打开；终端一行；卸载）、你的数据（一小段 + 隐私政策链接）、参与、致谢、许可证。功能长段落、快捷键表、第一次打开、从源码构建都已移走（网站和 CONTRIBUTING.md 里有）。上面 6.11、7.5 等节里写给 README 的段落从此只作网站文案的参考。

## 10. 「你的数据」现在的说法（2026-11-13，隐私检查后）

README 里那一段（网站隐私页同样意思）：

中文：Gocast 只在检查更新和每天取一次汇率时联网，两样都能关。预设、设置、剪贴板历史，还有你从搜索里打开过什么、用哪个词打开的（用来把常用的排前面，搜索词只留 17 天），都在 ~/Library/Application Support/com.hugo.gocast/，不会上传。

English: Gocast only goes online to check for updates and to fetch exchange rates once a day, and both can be turned off. Your presets, settings, clipboard history, and a record of what you open from search and the words you typed to find it (so your usual picks come first; the words are kept for 17 days) all stay in ~/Library/Application Support/com.hugo.gocast/ and are never uploaded.

另外记一句：签名里带公司名和团队编号、Bundle ID 里有 hugo，这些是公开信息，公开后谁都能看到，不用在文案里解释。

## 11. 给应用和预设设快捷键（v1.19，需求见 app-hotkeys-brief）

用词：还是「快捷键」，不叫「热键」「Hotkey」（Raycast 这么叫，我们统一用词表里一直是快捷键）。按下后应用到前面叫「打开」，已在最前面再按叫「隐藏」（跟 ⌘H 和系统菜单里的「隐藏」一致）；预设那边是悬浮终端，用「收起」，跟 ⌃` 一样。

### 11.1 ⌘K 菜单（应用和预设都有，放在原有几项后面，位置以 design.md 为准）

| 情况 | 中文 | English |
|---|---|---|
| 还没设 | 设置快捷键… | Set Shortcut… |
| 已经有了 | 更改快捷键… | Change Shortcut… |
| 已经有了 | 移除快捷键 | Remove Shortcut |

「设置」后面带省略号，因为点了还要再按键；「移除」点了就生效，不带。

### 11.2 面板里录制

| 位置 | 中文 | English |
|---|---|---|
| 标题 | 给“{名称}”设快捷键 | Shortcut for “{name}” |
| 录制中 | 请按下新的快捷键 | Press the new shortcut |
| 录制中提示 | 按 esc 取消 | Press esc to cancel |
| 只按了 ⇧ 或普通键 | 要带上 ⌘、⌥ 或 ⌃，或者单独用 F1–F19。 | Include ⌘, ⌥ or ⌃, or use F1–F19 on its own. |
| 跟 Gocast 自己的或别的应用、预设撞了 | 这个快捷键已用于“{另一项}”。 | This shortcut is already used for “{other}”. |
| 被别的应用占着（系统拒绝注册） | 这个快捷键已被别的应用占用。如果是在别的启动器里设过，先在那边删掉。 | Another app already uses this shortcut. If you set it in another launcher, remove it there first. |
| 存好后（左下角提示胶囊） | 已设为 {快捷键} | Shortcut set to {shortcut} |
| 移除后（左下角提示胶囊） | 已移除快捷键 | Shortcut removed |

{另一项} 用看得见的名字：Gocast 自己的三个写行标签（打开搜索面板、显示或收起悬浮终端、打开剪贴板历史），应用写应用名，预设写预设名。
「别的启动器」那半句就是给从 Raycast 搬过来的人看的，只在这条报错里出现一次，界面里不点 Raycast 的名字。

### 11.3 设置 › 快捷键（v1.20 改成一整张能搜的列表，照 Raycast）

不用先添加：Gocast 自己的三项在最上面，下面是所有应用和所有终端预设，每行右边直接点着设。原来的「添加应用…」「选择预设…」、空状态那句、选应用的系统窗口都不要了。

| 位置 | 中文 | English |
|---|---|---|
| 页面说明（列表上面一行） | 给应用或预设设一个快捷键，按一下打开，已在最前面时再按一下隐藏。 | Give an app or preset a shortcut. Press it to open, and again while it’s in front to hide it. |
| 搜索框占位 | 搜索应用或预设 | Search apps and presets |
| 筛选 · 1 | 全部 | All |
| 筛选 · 2 | 已设快捷键 | With Shortcut |
| 筛选 · 3 | 应用 | Apps |
| 筛选 · 4 | 预设 | Presets |
| 行右边的类型小字 | 应用 / 预设 / Gocast | App / Preset / Gocast |
| 没设时录制框里的灰字 | 设置快捷键 | Set Shortcut |
| 录制中 / 录制中提示 | 请按下新的快捷键 / 按 esc 取消 | Press the new shortcut / Press esc to cancel |
| 行末 ×（读屏） | 移除“{名称}”的快捷键 | Remove shortcut for “{name}” |
| 搜不到 | 没有匹配的应用或预设 | No matching apps or presets |
| 「已设快捷键」下没有应用或预设（Gocast 三项照常列在上面） | 还没有给应用或预设设快捷键。在「全部」里点任意一行右边就能设。 | No app or preset shortcuts yet. In All, click the right side of any row to set one. |
| 找不到的应用（行变灰，名字下面一行） | 找不到这个应用 | Can’t find this app |
| 找不到的应用 · 鼠标停留说明 | 应用装回来后，快捷键会自动恢复。 | The shortcut works again once the app is back. |

找不到的应用那一行，名字用设快捷键时记下的应用名（如「微信」），不用包名；只有旧数据里没记名字时才退回包名。

冲突（录制框变红，下面一行）：

| 情况 | 中文 | English |
|---|---|---|
| 跟别的应用或预设撞了 | 这个快捷键已用于“{另一项}”。 | This shortcut is already used for “{other}”. |
| 同一行的按钮 | 改用在这里 | Use Here |
| 跟 Gocast 自己的三项撞了（没有按钮） | 这个快捷键已用于“{另一项}”，换一个吧。 | This shortcut is already used for “{other}”. Try a different one. |
| 被别的应用占着 | 同 11.2 | Same as 11.2 |

「改用在这里」点了就生效：这一行拿到快捷键，“{另一项}”那行变回「设置快捷键」，不再弹确认，也不出提示，两行的变化就在眼前。不叫「覆盖」「替换」：用户想的是“把这个键挪过来”，这四个字正好是这个意思。
筛选用「已设快捷键」不用「已设置」：单看筛选那一排，「已设置」不知道设了什么。
类型小字跟搜索面板里结果行的标记同一套（应用、预设），Gocast 自己的三项写「Gocast」，跟搜索面板里那一组的组名一样。
11.2 面板里录制的「被别的应用占用」那句（带“别的启动器”半句）设置页也用同一句。

### 11.4 读屏（VoiceOver）

结果行有快捷键时，在原来那句最后加「快捷键 {按键}」，按键读成英文键名，跟系统读屏一致：

| 位置 | 中文 | English |
|---|---|---|
| 应用行 | {名称}，应用[，正在运行]，快捷键 {按键} | {name}, App[, Running], shortcut {keys} |
| 预设行 | {名称}，预设，{文件夹}，快捷键 {按键} | {name}, Preset, {folder}, shortcut {keys} |
| {按键} 的读法 | Control Option Command Shift 加键名，如「Option Command T」 | e.g. “Option Command T” |

看得见的键帽不配文字，跟 Gocast 组里的那几行一样。

设置 › 快捷键列表里的行（v1.20）：

| 位置 | 中文 | English |
|---|---|---|
| 已设 | {名称}，{类型}，快捷键 {按键} | {name}, {type}, shortcut {keys} |
| 没设 | {名称}，{类型}，未设快捷键 | {name}, {type}, no shortcut |
| 找不到的应用 | {名称}，应用，找不到这个应用，快捷键 {按键} | {name}, App, can’t find this app, shortcut {keys} |
| 录制中 | {名称}，请按下新的快捷键 | {name}, press the new shortcut |
| 撞了 | {名称}，这个快捷键已用于“{另一项}”，改用在这里，按钮 | {name}, this shortcut is already used for “{other}”, Use Here, button |

{类型} 就是行上的小字：应用 / 预设 / Gocast。
