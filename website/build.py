#!/usr/bin/env python3
"""Writes the Gocast site's six static pages into web/dist/.

    python3 website/build.py

Hand-edited copy lives in COPY below (English and Simplified Chinese).
Wording follows README.md / README.zh-Hans.md and docs/copy.md.
No build step is needed to deploy: the output is plain HTML committed in dist/.
"""
import os

HERE = os.path.dirname(os.path.abspath(__file__))
DIST = os.path.join(HERE, "dist")
REPO = "https://github.com/xcc3641/gocast"
DMG = REPO + "/releases/latest"
ISSUES = REPO + "/issues"
INSTALL = "curl -fsSL https://github.com/xcc3641/gocast/releases/latest/download/install.sh | sh"
UNINSTALL = "curl -fsSL https://github.com/xcc3641/gocast/releases/latest/download/install.sh | sh -s -- --uninstall"
DATA_DIR = "~/Library/Application Support/dev.gocast.app/"
EFFECTIVE = {"en": "November 13, 2026", "zh": "2026 年 11 月 13 日"}


def k(*keys):
    return "".join(f"<kbd>{x}</kbd>" for x in keys)


COPY = {
    "en": {
        "lang": "en", "prefix": "", "other": "/zh/", "other_label": "简体中文",
        "title": "Gocast — apps, terminal and clipboard on one key",
        "desc": "Gocast is a free, open-source launcher for the Mac. Press ⌘Space to open any app, ⌃` for a drop-down terminal, ⌃⌘V for clipboard history.",
        "headline": "One key. Apps and terminal, right there.",
        "sub": "Gocast is a small launcher for the Mac. Press ⌘Space to find and open an app. Press ⌃` and a terminal drops down from the top of the screen, already in your folder, running your command.",
        "download": "Download for Mac",
        "req": "Free · macOS 13 or later · Apple silicon",
        "cmd_label": "Or install from Terminal:",
        "features": [
            (k("⌘", "Space"), "Open any app", "Type a few letters and press ↵. English names, Chinese names and pinyin all work. The ones you open most come first."),
            (k("⌃", "`"), "A terminal that drops down", "It floats above every app. Hiding it stops nothing: your commands keep running. Presets open a folder and run a command, like <code>pi</code> in your project."),
            (k("⌃", "⌘", "V"), "Clipboard history", "Find the text, links, images and files you copied before, and paste into the app you were in. Pin the ones you use often."),
            (k("⌘", "Space"), "Math and currencies", "Type <code>(12+8)*3</code>, <code>10 ft in m</code> or <code>100 usd in jpy</code> in the search panel. The answer shows up on top."),
        ],
        "open": 'Open source under the Apache 2.0 license. <a href="' + REPO + '">Read the code on GitHub</a>.',
        "support": "Support", "privacy": "Privacy", "home": "Gocast",
        "copyright": "© 2026 The Gocast Authors",
        # support page
        "s_title": "Gocast Support",
        "s_lede": "Gocast is a free launcher for the Mac: apps, a drop-down terminal and clipboard history, each on one key.",
        "s_contact_h": "Get help",
        "s_contact": 'Report a problem or ask a question in <a href="' + ISSUES + '">GitHub Issues</a>. We usually reply within a few days. For a security problem, don’t open a public issue: use “Report a vulnerability” on the repository’s Security tab.',
        "faq": [
            ("macOS says it can’t verify the developer.",
             "Gocast isn’t signed with an Apple Developer ID yet. The first time, open System Settings › Privacy &amp; Security and click Open Anyway. You only do this once; updates install without asking. If you install from Terminal with the line below, macOS won’t ask at all.<pre>" + INSTALL + "</pre>"),
            ("⌘Space opens Spotlight, not Gocast.",
             "Spotlight uses ⌘Space by default, so Gocast starts on ⌥⇧Space. Turn off “Show Spotlight search” in System Settings › Keyboard › Keyboard Shortcuts › Spotlight and Gocast switches to ⌘Space on its own. Or pick any shortcut in Gocast’s Settings › Shortcuts."),
            ("Return copies but doesn’t paste.",
             "To paste into another app, Gocast presses ⌘V for you, and macOS asks you to turn on Gocast in System Settings › Privacy &amp; Security › Accessibility first. Until then, Return copies and you press ⌘V yourself."),
            ("Which Macs does it run on?",
             "macOS 13 or later on Apple silicon. Intel support is planned."),
            ("How do I uninstall it?",
             "Quit Gocast from its menu bar icon and move it to the Trash, or run:<pre>" + UNINSTALL + "</pre>Your presets, settings and clipboard history stay in <code>" + DATA_DIR + "</code>; delete that folder to remove them too."),
        ],
        "faq_h": "Common questions",
        # privacy page
        "p_title": "Privacy Policy",
        "p_lede": "Gocast has no account and collects nothing about you. This page says exactly what it keeps and when it goes online.",
        "p_sections": [
            ("What Gocast collects", "Nothing. Gocast has no account, no analytics, no ads and no crash reporting. We never receive anything you type, copy or run."),
            ("What stays on your Mac",
             "Your presets, settings and clipboard history are files in <code>" + DATA_DIR + "</code>, readable only by your Mac user. They are never uploaded. Clipboard history is kept as plain files, not encrypted. Passwords copied from password managers are never kept, and you can choose apps whose copies are never kept."),
            ("When Gocast goes online",
             "Only twice, and neither request carries anything you typed or copied:"
             "<ul><li><strong>Checking for updates</strong>, from the Gocast project on GitHub, about once a day. Turn it off with Settings › General › Check for updates automatically.</li>"
             "<li><strong>Exchange rates</strong> for the calculator, from Frankfurter (frankfurter.dev), once a day. Turn it off with Settings › General › Calculate in search.</li></ul>"
             "GitHub and Frankfurter see the request like any website sees a visit, such as your IP address, under their own privacy policies. Gocast sends them nothing else."),
            ("Third parties", "Gocast shares no data with anyone. The only outside services it contacts are the two above."),
            ("Keeping and deleting your data",
             "Clipboard history is kept for one month by default; change it in Settings › Clipboard (1 day to forever). Pinned items don’t expire. Clear History removes it at once. To delete everything, quit Gocast and delete <code>" + DATA_DIR + "</code>."),
            ("Changing your mind", "Turn off clipboard history, update checks or exchange rates in Settings at any time. Nothing has to be sent to us, because nothing ever was."),
            ("Questions", 'Ask in <a href="' + ISSUES + '">GitHub Issues</a>.'),
        ],
        "p_date": "Effective " + EFFECTIVE["en"] + ".",
        "back": "← Gocast",
    },
    "zh": {
        "lang": "zh-Hans", "prefix": "/zh", "other": "/", "other_label": "English",
        "title": "Gocast — 按一下，应用、终端和剪贴板都到",
        "desc": "Gocast 是一个免费、开源的 Mac 启动器。按 ⌘Space 打开应用，按 ⌃` 落下终端，按 ⌃⌘V 找回剪贴板历史。",
        "headline": "按一下，应用和终端都到。",
        "sub": "Gocast 是一个 Mac 上的小启动器。按 ⌘Space，找到应用就打开。按 ⌃`，终端从屏幕上方落下来，已经在你常用的文件夹里，跑着你常用的命令。",
        "download": "下载 Mac 版",
        "req": "免费 · macOS 13 或更高版本 · Apple 芯片",
        "cmd_label": "也可以在终端里一行装好：",
        "features": [
            (k("⌘", "Space"), "打开任何应用", "打几个字，按 ↵。中文名、英文名、拼音都能找到（输入 <code>wx</code> 就有微信），常用的排在前面。"),
            (k("⌃", "`"), "从上方落下的终端", "浮在所有应用上面，收起不会结束命令。预设会在指定文件夹里运行指定命令，比如在你的项目里运行 <code>pi</code>。"),
            (k("⌃", "⌘", "V"), "剪贴板历史", "找回之前拷贝过的文字、链接、图片和文件，粘贴到刚才的应用。常用的可以置顶。"),
            (k("⌘", "Space"), "算数和汇率", "在搜索框里直接打 <code>(12+8)*3</code>、<code>10 ft in m</code> 或 <code>100 美元换日元</code>，最上面就是答案。"),
        ],
        "open": '开源，使用 Apache 2.0 许可证。<a href="' + REPO + '">在 GitHub 上看代码</a>。',
        "support": "支持", "privacy": "隐私", "home": "Gocast",
        "copyright": "© 2026 The Gocast Authors",
        "s_title": "Gocast 支持",
        "s_lede": "Gocast 是一个免费的 Mac 启动器：应用、悬浮终端、剪贴板历史，各自按一下就到。",
        "s_contact_h": "获取帮助",
        "s_contact": '遇到问题或有疑问，请到 <a href="' + ISSUES + '">GitHub Issues</a> 提出，我们一般几天内回复。安全问题请不要公开提 issue，在仓库的 Security 页里点「Report a vulnerability」私下报告。',
        "faq_h": "常见问题",
        "faq": [
            ("系统说无法验证开发者。",
             "Gocast 暂时还没有 Apple 的 Developer ID 签名。第一次打开时，去系统设置 › 隐私与安全性，点「仍要打开」。只需要这一次，之后的更新会自动装。用下面这行命令在终端里安装的话，系统不会拦。<pre>" + INSTALL + "</pre>"),
            ("按 ⌘Space 出来的是聚焦搜索。",
             "系统默认把 ⌘Space 给了聚焦搜索，所以 Gocast 会先用 ⌥⇧Space。去系统设置 › 键盘 › 键盘快捷键 › 聚焦，关掉「显示聚焦搜索」，Gocast 会自己换回 ⌘Space。也可以在 Gocast 的设置 › 快捷键里换成任何快捷键。"),
            ("按回车只拷贝，没有粘贴。",
             "要粘贴到别的应用，Gocast 会替你按一下 ⌘V，系统要求先在系统设置 › 隐私与安全性 › 辅助功能里打开 Gocast。打开之前，回车只拷贝，再自己按 ⌘V。"),
            ("哪些 Mac 能用？",
             "macOS 13 或更高版本，Apple 芯片。之后会支持 Intel。"),
            ("怎么卸载？",
             "在菜单栏图标里退出 Gocast，把它拖进废纸篓；或者在终端运行：<pre>" + UNINSTALL + "</pre>预设、设置和剪贴板历史会留在 <code>" + DATA_DIR + "</code>，想一起删掉就删这个文件夹。"),
        ],
        "p_title": "隐私政策",
        "p_lede": "Gocast 没有账号，不收集你的任何信息。这一页如实写明它存了什么、什么时候联网。",
        "p_sections": [
            ("收集哪些数据", "不收集。Gocast 没有账号，没有统计、广告或崩溃上报。你输入、拷贝、运行的任何内容，我们都收不到。"),
            ("存在你 Mac 上的东西",
             "预设、设置和剪贴板历史都是文件，放在 <code>" + DATA_DIR + "</code>，只有你这个 Mac 用户能读，不会上传。剪贴板历史以普通文件保存，没有加密。密码管理器拷贝的密码不会记，你也可以指定哪些应用拷贝的内容不记。"),
            ("什么时候联网",
             "只有两种情况，都不带你输入或拷贝的任何内容："
             "<ul><li><strong>检查更新</strong>：大约每天一次，从 GitHub 上的 Gocast 项目获取。在设置 › 通用里关掉「自动检查更新」即可停止。</li>"
             "<li><strong>取汇率</strong>：计算器用，每天一次，来自 Frankfurter（frankfurter.dev）。在设置 › 通用里关掉「在搜索中计算」即可停止。</li></ul>"
             "GitHub 和 Frankfurter 会像任何网站一样看到这次访问（比如你的 IP 地址），按它们各自的隐私政策处理。除此之外 Gocast 不发送任何东西。"),
            ("第三方", "Gocast 不和任何人分享数据。它只会连上面这两个外部服务。"),
            ("保存多久、怎么删除",
             "剪贴板历史默认保留一个月，可以在设置 › 剪贴板里改（1 天到一直保留），置顶的不会过期；「清空历史…」会立刻删掉。想删掉全部数据，退出 Gocast 后删除 <code>" + DATA_DIR + "</code>。"),
            ("撤回同意", "随时可以在设置里关掉剪贴板历史、自动检查更新或取汇率。不需要联系我们，因为我们本来就没有你的数据。"),
            ("有疑问", '请到 <a href="' + ISSUES + '">GitHub Issues</a> 提出。'),
        ],
        "p_date": "生效日期：" + EFFECTIVE["zh"] + "。",
        "back": "← Gocast",
    },
}

