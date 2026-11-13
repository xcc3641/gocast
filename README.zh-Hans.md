# Gocast

**按一下，应用和终端都到。**

[English](README.md) · 简体中文

Gocast 是一个 Mac 上的小启动器。按 ⌘Space，找到应用就打开。按 ⌃\`，终端从屏幕上方落下来，已经在你常用的文件夹里，跑着你常用的命令。

## 能做什么

**搜索面板。** 打几个字，按 ↵。中文名、英文名、拼音都能找到应用（输入 `wx` 就有微信），常用的排在前面。按 ⌘K 有更多操作，比如在访达中显示、拷贝路径。

**悬浮终端。** 浮在所有应用上面，出现在鼠标所在的那块屏幕上，点别处就收起。收起不会结束命令，再叫出来还是原样。⌘T 开新标签。

**预设。** 预设会记住在哪个文件夹运行什么命令，比如在 `~/Projects/my-app` 里运行 `pi`。在设置里添加预设，之后在搜索面板里输入预设名，就会在悬浮终端里打开。

**剪贴板历史。** 按 ⌃⌘V，找回之前拷贝过的文字、链接、图片和文件，按 ↵ 粘贴到刚才的应用。常用的可以置顶。往终端里粘贴图片时，贴进去的是图片在本机的文件路径。

**计算器。** 在搜索框里直接打算式、单位或汇率，比如 `(12+8)*3`、`10 ft in m`、`100 美元换日元`，最上面就是答案，按 ↵ 拷走。汇率每天取一次，断网也能用上次的。

Gocast 待在菜单栏，不占 Dock。深色、浅色跟着系统走，界面有简体中文和英文。会自动更新。

## 安装

**磁盘映像。** 到 [Releases](https://github.com/xcc3641/gocast/releases/latest) 下载最新的 `.dmg`，打开后把 Gocast 拖进“应用程序”文件夹。第一次打开时，去系统设置 › 隐私与安全性，点「仍要打开」。

**终端。** 贴一行就装好。它从本项目的发布页下载最新版，核对是官方发布的（被改过就拒装），放进「应用程序」（没有权限就放进 `~/Applications`），不用管理员密码，然后打开 Gocast。这样装的不用点「仍要打开」。[安装脚本](install.sh) 在仓库里，可以先看内容再跑。

```sh
curl -fsSL https://github.com/xcc3641/gocast/releases/latest/download/install.sh | sh
```

卸载：

```sh
curl -fsSL https://github.com/xcc3641/gocast/releases/latest/download/install.sh | sh -s -- --uninstall
```

预设、设置和剪贴板历史会留着。

**系统要求：** macOS 13 或更高版本，Apple 芯片。之后会支持 Intel。

## 第一次打开

Gocast 想用 ⌘Space，而系统默认把它给了聚焦搜索。如果聚焦搜索还占着，Gocast 会先用 ⌥⇧Space，并打开设置 › 快捷键告诉你。去系统设置 › 键盘 › 键盘快捷键 › 聚焦，关掉“显示聚焦搜索”，Gocast 会自己换回 ⌘Space。也可以就用 ⌥⇧Space，或者换成你喜欢的任何快捷键。

## 快捷键

| 按键 | 在哪里 | 做什么 |
| --- | --- | --- |
| ⌘Space | 任何地方 | 打开搜索面板 |
| ⌃\` | 任何地方 | 显示或收起悬浮终端 |
| ⌃⌘V | 任何地方 | 打开剪贴板历史 |
| ↵ | 搜索面板 | 打开选中的应用或预设 |
| ⌘K | 搜索面板 | 更多操作 |
| ⌘, | 搜索面板 | 设置 |
| Esc | 搜索面板 | 清空输入，再按一次收起 |
| ⌘T | 悬浮终端 | 新标签 |
| ⌘W | 悬浮终端 | 关闭标签 |

这三个全局快捷键都可以在设置 › 快捷键里改。

## 你的数据

Gocast 没有账号，不会把你的任何信息发到别处。它只在检查更新和每天取一次汇率时联网，取汇率时不带你输入的任何内容。两样都能在设置 › 通用里关掉（“自动检查更新”“在搜索中计算”）。预设和设置都是普通文件，放在 `~/Library/Application Support/dev.gocast.app/`。

剪贴板历史也只存在这台 Mac 上（同一个文件夹），不会上传。密码管理器拷贝的密码不会记。可以在设置 › 剪贴板里关掉、清空，或者选哪些应用不记。

## 从源码构建

需要 Apple 芯片的 Mac（macOS 13 或更高版本）、Go 1.27.3 或更高版本，以及和 `go.mod` 里版本一致的 `mygo` 命令行工具（按下面的命令安装，`~/go/bin` 要在 `PATH` 里）。

```sh
git clone https://github.com/xcc3641/gocast.git
cd gocast
go install "github.com/egoist/mygo/cmd/mygo@$(go list -m -f '{{.Version}}' github.com/egoist/mygo)"
go run .            # 运行
go test ./...       # 跑测试
mygo build          # 在 build/darwin-arm64/ 里生成 Gocast.app 和 .dmg
```

提交改动前请先看 [CONTRIBUTING.md](CONTRIBUTING.md)。界面改动以 [docs/design.md](docs/design.md) 为准，文案以 [docs/copy.md](docs/copy.md) 为准。

## 致谢

Gocast 用 [MyGo](https://github.com/egoist/mygo) 写成。终端用的是 [Ghostty](https://ghostty.org) 的 libghostty-vt，拼音搜索用的是 [go-pinyin](https://github.com/mozillazg/go-pinyin)，汇率来自 [Frankfurter](https://frankfurter.dev)（各国央行公布的参考汇率）。

Gocast 受 Raycast 和 Ghostty 快捷终端的启发，与两者都没有关联，也没有得到它们的认可，没有使用它们的代码或图片素材。

## 许可证

[Apache License 2.0](LICENSE)。Gocast 附带的第三方软件见 [NOTICE](NOTICE) 和 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