ICON_SVG = ("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 64 64'%3E"
            "%3Crect width='64' height='64' rx='14' fill='%231C1C1E'/%3E"
            "%3Crect x='25' y='17' width='14' height='30' rx='3' fill='white'/%3E%3C/svg%3E")


def head(c, title, desc, path):
    other = c["other"] + path.lstrip("/") if path != "/" else c["other"]
    return f"""<!doctype html>
<html lang="{c['lang']}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{title}</title>
<meta name="description" content="{desc}">
<meta name="theme-color" content="#FFFFFF" media="(prefers-color-scheme: light)">
<meta name="theme-color" content="#111113" media="(prefers-color-scheme: dark)">
<meta property="og:title" content="{title}">
<meta property="og:description" content="{desc}">
<meta property="og:image" content="/assets/icon-256.png">
<link rel="icon" href="{ICON_SVG}" type="image/svg+xml">
<link rel="apple-touch-icon" href="/assets/apple-touch-icon.png">
<link rel="alternate" hreflang="en" href="{path}">
<link rel="alternate" hreflang="zh-Hans" href="/zh/{path.lstrip('/')}">
<link rel="stylesheet" href="/assets/site.css">
</head>
<body>
<nav class="lang" aria-label="Language"><a href="{other}" hreflang="{'zh-Hans' if c['lang']=='en' else 'en'}">{c['other_label']}</a></nav>
"""


def foot(c):
    p = c["prefix"]
    return f"""<footer>
<span class="copy">{c['copyright']}</span>
<a href="{p}/support/">{c['support']}</a>
<a href="{p}/privacy/">{c['privacy']}</a>
<a href="{REPO}">GitHub</a>
</footer>
</body>
</html>
"""


def landing(c):
    feats = "\n".join(
        f'<section><div class="keys">{keys}</div><h2>{h}</h2><p>{t}</p></section>'
        for keys, h, t in c["features"])
    return head(c, c["title"], c["desc"], "/") + f"""<main>
<header class="hero">
<img class="icon" src="/assets/icon-256.png" width="96" height="96" alt="">
<h1 class="name">Gocast</h1>
<p class="headline">{c['headline']}</p>
<p class="sub">{c['sub']}</p>
<div class="get">
<a class="button" href="{DMG}">{c['download']}</a>
<span class="req">{c['req']}</span>
</div>
<div class="cmd">
<p>{c['cmd_label']}</p>
<pre><code>{INSTALL}</code></pre>
</div>
</header>
<div class="features">
{feats}
</div>
<div class="open"><p>{c['open']}</p></div>
</main>
""" + foot(c)


def support(c):
    faq = "\n".join(f"<h3>{q}</h3><div>{a}</div>" for q, a in c["faq"])
    return head(c, c["s_title"], c["s_lede"], "/support/") + f"""<main class="page">
<a class="back" href="{c['prefix']}/">{c['back']}</a>
<h1>{c['s_title']}</h1>
<p class="lede">{c['s_lede']}</p>
<h2>{c['s_contact_h']}</h2>
<p>{c['s_contact']}</p>
<h2 class="faq-h">{c['faq_h']}</h2>
{faq}
</main>
""" + foot(c)


def privacy(c):
    secs = "\n".join(f"<h2>{h}</h2><p>{t}</p>" for h, t in c["p_sections"])
    return head(c, "Gocast — " + c["p_title"], c["p_lede"], "/privacy/") + f"""<main class="page">
<a class="back" href="{c['prefix']}/">{c['back']}</a>
<h1>{c['p_title']}</h1>
<p class="lede">{c['p_lede']}</p>
{secs}
<p class="date">{c['p_date']}</p>
</main>
""" + foot(c)


def write(rel, html):
    path = os.path.join(DIST, rel)
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(html)


NOT_FOUND = """<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Not found · Gocast</title>
<link rel="icon" href="{ICON}" type="image/svg+xml">
<link rel="stylesheet" href="/assets/site.css">
</head>
<body>
<main class="page">
<h1>Page not found</h1>
<p class="lede">There’s nothing here. <a href="/">Go to Gocast</a> · <a href="/zh/">前往 Gocast 中文页</a></p>
</main>
</body>
</html>
"""

for key, c in COPY.items():
    base = "" if key == "en" else "zh/"
    write(base + "index.html", landing(c))
    write(base + "support/index.html", support(c))
    write(base + "privacy/index.html", privacy(c))
write("404.html", NOT_FOUND.replace("{ICON}", ICON_SVG))
print("wrote", DIST)
